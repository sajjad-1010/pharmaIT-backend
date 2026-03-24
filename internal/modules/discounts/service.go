package discounts

import (
	"context"
	"strings"
	"time"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/modules/outbox"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	outbox *outbox.Service
}

func NewService(db *gorm.DB, outboxSvc *outbox.Service) *Service {
	return &Service{db: db, outbox: outboxSvc}
}

type CreateCampaignInput struct {
	WholesalerID uuid.UUID                    `json:"wholesaler_id"`
	Title        string                       `json:"title"`
	StartsAt     *time.Time                   `json:"starts_at"`
	EndsAt       *time.Time                   `json:"ends_at"`
	Status       model.DiscountCampaignStatus `json:"status"`
}

func (s *Service) CreateCampaign(ctx context.Context, input CreateCampaignInput) (*model.DiscountCampaign, error) {
	if strings.TrimSpace(input.Title) == "" {
		return nil, appErr.BadRequest("INVALID_TITLE", "title is required", nil)
	}
	status := input.Status
	if status == "" {
		status = model.DiscountCampaignStatusDraft
	}

	row := &model.DiscountCampaign{
		ID:           uuid.New(),
		WholesalerID: input.WholesalerID,
		Title:        strings.TrimSpace(input.Title),
		StartsAt:     input.StartsAt,
		EndsAt:       input.EndsAt,
		Status:       status,
	}

	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, appErr.Internal("failed to create discount campaign")
	}
	return row, nil
}

type CreateItemInput struct {
	CampaignID    uuid.UUID          `json:"campaign_id"`
	OfferID       uuid.UUID          `json:"offer_id"`
	DiscountType  model.DiscountType `json:"discount_type"`
	DiscountValue string             `json:"discount_value"`
}

func (s *Service) AddItem(ctx context.Context, wholesalerID uuid.UUID, input CreateItemInput) (*model.DiscountItem, error) {
	var campaign model.DiscountCampaign
	if err := s.db.WithContext(ctx).First(&campaign, "id = ?", input.CampaignID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, appErr.NotFound("CAMPAIGN_NOT_FOUND", "campaign not found")
		}
		return nil, appErr.Internal("failed to query campaign")
	}
	if campaign.WholesalerID != wholesalerID {
		return nil, appErr.Forbidden("FORBIDDEN", "campaign does not belong to wholesaler")
	}

	value, err := decimal.NewFromString(strings.TrimSpace(input.DiscountValue))
	if err != nil || value.IsNegative() {
		return nil, appErr.BadRequest("INVALID_DISCOUNT_VALUE", "discount_value must be non-negative decimal", nil)
	}
	switch input.DiscountType {
	case model.DiscountTypePercent, model.DiscountTypeFixed:
	default:
		return nil, appErr.BadRequest("INVALID_DISCOUNT_TYPE", "discount_type must be PERCENT or FIXED", nil)
	}

	row := &model.DiscountItem{
		ID:            uuid.New(),
		CampaignID:    input.CampaignID,
		OfferID:       input.OfferID,
		DiscountType:  input.DiscountType,
		DiscountValue: value,
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return appErr.Internal("failed to create discount item")
		}
		out, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "offer.updated",
			Payload: map[string]interface{}{
				"campaign_id":    row.CampaignID,
				"offer_id":       row.OfferID,
				"discount_type":  row.DiscountType,
				"discount_value": row.DiscountValue.StringFixed(4),
			},
		})
		if err != nil {
			return err
		}
		return s.outbox.Notify(ctx, tx, out.ID)
	}); err != nil {
		return nil, err
	}

	return row, nil
}

func (s *Service) ListCampaigns(ctx context.Context, wholesalerID uuid.UUID) ([]model.DiscountCampaign, error) {
	var rows []model.DiscountCampaign
	if err := s.db.WithContext(ctx).
		Where("wholesaler_id = ?", wholesalerID).
		Order("id DESC").
		Find(&rows).Error; err != nil {
		return nil, appErr.Internal("failed to list campaigns")
	}
	return rows, nil
}
