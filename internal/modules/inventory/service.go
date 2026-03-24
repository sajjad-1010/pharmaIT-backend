package inventory

import (
	"context"
	"fmt"
	"time"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/modules/outbox"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db        *gorm.DB
	outboxSvc *outbox.Service
}

func NewService(db *gorm.DB, outboxSvc *outbox.Service) *Service {
	return &Service{
		db:        db,
		outboxSvc: outboxSvc,
	}
}

type MovementInput struct {
	WholesalerID uuid.UUID
	OfferID      uuid.UUID
	Type         model.InventoryMovementType
	Qty          int
	RefType      *string
	RefID        *uuid.UUID
}

func (s *Service) RecordMovement(ctx context.Context, input MovementInput) (*model.InventoryMovement, int, error) {
	if input.Qty <= 0 {
		return nil, 0, appErr.BadRequest("INVALID_QTY", "movement qty must be > 0", nil)
	}

	var created *model.InventoryMovement
	var newAvailable int

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		offerLock := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND wholesaler_id = ? AND is_active = TRUE", input.OfferID, input.WholesalerID).
			First(&model.WholesalerOffer{})
		if offerLock.Error != nil && offerLock.Error != gorm.ErrRecordNotFound {
			return appErr.Internal("failed to lock related offer")
		}

		currentBefore, err := s.CurrentAvailable(ctx, tx, input.WholesalerID, input.OfferID)
		if err != nil {
			return err
		}
		if (input.Type == model.InventoryMovementTypeOut || input.Type == model.InventoryMovementTypeReserved) && currentBefore < input.Qty {
			return appErr.Conflict("INSUFFICIENT_STOCK", "insufficient stock for outbound movement", map[string]int{
				"available": currentBefore,
				"requested": input.Qty,
			})
		}

		row, err := s.AddMovement(ctx, tx, input)
		if err != nil {
			return err
		}
		created = row

		available, err := s.CurrentAvailable(ctx, tx, input.WholesalerID, input.OfferID)
		if err != nil {
			return err
		}
		newAvailable = available

		if err := tx.Model(&model.WholesalerOffer{}).
			Where("id = ? AND wholesaler_id = ? AND is_active = TRUE", input.OfferID, input.WholesalerID).
			Update("available_qty", available).Error; err != nil {
			return appErr.Internal("failed to sync offer available qty")
		}

		outboxRow, err := s.outboxSvc.Write(ctx, tx, outbox.Event{
			Type: "inventory.changed",
			Payload: map[string]interface{}{
				"wholesaler_id": input.WholesalerID,
				"offer_id":      input.OfferID,
				"type":          input.Type,
				"qty":           input.Qty,
				"available_qty": available,
				"ref_type":      input.RefType,
				"ref_id":        input.RefID,
			},
		})
		if err != nil {
			return err
		}

		return s.outboxSvc.Notify(ctx, tx, outboxRow.ID)
	})
	if err != nil {
		return nil, 0, err
	}

	return created, newAvailable, nil
}

func (s *Service) AddMovement(ctx context.Context, tx *gorm.DB, input MovementInput) (*model.InventoryMovement, error) {
	if input.Qty <= 0 {
		return nil, appErr.BadRequest("INVALID_QTY", "movement qty must be > 0", nil)
	}

	row := &model.InventoryMovement{
		ID:           uuid.New(),
		WholesalerID: input.WholesalerID,
		OfferID:      input.OfferID,
		Type:         input.Type,
		Qty:          input.Qty,
		RefType:      input.RefType,
		RefID:        input.RefID,
		CreatedAt:    time.Now().UTC(),
	}

	exec := s.db.WithContext(ctx)
	if tx != nil {
		exec = tx.WithContext(ctx)
	}
	if err := exec.Create(row).Error; err != nil {
		return nil, appErr.Internal("failed to insert inventory movement")
	}

	return row, nil
}

func (s *Service) CurrentAvailable(ctx context.Context, tx *gorm.DB, wholesalerID, offerID uuid.UUID) (int, error) {
	exec := s.db.WithContext(ctx)
	if tx != nil {
		exec = tx.WithContext(ctx)
	}

	var available int64
	query := `
        SELECT COALESCE(SUM(
            CASE type
                WHEN 'IN' THEN qty
                WHEN 'RELEASED' THEN qty
                WHEN 'ADJUST' THEN qty
                WHEN 'OUT' THEN -qty
                WHEN 'RESERVED' THEN -qty
                ELSE 0
            END
        ), 0)
        FROM inventory_movements
        WHERE wholesaler_id = ? AND offer_id = ?
    `
	if err := exec.Raw(query, wholesalerID, offerID).Scan(&available).Error; err != nil {
		return 0, appErr.Internal("failed to compute available inventory")
	}

	if available < 0 {
		available = 0
	}
	return int(available), nil
}

type ReserveInput struct {
	OfferID uuid.UUID
	OrderID uuid.UUID
	Qty     int
}

func (s *Service) ReserveForOrder(ctx context.Context, input ReserveInput) error {
	if input.Qty <= 0 {
		return appErr.BadRequest("INVALID_QTY", "qty must be > 0", nil)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var offer model.WholesalerOffer
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&offer, "id = ? AND is_active = TRUE", input.OfferID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return appErr.NotFound("OFFER_NOT_FOUND", "offer not found")
			}
			return appErr.Internal("failed to lock offer for reservation")
		}

		current, err := s.CurrentAvailable(ctx, tx, offer.WholesalerID, offer.ID)
		if err != nil {
			return err
		}
		if current < input.Qty {
			return appErr.Conflict("INSUFFICIENT_STOCK", "not enough stock to reserve", map[string]int{
				"available": current,
				"requested": input.Qty,
			})
		}

		refType := "order"
		refID := input.OrderID
		if _, err := s.AddMovement(ctx, tx, MovementInput{
			WholesalerID: offer.WholesalerID,
			OfferID:      offer.ID,
			Type:         model.InventoryMovementTypeReserved,
			Qty:          input.Qty,
			RefType:      &refType,
			RefID:        &refID,
		}); err != nil {
			return err
		}

		newAvailable := current - input.Qty
		if err := tx.Model(&model.WholesalerOffer{}).
			Where("id = ?", offer.ID).
			Update("available_qty", newAvailable).Error; err != nil {
			return appErr.Internal("failed to update offer available quantity")
		}

		outboxRow, err := s.outboxSvc.Write(ctx, tx, outbox.Event{
			Type: "inventory.changed",
			Payload: map[string]interface{}{
				"offer_id":      offer.ID,
				"wholesaler_id": offer.WholesalerID,
				"available_qty": newAvailable,
				"reason":        "order.reserved",
				"order_id":      input.OrderID,
			},
		})
		if err != nil {
			return err
		}

		if err := s.outboxSvc.Notify(ctx, tx, outboxRow.ID); err != nil {
			return fmt.Errorf("notify outbox: %w", err)
		}

		return nil
	})
}
