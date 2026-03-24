package offers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/http/pagination"
	"pharmalink/server/internal/modules/outbox"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	redis  *redis.Client
	outbox *outbox.Service
}

func NewService(db *gorm.DB, redis *redis.Client, outboxSvc *outbox.Service) *Service {
	return &Service{
		db:     db,
		redis:  redis,
		outbox: outboxSvc,
	}
}

type ListInput struct {
	Query  string
	Limit  int
	Cursor *pagination.Cursor
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[model.WholesalerOffer], error) {
	query := strings.TrimSpace(input.Query)
	cacheKey := fmt.Sprintf("offers:query=%s:limit=%d:cursor=%v", strings.ToLower(query), input.Limit, input.Cursor)
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
			var out pagination.Result[model.WholesalerOffer]
			if jsonErr := json.Unmarshal([]byte(cached), &out); jsonErr == nil {
				return out, nil
			}
		}
	}

	q := s.db.WithContext(ctx).Model(&model.WholesalerOffer{}).Where("is_active = TRUE")
	if query != "" {
		like := "%" + query + "%"
		q = q.Where("name ILIKE ? OR similarity(name, ?) > 0.15", like, query)
	}
	if input.Cursor != nil {
		q = q.Where("(updated_at, id) < (?, ?)", input.Cursor.Timestamp, input.Cursor.ID)
	}
	q = q.Order("updated_at DESC").Order("id DESC").Limit(input.Limit)

	var rows []model.WholesalerOffer
	if err := q.Find(&rows).Error; err != nil {
		return pagination.Result[model.WholesalerOffer]{}, appErr.Internal("failed to list offers")
	}

	out := pagination.BuildResult(rows, input.Limit, func(item model.WholesalerOffer) (time.Time, uuid.UUID) {
		return item.UpdatedAt, item.ID
	})

	if s.redis != nil {
		if payload, err := json.Marshal(out); err == nil {
			_ = s.redis.Set(ctx, cacheKey, payload, 90*time.Second).Err()
		}
	}

	return out, nil
}

type UpsertOfferInput struct {
	Name         string  `json:"name"`
	DisplayPrice string  `json:"display_price"`
	ExpiryDate   *string `json:"expiry_date"`
	Producer     *string `json:"producer"`
	IsActive     *bool   `json:"is_active"`
}

type BatchCreateInput struct {
	Items []UpsertOfferInput `json:"items"`
}

type BatchError struct {
	Index   int         `json:"index"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type BatchCreateResult struct {
	CreatedCount int                     `json:"created_count"`
	FailedCount  int                     `json:"failed_count"`
	Items        []model.WholesalerOffer `json:"items"`
	Errors       []BatchError            `json:"errors"`
}

func (s *Service) buildOffer(wholesalerID uuid.UUID, input UpsertOfferInput) (*model.WholesalerOffer, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, appErr.BadRequest("INVALID_NAME", "name is required", nil)
	}

	price, err := decimal.NewFromString(strings.TrimSpace(input.DisplayPrice))
	if err != nil || price.IsNegative() {
		return nil, appErr.BadRequest("INVALID_DISPLAY_PRICE", "display_price must be non-negative decimal", nil)
	}

	offer := &model.WholesalerOffer{
		ID:           uuid.New(),
		WholesalerID: wholesalerID,
		Name:         name,
		Producer:     trimOptional(input.Producer),
		DisplayPrice: price,
		AvailableQty: 0,
		IsActive:     true,
	}
	if input.IsActive != nil {
		offer.IsActive = *input.IsActive
	}
	if input.ExpiryDate != nil && strings.TrimSpace(*input.ExpiryDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*input.ExpiryDate))
		if err != nil {
			return nil, appErr.BadRequest("INVALID_EXPIRY_DATE", "expiry_date must be YYYY-MM-DD", nil)
		}
		offer.ExpiryDate = &parsed
	}
	return offer, nil
}

func (s *Service) Create(ctx context.Context, wholesalerID uuid.UUID, input UpsertOfferInput) (*model.WholesalerOffer, error) {
	offer, err := s.buildOffer(wholesalerID, input)
	if err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(offer).Error; err != nil {
			return appErr.Internal("failed to create offer")
		}

		outboxRow, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "offer.updated",
			Payload: map[string]interface{}{
				"offer_id":      offer.ID,
				"wholesaler_id": offer.WholesalerID,
				"name":          offer.Name,
				"producer":      offer.Producer,
				"display_price": offer.DisplayPrice.StringFixed(4),
				"available_qty": offer.AvailableQty,
				"is_active":     offer.IsActive,
			},
		})
		if err != nil {
			return err
		}
		return s.outbox.Notify(ctx, tx, outboxRow.ID)
	}); err != nil {
		return nil, err
	}

	return offer, nil
}

func (s *Service) CreateBatch(ctx context.Context, wholesalerID uuid.UUID, input BatchCreateInput) (*BatchCreateResult, error) {
	if len(input.Items) == 0 {
		return nil, appErr.BadRequest("EMPTY_BATCH", "items are required", nil)
	}
	if len(input.Items) > 10000 {
		return nil, appErr.BadRequest("BATCH_TOO_LARGE", "items limit is 10000", nil)
	}

	result := &BatchCreateResult{
		Items:  make([]model.WholesalerOffer, 0, len(input.Items)),
		Errors: make([]BatchError, 0),
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for idx, item := range input.Items {
			offer, err := s.buildOffer(wholesalerID, item)
			if err != nil {
				var appE appErr.AppError
				if ok := errorAs(err, &appE); ok {
					result.Errors = append(result.Errors, BatchError{
						Index:   idx,
						Code:    appE.Code,
						Message: appE.Message,
						Details: appE.Details,
					})
					continue
				}
				return err
			}

			if err := tx.Create(offer).Error; err != nil {
				return appErr.Internal("failed to create offer batch")
			}

			outboxRow, err := s.outbox.Write(ctx, tx, outbox.Event{
				Type: "offer.updated",
				Payload: map[string]interface{}{
					"offer_id":      offer.ID,
					"wholesaler_id": offer.WholesalerID,
					"name":          offer.Name,
					"producer":      offer.Producer,
					"display_price": offer.DisplayPrice.StringFixed(4),
					"available_qty": offer.AvailableQty,
					"is_active":     offer.IsActive,
				},
			})
			if err != nil {
				return err
			}
			if err := s.outbox.Notify(ctx, tx, outboxRow.ID); err != nil {
				return err
			}

			result.Items = append(result.Items, *offer)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	result.CreatedCount = len(result.Items)
	result.FailedCount = len(result.Errors)
	return result, nil
}

func (s *Service) Update(ctx context.Context, wholesalerID, offerID uuid.UUID, input UpsertOfferInput) (*model.WholesalerOffer, error) {
	var offer model.WholesalerOffer
	if err := s.db.WithContext(ctx).
		First(&offer, "id = ? AND wholesaler_id = ?", offerID, wholesalerID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, appErr.NotFound("OFFER_NOT_FOUND", "offer not found")
		}
		return nil, appErr.Internal("failed to query offer")
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(input.Name) != "" {
		updates["name"] = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.DisplayPrice) != "" {
		price, err := decimal.NewFromString(strings.TrimSpace(input.DisplayPrice))
		if err != nil || price.IsNegative() {
			return nil, appErr.BadRequest("INVALID_DISPLAY_PRICE", "display_price must be non-negative decimal", nil)
		}
		updates["display_price"] = price
	}
	if input.Producer != nil {
		updates["producer"] = trimOptional(input.Producer)
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}
	if input.ExpiryDate != nil {
		if strings.TrimSpace(*input.ExpiryDate) == "" {
			updates["expiry_date"] = nil
		} else {
			parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*input.ExpiryDate))
			if err != nil {
				return nil, appErr.BadRequest("INVALID_EXPIRY_DATE", "expiry_date must be YYYY-MM-DD", nil)
			}
			updates["expiry_date"] = parsed
		}
	}

	if len(updates) == 0 {
		return &offer, nil
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.WholesalerOffer{}).
			Where("id = ? AND wholesaler_id = ?", offerID, wholesalerID).
			Updates(updates).Error; err != nil {
			return appErr.Internal("failed to update offer")
		}

		if err := tx.First(&offer, "id = ?", offerID).Error; err != nil {
			return appErr.Internal("failed to load updated offer")
		}

		outboxRow, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "offer.updated",
			Payload: map[string]interface{}{
				"offer_id":      offer.ID,
				"wholesaler_id": offer.WholesalerID,
				"name":          offer.Name,
				"producer":      offer.Producer,
				"display_price": offer.DisplayPrice.StringFixed(4),
				"available_qty": offer.AvailableQty,
				"is_active":     offer.IsActive,
			},
		})
		if err != nil {
			return err
		}

		return s.outbox.Notify(ctx, tx, outboxRow.ID)
	}); err != nil {
		return nil, err
	}

	return &offer, nil
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

func errorAs(err error, target *appErr.AppError) bool {
	switch v := err.(type) {
	case appErr.AppError:
		*target = v
		return true
	case *appErr.AppError:
		*target = *v
		return true
	default:
		return false
	}
}
