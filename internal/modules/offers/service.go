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
	db       *gorm.DB
	redis    *redis.Client
	outbox   *outbox.Service
}

func NewService(db *gorm.DB, redis *redis.Client, outboxSvc *outbox.Service) *Service {
	return &Service{
		db:     db,
		redis:  redis,
		outbox: outboxSvc,
	}
}

type ListInput struct {
	MedicineID *uuid.UUID
	Limit      int
	Cursor     *pagination.Cursor
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[model.WholesalerOffer], error) {
	cacheKey := fmt.Sprintf("offers:medicine=%v:limit=%d:cursor=%v", input.MedicineID, input.Limit, input.Cursor)
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
			var out pagination.Result[model.WholesalerOffer]
			if jsonErr := json.Unmarshal([]byte(cached), &out); jsonErr == nil {
				return out, nil
			}
		}
	}

	q := s.db.WithContext(ctx).Model(&model.WholesalerOffer{}).Where("is_active = TRUE")
	if input.MedicineID != nil {
		q = q.Where("medicine_id = ?", *input.MedicineID)
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
	MedicineID       uuid.UUID `json:"medicine_id"`
	DisplayPrice     string    `json:"display_price"`
	Currency         string    `json:"currency"`
	AvailableQty     int       `json:"available_qty"`
	MinOrderQty      int       `json:"min_order_qty"`
	DeliveryETAHours *int      `json:"delivery_eta_hours"`
	ExpiryDate       *string   `json:"expiry_date"`
	IsActive         *bool     `json:"is_active"`
}

func (s *Service) Create(ctx context.Context, wholesalerID uuid.UUID, input UpsertOfferInput) (*model.WholesalerOffer, error) {
	price, err := decimal.NewFromString(strings.TrimSpace(input.DisplayPrice))
	if err != nil || price.IsNegative() {
		return nil, appErr.BadRequest("INVALID_DISPLAY_PRICE", "display_price must be non-negative decimal", nil)
	}
	if input.MinOrderQty <= 0 {
		input.MinOrderQty = 1
	}
	if input.AvailableQty < 0 {
		return nil, appErr.BadRequest("INVALID_AVAILABLE_QTY", "available_qty cannot be negative", nil)
	}

	offer := &model.WholesalerOffer{
		ID:               uuid.New(),
		WholesalerID:     wholesalerID,
		MedicineID:       input.MedicineID,
		DisplayPrice:     price,
		Currency:         strings.ToUpper(strings.TrimSpace(input.Currency)),
		AvailableQty:     input.AvailableQty,
		MinOrderQty:      input.MinOrderQty,
		DeliveryETAHours: input.DeliveryETAHours,
		IsActive:         true,
	}
	if input.IsActive != nil {
		offer.IsActive = *input.IsActive
	}
	if input.ExpiryDate != nil && strings.TrimSpace(*input.ExpiryDate) != "" {
		if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*input.ExpiryDate)); err == nil {
			offer.ExpiryDate = &parsed
		} else {
			return nil, appErr.BadRequest("INVALID_EXPIRY_DATE", "expiry_date must be YYYY-MM-DD", nil)
		}
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(offer).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "uq_wholesaler_medicine_active_offer") {
				return appErr.Conflict("ACTIVE_OFFER_EXISTS", "active offer already exists for this medicine", nil)
			}
			return appErr.Internal("failed to create offer")
		}

		outboxRow, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "offer.updated",
			Payload: map[string]interface{}{
				"offer_id":       offer.ID,
				"wholesaler_id":  offer.WholesalerID,
				"medicine_id":    offer.MedicineID,
				"display_price":  offer.DisplayPrice.StringFixed(4),
				"available_qty":  offer.AvailableQty,
				"is_active":      offer.IsActive,
				"delivery_hours": offer.DeliveryETAHours,
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
	if strings.TrimSpace(input.DisplayPrice) != "" {
		price, err := decimal.NewFromString(strings.TrimSpace(input.DisplayPrice))
		if err != nil || price.IsNegative() {
			return nil, appErr.BadRequest("INVALID_DISPLAY_PRICE", "display_price must be non-negative decimal", nil)
		}
		updates["display_price"] = price
	}
	if strings.TrimSpace(input.Currency) != "" {
		updates["currency"] = strings.ToUpper(strings.TrimSpace(input.Currency))
	}
	if input.AvailableQty >= 0 {
		updates["available_qty"] = input.AvailableQty
	}
	if input.MinOrderQty > 0 {
		updates["min_order_qty"] = input.MinOrderQty
	}
	if input.DeliveryETAHours != nil {
		updates["delivery_eta_hours"] = *input.DeliveryETAHours
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
				"medicine_id":   offer.MedicineID,
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

