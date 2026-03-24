package rare

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
	return &Service{db: db, outbox: outboxSvc}
}

type CreateRequestInput struct {
	PharmacyID        uuid.UUID `json:"pharmacy_id"`
	RequestedNameText *string   `json:"requested_name_text"`
	Qty               int       `json:"qty"`
	DeadlineAt        time.Time `json:"deadline_at"`
	Notes             *string   `json:"notes"`
}

func (s *Service) CreateRequest(ctx context.Context, input CreateRequestInput) (*model.RareRequest, error) {
	if input.Qty <= 0 {
		return nil, appErr.BadRequest("INVALID_QTY", "qty must be > 0", nil)
	}
	if strings.TrimSpace(ptrValue(input.RequestedNameText)) == "" {
		return nil, appErr.BadRequest("MISSING_TARGET", "requested_name_text must be provided", nil)
	}
	if time.Now().After(input.DeadlineAt) {
		return nil, appErr.BadRequest("INVALID_DEADLINE", "deadline_at must be in the future", nil)
	}

	row := &model.RareRequest{
		ID:                uuid.New(),
		PharmacyID:        input.PharmacyID,
		RequestedNameText: trimPtr(input.RequestedNameText),
		Qty:               input.Qty,
		DeadlineAt:        input.DeadlineAt,
		Notes:             trimPtr(input.Notes),
		Status:            model.RareRequestStatusOpen,
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return appErr.Internal("failed to create rare request")
		}
		out, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "rare.request_created",
			Payload: map[string]interface{}{
				"rare_request_id": row.ID,
				"pharmacy_id":     row.PharmacyID,
				"requested_name":  row.RequestedNameText,
				"qty":             row.Qty,
				"deadline_at":     row.DeadlineAt,
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
	Status *model.RareRequestStatus
	Limit  int
	Cursor *pagination.Cursor
}

func (s *Service) ListRequests(ctx context.Context, input ListRequestsInput) (pagination.Result[model.RareRequest], error) {
	q := s.db.WithContext(ctx).Model(&model.RareRequest{})
	if input.Status != nil {
		q = q.Where("status = ?", *input.Status)
	}
	if input.Cursor != nil {
		q = q.Where("(deadline_at, id) > (?, ?)", input.Cursor.Timestamp, input.Cursor.ID)
	}
	q = q.Order("deadline_at ASC").Order("id ASC").Limit(input.Limit)

	var rows []model.RareRequest
	if err := q.Find(&rows).Error; err != nil {
		return pagination.Result[model.RareRequest]{}, appErr.Internal("failed to list rare requests")
	}

	return pagination.BuildResult(rows, input.Limit, func(item model.RareRequest) (time.Time, uuid.UUID) {
		return item.DeadlineAt, item.ID
	}), nil
}

type CreateBidInput struct {
	RareRequestID    uuid.UUID `json:"rare_request_id"`
	WholesalerID     uuid.UUID `json:"wholesaler_id"`
	Price            string    `json:"price"`
	Currency         string    `json:"currency"`
	AvailableQty     int       `json:"available_qty"`
	DeliveryETAHours *int      `json:"delivery_eta_hours"`
	Notes            *string   `json:"notes"`
}

func (s *Service) CreateBid(ctx context.Context, input CreateBidInput) (*model.RareBid, error) {
	price, err := decimal.NewFromString(strings.TrimSpace(input.Price))
	if err != nil || price.IsNegative() {
		return nil, appErr.BadRequest("INVALID_PRICE", "price must be non-negative decimal", nil)
	}
	if input.AvailableQty < 0 {
		return nil, appErr.BadRequest("INVALID_AVAILABLE_QTY", "available_qty cannot be negative", nil)
	}

	var req model.RareRequest
	if err := s.db.WithContext(ctx).First(&req, "id = ?", input.RareRequestID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, appErr.NotFound("RARE_REQUEST_NOT_FOUND", "rare request not found")
		}
		return nil, appErr.Internal("failed to query rare request")
	}
	if req.Status != model.RareRequestStatusOpen && req.Status != model.RareRequestStatusInReview {
		return nil, appErr.Conflict("RARE_REQUEST_CLOSED", "rare request is not accepting bids", nil)
	}

	row := &model.RareBid{
		ID:               uuid.New(),
		RareRequestID:    input.RareRequestID,
		WholesalerID:     input.WholesalerID,
		Price:            price,
		Currency:         strings.ToUpper(strings.TrimSpace(input.Currency)),
		AvailableQty:     input.AvailableQty,
		DeliveryETAHours: input.DeliveryETAHours,
		Notes:            trimPtr(input.Notes),
		Status:           model.RareBidStatusSubmitted,
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return appErr.Internal("failed to create bid")
		}
		if req.Status == model.RareRequestStatusOpen {
			if err := tx.Model(&model.RareRequest{}).
				Where("id = ?", req.ID).
				Update("status", model.RareRequestStatusInReview).Error; err != nil {
				return appErr.Internal("failed to update rare request status")
			}
		}

		out, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "rare.bid_created",
			Payload: map[string]interface{}{
				"rare_bid_id":      row.ID,
				"rare_request_id":  row.RareRequestID,
				"wholesaler_id":    row.WholesalerID,
				"price":            row.Price.StringFixed(4),
				"currency":         row.Currency,
				"available_qty":    row.AvailableQty,
				"delivery_eta_hrs": row.DeliveryETAHours,
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

func (s *Service) SelectBid(ctx context.Context, bidID uuid.UUID, pharmacyID uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bid model.RareBid
		if err := tx.First(&bid, "id = ?", bidID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return appErr.NotFound("RARE_BID_NOT_FOUND", "rare bid not found")
			}
			return appErr.Internal("failed to query bid")
		}

		var req model.RareRequest
		if err := tx.First(&req, "id = ?", bid.RareRequestID).Error; err != nil {
			return appErr.Internal("failed to query rare request")
		}
		if req.PharmacyID != pharmacyID {
			return appErr.Forbidden("FORBIDDEN", "you cannot select bid for this request")
		}
		if req.Status == model.RareRequestStatusClosed || req.Status == model.RareRequestStatusCanceled {
			return appErr.Conflict("RARE_REQUEST_CLOSED", "rare request is closed", nil)
		}

		if err := tx.Model(&model.RareBid{}).
			Where("rare_request_id = ? AND id <> ?", req.ID, bid.ID).
			Update("status", model.RareBidStatusRejected).Error; err != nil {
			return appErr.Internal("failed to reject other bids")
		}
		if err := tx.Model(&model.RareBid{}).
			Where("id = ?", bid.ID).
			Update("status", model.RareBidStatusAccepted).Error; err != nil {
			return appErr.Internal("failed to accept selected bid")
		}
		if err := tx.Model(&model.RareRequest{}).
			Where("id = ?", req.ID).
			Update("status", model.RareRequestStatusSelected).Error; err != nil {
			return appErr.Internal("failed to update rare request status")
		}

		out, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "rare.bid_selected",
			Payload: map[string]interface{}{
				"rare_bid_id":     bid.ID,
				"rare_request_id": req.ID,
				"selected_by":     pharmacyID,
				"wholesaler_id":   bid.WholesalerID,
				"price":           bid.Price.StringFixed(4),
				"currency":        bid.Currency,
			},
		})
		if err != nil {
			return err
		}
		if err := s.outbox.Notify(ctx, tx, out.ID); err != nil {
			return fmt.Errorf("notify rare selected event: %w", err)
		}

		return nil
	})
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
