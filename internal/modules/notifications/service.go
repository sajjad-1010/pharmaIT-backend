package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/http/pagination"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type Service struct {
	db       *gorm.DB
	provider PushProvider
	log      zerolog.Logger
}

func NewService(db *gorm.DB, provider PushProvider, log zerolog.Logger) *Service {
	return &Service{
		db:       db,
		provider: provider,
		log:      log,
	}
}

type DeviceUpsertInput struct {
	Platform    model.NotificationDevicePlatform `json:"platform"`
	Token       string                           `json:"token"`
	DeviceLabel *string                          `json:"device_label"`
}

type PreferencePatch struct {
	InAppEnabled               *bool `json:"in_app_enabled"`
	PushEnabled                *bool `json:"push_enabled"`
	OrderCreated               *bool `json:"order_created"`
	OrderStatusChanged         *bool `json:"order_status_changed"`
	PaymentUpdated             *bool `json:"payment_updated"`
	AccessUpdated              *bool `json:"access_updated"`
	RareBidReceived            *bool `json:"rare_bid_received"`
	RareBidSelected            *bool `json:"rare_bid_selected"`
	ManufacturerRequestCreated *bool `json:"manufacturer_request_created"`
	ManufacturerQuoteCreated   *bool `json:"manufacturer_quote_created"`
}

type NotificationPayload map[string]interface{}

type NotificationItem struct {
	ID        uuid.UUID              `json:"id"`
	Kind      model.NotificationKind `json:"kind"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Payload   NotificationPayload    `json:"payload"`
	IsRead    bool                   `json:"is_read"`
	ReadAt    *time.Time             `json:"read_at,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

func (s *Service) UpsertDevice(ctx context.Context, userID uuid.UUID, input DeviceUpsertInput) (*model.NotificationDevice, error) {
	token := strings.TrimSpace(input.Token)
	if token == "" {
		return nil, appErr.BadRequest("INVALID_TOKEN", "token is required", nil)
	}
	switch input.Platform {
	case model.NotificationDevicePlatformAndroid, model.NotificationDevicePlatformWeb:
	default:
		return nil, appErr.BadRequest("INVALID_PLATFORM", "platform must be ANDROID or WEB", nil)
	}

	deviceLabel := trimOptional(input.DeviceLabel)
	row := &model.NotificationDevice{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("token = ?", token).First(row).Error
		switch err {
		case nil:
			updates := map[string]interface{}{
				"user_id":      userID,
				"platform":     input.Platform,
				"device_label": deviceLabel,
				"is_active":    true,
				"last_seen_at": time.Now().UTC(),
			}
			return tx.Model(&model.NotificationDevice{}).Where("id = ?", row.ID).Updates(updates).Error
		case gorm.ErrRecordNotFound:
			*row = model.NotificationDevice{
				ID:          uuid.New(),
				UserID:      userID,
				Platform:    input.Platform,
				Token:       token,
				DeviceLabel: deviceLabel,
				IsActive:    true,
				LastSeenAt:  time.Now().UTC(),
			}
			return tx.Create(row).Error
		default:
			return err
		}
	})
	if err != nil {
		return nil, appErr.Internal("failed to upsert notification device")
	}

	if err := s.ensurePreferences(ctx, userID); err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).First(row, "token = ?", token).Error; err != nil {
		return nil, appErr.Internal("failed to reload notification device")
	}
	return row, nil
}

func (s *Service) DeactivateDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	result := s.db.WithContext(ctx).
		Model(&model.NotificationDevice{}).
		Where("id = ? AND user_id = ?", deviceID, userID).
		Updates(map[string]interface{}{
			"is_active":    false,
			"last_seen_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return appErr.Internal("failed to deactivate notification device")
	}
	if result.RowsAffected == 0 {
		return appErr.NotFound("DEVICE_NOT_FOUND", "notification device not found")
	}
	return nil
}

func (s *Service) GetPreferences(ctx context.Context, userID uuid.UUID) (*model.NotificationPreference, error) {
	if err := s.ensurePreferences(ctx, userID); err != nil {
		return nil, err
	}
	var pref model.NotificationPreference
	if err := s.db.WithContext(ctx).First(&pref, "user_id = ?", userID).Error; err != nil {
		return nil, appErr.Internal("failed to load notification preferences")
	}
	return &pref, nil
}

func (s *Service) UpdatePreferences(ctx context.Context, userID uuid.UUID, patch PreferencePatch) (*model.NotificationPreference, error) {
	if err := s.ensurePreferences(ctx, userID); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	applyBoolPatch(updates, "in_app_enabled", patch.InAppEnabled)
	applyBoolPatch(updates, "push_enabled", patch.PushEnabled)
	applyBoolPatch(updates, "order_created", patch.OrderCreated)
	applyBoolPatch(updates, "order_status_changed", patch.OrderStatusChanged)
	applyBoolPatch(updates, "payment_updated", patch.PaymentUpdated)
	applyBoolPatch(updates, "access_updated", patch.AccessUpdated)
	applyBoolPatch(updates, "rare_bid_received", patch.RareBidReceived)
	applyBoolPatch(updates, "rare_bid_selected", patch.RareBidSelected)
	applyBoolPatch(updates, "manufacturer_request_created", patch.ManufacturerRequestCreated)
	applyBoolPatch(updates, "manufacturer_quote_created", patch.ManufacturerQuoteCreated)

	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).
			Model(&model.NotificationPreference{}).
			Where("user_id = ?", userID).
			Updates(updates).Error; err != nil {
			return nil, appErr.Internal("failed to update notification preferences")
		}
	}

	return s.GetPreferences(ctx, userID)
}

type ListInput struct {
	UserID     uuid.UUID
	Limit      int
	Cursor     *pagination.Cursor
	UnreadOnly bool
}

func (s *Service) ListDevices(ctx context.Context, userID uuid.UUID) ([]model.NotificationDevice, error) {
	var rows []model.NotificationDevice
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, appErr.Internal("failed to list notification devices")
	}
	return rows, nil
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[NotificationItem], error) {
	q := s.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ?", input.UserID)
	if input.UnreadOnly {
		q = q.Where("is_read = FALSE")
	}
	if input.Cursor != nil {
		q = q.Where("(created_at, id) < (?, ?)", input.Cursor.Timestamp, input.Cursor.ID)
	}
	q = q.Order("created_at DESC").Order("id DESC").Limit(input.Limit)

	var rows []model.Notification
	if err := q.Find(&rows).Error; err != nil {
		return pagination.Result[NotificationItem]{}, appErr.Internal("failed to list notifications")
	}

	items, err := decodeNotificationItems(rows)
	if err != nil {
		return pagination.Result[NotificationItem]{}, err
	}

	return pagination.BuildResult(items, input.Limit, func(item NotificationItem) (time.Time, uuid.UUID) {
		return item.CreatedAt, item.ID
	}), nil
}

func (s *Service) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": &now,
		})
	if result.Error != nil {
		return appErr.Internal("failed to mark notification as read")
	}
	if result.RowsAffected == 0 {
		return appErr.NotFound("NOTIFICATION_NOT_FOUND", "notification not found")
	}
	return nil
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ? AND is_read = FALSE", userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": &now,
		}).Error; err != nil {
		return appErr.Internal("failed to mark notifications as read")
	}
	return nil
}

func (s *Service) HandleEvent(ctx context.Context, eventType string, payload map[string]interface{}) error {
	switch eventType {
	case "order.status_changed":
		return s.handleOrderEvent(ctx, payload)
	case "payment.verified":
		return s.handleSingleUserEvent(ctx, payload, extractUUID(payload, "user_id"), model.NotificationKindPaymentUpdated, "Payment verified", "Your payment was verified.", buildDedupeKey("payment.verified", extractUUID(payload, "user_id"), firstString(payload["payment_id"]), firstString(payload["invoice_id"])))
	case "access.updated":
		return s.handleSingleUserEvent(ctx, payload, extractUUID(payload, "user_id"), model.NotificationKindAccessUpdated, "Access updated", "Your access window has been updated.", buildDedupeKey("access.updated", extractUUID(payload, "user_id"), firstString(payload["invoice_id"]), firstString(payload["access_until"])))
	case "rare.bid_created":
		return s.handleRareBidCreated(ctx, payload)
	case "rare.bid_selected":
		return s.handleSingleUserEvent(ctx, payload, extractUUID(payload, "wholesaler_id"), model.NotificationKindRareBidSelected, "Rare bid selected", "Your rare bid was selected by a pharmacy.", buildDedupeKey("rare.bid_selected", extractUUID(payload, "wholesaler_id"), firstString(payload["rare_bid_id"]), firstString(payload["rare_request_id"])))
	case "manufacturer.request_created":
		return s.handleSingleUserEvent(ctx, payload, extractUUID(payload, "manufacturer_id"), model.NotificationKindManufacturerRequestCreated, "New manufacturer request", "A wholesaler sent a new manufacturer request.", buildDedupeKey("manufacturer.request_created", extractUUID(payload, "manufacturer_id"), firstString(payload["request_id"])))
	case "manufacturer.quote_created":
		return s.handleManufacturerQuoteCreated(ctx, payload)
	default:
		return nil
	}
}

func (s *Service) handleOrderEvent(ctx context.Context, payload map[string]interface{}) error {
	orderID := firstString(payload["order_id"])
	newStatus := strings.ToUpper(firstString(payload["new_status"]))
	oldStatus := strings.ToUpper(firstString(payload["old_status"]))
	pharmacyID := extractUUID(payload, "pharmacy_id")
	wholesalerID := extractUUID(payload, "wholesaler_id")

	if oldStatus == "" && newStatus == string(model.OrderStatusCreated) {
		return s.createNotificationForUser(ctx, wholesalerID, model.NotificationKindOrderCreated, "New order received", fmt.Sprintf("A pharmacy placed order %s.", orderID), payload, fmt.Sprintf("order.created:%s:%s", wholesalerID, orderID))
	}

	if err := s.createNotificationForUser(ctx, pharmacyID, model.NotificationKindOrderStatusChanged, "Order status changed", fmt.Sprintf("Order %s is now %s.", orderID, newStatus), payload, fmt.Sprintf("order.status_changed:%s:%s:%s", pharmacyID, orderID, newStatus)); err != nil {
		return err
	}
	return s.createNotificationForUser(ctx, wholesalerID, model.NotificationKindOrderStatusChanged, "Order status changed", fmt.Sprintf("Order %s is now %s.", orderID, newStatus), payload, fmt.Sprintf("order.status_changed:%s:%s:%s", wholesalerID, orderID, newStatus))
}

func (s *Service) handleRareBidCreated(ctx context.Context, payload map[string]interface{}) error {
	requestID := extractUUID(payload, "rare_request_id")
	if requestID == uuid.Nil {
		return appErr.Internal("rare bid notification is missing rare_request_id")
	}
	var req model.RareRequest
	if err := s.db.WithContext(ctx).First(&req, "id = ?", requestID).Error; err != nil {
		return appErr.Internal("failed to resolve rare request recipient")
	}
	return s.createNotificationForUser(ctx, req.PharmacyID, model.NotificationKindRareBidReceived, "New rare bid received", "A wholesaler submitted a bid for your rare request.", payload, buildDedupeKey("rare.bid_received", req.PharmacyID, firstString(payload["rare_bid_id"]), requestID.String()))
}

func (s *Service) handleManufacturerQuoteCreated(ctx context.Context, payload map[string]interface{}) error {
	requestID := extractUUID(payload, "request_id")
	if requestID == uuid.Nil {
		return appErr.Internal("manufacturer quote notification is missing request_id")
	}
	var req model.ManufacturerRequest
	if err := s.db.WithContext(ctx).First(&req, "id = ?", requestID).Error; err != nil {
		return appErr.Internal("failed to resolve manufacturer request recipient")
	}
	return s.createNotificationForUser(ctx, req.WholesalerID, model.NotificationKindManufacturerQuoteCreated, "New manufacturer quote", "A manufacturer submitted a quote for your request.", payload, buildDedupeKey("manufacturer.quote_created", req.WholesalerID, firstString(payload["quote_id"]), requestID.String()))
}

func (s *Service) handleSingleUserEvent(ctx context.Context, payload map[string]interface{}, userID uuid.UUID, kind model.NotificationKind, title, body, dedupeKey string) error {
	if userID == uuid.Nil {
		return appErr.Internal("notification payload missing target user")
	}
	return s.createNotificationForUser(ctx, userID, kind, title, body, payload, dedupeKey)
}

func (s *Service) createNotificationForUser(ctx context.Context, userID uuid.UUID, kind model.NotificationKind, title, body string, payload map[string]interface{}, dedupeKey string) error {
	if userID == uuid.Nil {
		return nil
	}

	pref, err := s.GetPreferences(ctx, userID)
	if err != nil {
		return err
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return appErr.Internal("failed to encode notification payload")
	}

	row := &model.Notification{
		ID:          uuid.New(),
		UserID:      userID,
		Kind:        kind,
		Title:       title,
		Body:        body,
		PayloadJSON: rawPayload,
		IsRead:      false,
	}
	if strings.TrimSpace(dedupeKey) != "" {
		key := dedupeKey
		row.DedupeKey = &key
	}

	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		if isDuplicateKey(err) {
			return nil
		}
		return appErr.Internal("failed to create notification row")
	}

	if pref.PushEnabled {
		if err := s.dispatchPush(ctx, row, pref); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) dispatchPush(ctx context.Context, notification *model.Notification, pref *model.NotificationPreference) error {
	if !kindPushEnabled(pref, notification.Kind) {
		return nil
	}

	var devices []model.NotificationDevice
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND is_active = TRUE", notification.UserID).
		Find(&devices).Error; err != nil {
		return appErr.Internal("failed to load notification devices")
	}

	for _, device := range devices {
		delivery := model.NotificationDelivery{
			ID:             uuid.New(),
			NotificationID: notification.ID,
			DeviceID:       device.ID,
			Platform:       device.Platform,
			Status:         model.NotificationDeliveryStatusPending,
		}

		if err := s.db.WithContext(ctx).Create(&delivery).Error; err != nil {
			return appErr.Internal("failed to create notification delivery log")
		}

		status := model.NotificationDeliveryStatusSent
		var deliveredAt *time.Time
		var errorText *string
		if s.provider == nil {
			status = model.NotificationDeliveryStatusSkipped
			msg := ErrPushNotConfigured.Error()
			errorText = &msg
		} else if err := s.provider.Send(ctx, PushMessage{Notification: *notification, Device: device}); err != nil {
			if errors.Is(err, ErrPushNotConfigured) {
				status = model.NotificationDeliveryStatusSkipped
			} else if errors.Is(err, ErrPushTokenInvalid) {
				status = model.NotificationDeliveryStatusFailed
				if deactivateErr := s.db.WithContext(ctx).
					Model(&model.NotificationDevice{}).
					Where("id = ?", device.ID).
					Updates(map[string]interface{}{
						"is_active":    false,
						"last_seen_at": time.Now().UTC(),
					}).Error; deactivateErr != nil {
					return appErr.Internal("failed to deactivate invalid notification device")
				}
			} else {
				status = model.NotificationDeliveryStatusFailed
			}
			msg := err.Error()
			errorText = &msg
		} else {
			now := time.Now().UTC()
			deliveredAt = &now
		}

		if err := s.db.WithContext(ctx).
			Model(&model.NotificationDelivery{}).
			Where("id = ?", delivery.ID).
			Updates(map[string]interface{}{
				"status":       status,
				"error_text":   errorText,
				"delivered_at": deliveredAt,
			}).Error; err != nil {
			return appErr.Internal("failed to update notification delivery log")
		}
	}
	return nil
}

func decodeNotificationItems(rows []model.Notification) ([]NotificationItem, error) {
	items := make([]NotificationItem, 0, len(rows))
	for _, row := range rows {
		payload := NotificationPayload{}
		if err := json.Unmarshal(row.PayloadJSON, &payload); err != nil {
			return nil, appErr.Internal("failed to decode notification payload")
		}
		items = append(items, NotificationItem{
			ID:        row.ID,
			Kind:      row.Kind,
			Title:     row.Title,
			Body:      row.Body,
			Payload:   payload,
			IsRead:    row.IsRead,
			ReadAt:    row.ReadAt,
			CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}

func (s *Service) ensurePreferences(ctx context.Context, userID uuid.UUID) error {
	row := model.NotificationPreference{UserID: userID}
	if err := s.db.WithContext(ctx).FirstOrCreate(&row, model.NotificationPreference{UserID: userID}).Error; err != nil {
		return appErr.Internal("failed to ensure notification preferences")
	}
	return nil
}

func applyBoolPatch(target map[string]interface{}, key string, value *bool) {
	if value != nil {
		target[key] = *value
	}
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func firstString(v interface{}) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func buildDedupeKey(prefix string, userID uuid.UUID, parts ...string) string {
	segments := []string{prefix, userID.String()}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			segments = append(segments, trimmed)
		}
	}
	return strings.Join(segments, ":")
}

func extractUUID(payload map[string]interface{}, key string) uuid.UUID {
	value := firstString(payload[key])
	id, _ := uuid.Parse(value)
	return id
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}

func kindPushEnabled(pref *model.NotificationPreference, kind model.NotificationKind) bool {
	switch kind {
	case model.NotificationKindOrderCreated:
		return pref.OrderCreated
	case model.NotificationKindOrderStatusChanged:
		return pref.OrderStatusChanged
	case model.NotificationKindPaymentUpdated:
		return pref.PaymentUpdated
	case model.NotificationKindAccessUpdated:
		return pref.AccessUpdated
	case model.NotificationKindRareBidReceived:
		return pref.RareBidReceived
	case model.NotificationKindRareBidSelected:
		return pref.RareBidSelected
	case model.NotificationKindManufacturerRequestCreated:
		return pref.ManufacturerRequestCreated
	case model.NotificationKindManufacturerQuoteCreated:
		return pref.ManufacturerQuoteCreated
	default:
		return true
	}
}
