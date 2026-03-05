package manufacturer

import (
	"context"
	"fmt"
	"strings"
	"time"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/http/pagination"
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
	return &Service{
		db:     db,
		outbox: outboxSvc,
	}
}

type CreateRequestInput struct {
	WholesalerID      uuid.UUID  `json:"wholesaler_id"`
	ManufacturerID    uuid.UUID  `json:"manufacturer_id"`
	MedicineID        *uuid.UUID `json:"medicine_id"`
	RequestedNameText *string    `json:"requested_name_text"`
	Qty               int        `json:"qty"`
	NeededBy          *time.Time `json:"needed_by"`
}

func (s *Service) CreateRequest(ctx context.Context, input CreateRequestInput) (*model.ManufacturerRequest, error) {
	if input.Qty <= 0 {
		return nil, appErr.BadRequest("INVALID_QTY", "qty must be > 0", nil)
	}
	if input.MedicineID == nil && strings.TrimSpace(ptrValue(input.RequestedNameText)) == "" {
		return nil, appErr.BadRequest("MISSING_TARGET", "medicine_id or requested_name_text is required", nil)
	}

	row := &model.ManufacturerRequest{
		ID:                uuid.New(),
		WholesalerID:      input.WholesalerID,
		ManufacturerID:    input.ManufacturerID,
		MedicineID:        input.MedicineID,
		RequestedNameText: trimPtr(input.RequestedNameText),
		Qty:               input.Qty,
		NeededBy:          input.NeededBy,
		Status:            model.ManufacturerRequestStatusCreated,
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return appErr.Internal("failed to create manufacturer request")
		}
		out, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "manufacturer.request_created",
			Payload: map[string]interface{}{
				"request_id":       row.ID,
				"wholesaler_id":    row.WholesalerID,
				"manufacturer_id":  row.ManufacturerID,
				"medicine_id":      row.MedicineID,
				"requested_name":   row.RequestedNameText,
				"qty":              row.Qty,
				"status":           row.Status,
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

type ListRequestsInput struct {
	ManufacturerID *uuid.UUID
	WholesalerID   *uuid.UUID
	Status         *model.ManufacturerRequestStatus
	Limit          int
	Cursor         *pagination.Cursor
}

func (s *Service) ListRequests(ctx context.Context, input ListRequestsInput) (pagination.Result[model.ManufacturerRequest], error) {
	q := s.db.WithContext(ctx).Model(&model.ManufacturerRequest{})
	if input.ManufacturerID != nil {
		q = q.Where("manufacturer_id = ?", *input.ManufacturerID)
	}
	if input.WholesalerID != nil {
		q = q.Where("wholesaler_id = ?", *input.WholesalerID)
	}
	if input.Status != nil {
		q = q.Where("status = ?", *input.Status)
	}
	if input.Cursor != nil {
		q = q.Where("(created_at, id) < (?, ?)", input.Cursor.Timestamp, input.Cursor.ID)
	}
	q = q.Order("created_at DESC").Order("id DESC").Limit(input.Limit)

	var rows []model.ManufacturerRequest
	if err := q.Find(&rows).Error; err != nil {
		return pagination.Result[model.ManufacturerRequest]{}, appErr.Internal("failed to list manufacturer requests")
	}

	return pagination.BuildResult(rows, input.Limit, func(item model.ManufacturerRequest) (time.Time, uuid.UUID) {
		return item.CreatedAt, item.ID
	}), nil
}

type CreateQuoteInput struct {
	RequestID      uuid.UUID  `json:"request_id"`
	ManufacturerID uuid.UUID  `json:"manufacturer_id"`
	UnitPriceFinal string     `json:"unit_price_final"`
	Currency       string     `json:"currency"`
	LeadTimeDays   *int       `json:"lead_time_days"`
	ValidUntil     *time.Time `json:"valid_until"`
	Notes          *string    `json:"notes"`
}

func (s *Service) CreateQuote(ctx context.Context, input CreateQuoteInput) (*model.ManufacturerQuote, error) {
	price, err := decimal.NewFromString(strings.TrimSpace(input.UnitPriceFinal))
	if err != nil || price.IsNegative() {
		return nil, appErr.BadRequest("INVALID_PRICE", "unit_price_final must be non-negative decimal", nil)
	}

	var req model.ManufacturerRequest
	if err := s.db.WithContext(ctx).First(&req, "id = ?", input.RequestID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, appErr.NotFound("MANUFACTURER_REQUEST_NOT_FOUND", "request not found")
		}
		return nil, appErr.Internal("failed to query request")
	}
	if req.ManufacturerID != input.ManufacturerID {
		return nil, appErr.Forbidden("FORBIDDEN", "you cannot quote this request")
	}

	row := &model.ManufacturerQuote{
		ID:             uuid.New(),
		RequestID:      input.RequestID,
		ManufacturerID: input.ManufacturerID,
		UnitPriceFinal: price,
		Currency:       strings.ToUpper(strings.TrimSpace(input.Currency)),
		LeadTimeDays:   input.LeadTimeDays,
		ValidUntil:     input.ValidUntil,
		Notes:          trimPtr(input.Notes),
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return appErr.Internal("failed to create manufacturer quote")
		}
		if err := tx.Model(&model.ManufacturerRequest{}).
			Where("id = ?", req.ID).
			Update("status", model.ManufacturerRequestStatusQuoted).Error; err != nil {
			return appErr.Internal("failed to update request status")
		}

		out, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "manufacturer.quote_created",
			Payload: map[string]interface{}{
				"quote_id":          row.ID,
				"request_id":        row.RequestID,
				"manufacturer_id":   row.ManufacturerID,
				"unit_price_final":  row.UnitPriceFinal.StringFixed(4),
				"currency":          row.Currency,
				"lead_time_days":    row.LeadTimeDays,
				"valid_until":       row.ValidUntil,
			},
		})
		if err != nil {
			return err
		}
		if err := s.outbox.Notify(ctx, tx, out.ID); err != nil {
			return fmt.Errorf("notify manufacturer quote event: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return row, nil
}

func ptrValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func trimPtr(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	if t == "" {
		return nil
	}
	return &t
}

