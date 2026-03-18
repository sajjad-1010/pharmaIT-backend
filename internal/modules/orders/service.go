package orders

import (
	"context"
	"fmt"
	"time"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/http/pagination"
	"pharmalink/server/internal/modules/inventory"
	"pharmalink/server/internal/modules/outbox"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db           *gorm.DB
	inventorySvc *inventory.Service
	outboxSvc    *outbox.Service
}

func NewService(db *gorm.DB, inventorySvc *inventory.Service, outboxSvc *outbox.Service) *Service {
	return &Service{
		db:           db,
		inventorySvc: inventorySvc,
		outboxSvc:    outboxSvc,
	}
}

type CreateItem struct {
	OfferID uuid.UUID `json:"offer_id"`
	Qty     int       `json:"qty"`
}

type CreateOrderInput struct {
	PharmacyID   uuid.UUID    `json:"pharmacy_id"`
	WholesalerID uuid.UUID    `json:"wholesaler_id"`
	Currency     string       `json:"currency"`
	Items        []CreateItem `json:"items"`
}

func (s *Service) CreateOrder(ctx context.Context, input CreateOrderInput) (*model.Order, error) {
	if input.PharmacyID == uuid.Nil || input.WholesalerID == uuid.Nil {
		return nil, appErr.BadRequest("INVALID_ORDER", "pharmacy_id and wholesaler_id are required", nil)
	}
	if len(input.Items) == 0 {
		return nil, appErr.BadRequest("EMPTY_ORDER", "order items are required", nil)
	}

	order := &model.Order{
		ID:           uuid.New(),
		PharmacyID:   input.PharmacyID,
		WholesalerID: input.WholesalerID,
		Status:       model.OrderStatusCreated,
		Currency:     input.Currency,
		TotalAmount:  decimal.Zero,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return appErr.Internal("failed to create order")
		}

		total := decimal.Zero
		for _, item := range input.Items {
			if item.Qty <= 0 {
				return appErr.BadRequest("INVALID_QTY", "item qty must be > 0", nil)
			}

			var offer model.WholesalerOffer
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&offer, "id = ? AND wholesaler_id = ? AND is_active = TRUE", item.OfferID, input.WholesalerID).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return appErr.NotFound("OFFER_NOT_FOUND", "offer not found for wholesaler")
				}
				return appErr.Internal("failed to lock offer")
			}

			available, err := s.inventorySvc.CurrentAvailable(ctx, tx, offer.WholesalerID, offer.MedicineID)
			if err != nil {
				return err
			}
			if available < item.Qty {
				return appErr.Conflict("INSUFFICIENT_STOCK", "not enough stock", map[string]int{
					"available": available,
					"requested": item.Qty,
				})
			}

			lineTotal := offer.DisplayPrice.Mul(decimal.NewFromInt(int64(item.Qty)))
			orderItem := model.OrderItem{
				ID:         uuid.New(),
				OrderID:    order.ID,
				MedicineID: offer.MedicineID,
				Qty:        item.Qty,
				UnitPrice:  offer.DisplayPrice,
				LineTotal:  lineTotal,
			}
			if err := tx.Create(&orderItem).Error; err != nil {
				return appErr.Internal("failed to create order item")
			}

			refType := "order"
			refID := order.ID
			if _, err := s.inventorySvc.AddMovement(ctx, tx, inventory.MovementInput{
				WholesalerID: offer.WholesalerID,
				MedicineID:   offer.MedicineID,
				Type:         model.InventoryMovementTypeReserved,
				Qty:          item.Qty,
				RefType:      &refType,
				RefID:        &refID,
			}); err != nil {
				return err
			}

			if err := tx.Model(&model.WholesalerOffer{}).
				Where("id = ?", offer.ID).
				Update("available_qty", available-item.Qty).Error; err != nil {
				return appErr.Internal("failed to update offer available qty")
			}

			total = total.Add(lineTotal)

			outInv, err := s.outboxSvc.Write(ctx, tx, outbox.Event{
				Type: "inventory.changed",
				Payload: map[string]interface{}{
					"offer_id":      offer.ID,
					"wholesaler_id": offer.WholesalerID,
					"medicine_id":   offer.MedicineID,
					"available_qty": available - item.Qty,
					"order_id":      order.ID,
				},
			})
			if err != nil {
				return err
			}
			if err := s.outboxSvc.Notify(ctx, tx, outInv.ID); err != nil {
				return fmt.Errorf("notify inventory event: %w", err)
			}
		}

		if err := tx.Model(&model.Order{}).
			Where("id = ?", order.ID).
			Update("total_amount", total).Error; err != nil {
			return appErr.Internal("failed to update order total")
		}
		order.TotalAmount = total

		out, err := s.outboxSvc.Write(ctx, tx, outbox.Event{
			Type: "order.status_changed",
			Payload: map[string]interface{}{
				"order_id":      order.ID,
				"old_status":    nil,
				"new_status":    order.Status,
				"pharmacy_id":   order.PharmacyID,
				"wholesaler_id": order.WholesalerID,
				"total_amount":  order.TotalAmount.StringFixed(4),
				"currency":      order.Currency,
			},
		})
		if err != nil {
			return err
		}
		if err := s.outboxSvc.Notify(ctx, tx, out.ID); err != nil {
			return fmt.Errorf("notify order event: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return order, nil
}

type ListInput struct {
	ForPharmacyID   *uuid.UUID
	ForWholesalerID *uuid.UUID
	Limit           int
	Cursor          *pagination.Cursor
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[model.Order], error) {
	q := s.db.WithContext(ctx).Model(&model.Order{})
	if input.ForPharmacyID != nil {
		q = q.Where("pharmacy_id = ?", *input.ForPharmacyID)
	}
	if input.ForWholesalerID != nil {
		q = q.Where("wholesaler_id = ?", *input.ForWholesalerID)
	}
	if input.Cursor != nil {
		q = q.Where("(created_at, id) < (?, ?)", input.Cursor.Timestamp, input.Cursor.ID)
	}
	q = q.Order("created_at DESC").Order("id DESC").Limit(input.Limit)

	var rows []model.Order
	if err := q.Find(&rows).Error; err != nil {
		return pagination.Result[model.Order]{}, appErr.Internal("failed to list orders")
	}

	return pagination.BuildResult(rows, input.Limit, func(item model.Order) (time.Time, uuid.UUID) {
		return item.CreatedAt, item.ID
	}), nil
}

func (s *Service) GetByID(ctx context.Context, orderID uuid.UUID, requesterID uuid.UUID, role model.UserRole) (*model.Order, []model.OrderItem, error) {
	var order model.Order
	if err := s.db.WithContext(ctx).First(&order, "id = ?", orderID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, appErr.NotFound("ORDER_NOT_FOUND", "order not found")
		}
		return nil, nil, appErr.Internal("failed to query order")
	}

	if role == model.UserRolePharmacy && order.PharmacyID != requesterID {
		return nil, nil, appErr.Forbidden("FORBIDDEN", "order does not belong to pharmacy")
	}
	if role == model.UserRoleWholesaler && order.WholesalerID != requesterID {
		return nil, nil, appErr.Forbidden("FORBIDDEN", "order does not belong to wholesaler")
	}

	var items []model.OrderItem
	if err := s.db.WithContext(ctx).Find(&items, "order_id = ?", orderID).Error; err != nil {
		return nil, nil, appErr.Internal("failed to query order items")
	}
	return &order, items, nil
}

func (s *Service) UpdateStatus(ctx context.Context, orderID uuid.UUID, actorRole model.UserRole, actorID uuid.UUID, next model.OrderStatus) (*model.Order, error) {
	var order model.Order
	if err := s.db.WithContext(ctx).First(&order, "id = ?", orderID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, appErr.NotFound("ORDER_NOT_FOUND", "order not found")
		}
		return nil, appErr.Internal("failed to query order")
	}

	if actorRole == model.UserRoleWholesaler && order.WholesalerID != actorID {
		return nil, appErr.Forbidden("FORBIDDEN", "you cannot update this order")
	}
	if actorRole == model.UserRolePharmacy && order.PharmacyID != actorID {
		return nil, appErr.Forbidden("FORBIDDEN", "you cannot update this order")
	}

	if !isValidTransition(order.Status, next) {
		return nil, appErr.BadRequest("INVALID_ORDER_TRANSITION", "order status transition is invalid", map[string]string{
			"from": string(order.Status),
			"to":   string(next),
		})
	}

	oldStatus := order.Status
	order.Status = next
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).Where("id = ?", order.ID).Update("status", next).Error; err != nil {
			return appErr.Internal("failed to update order status")
		}
		out, err := s.outboxSvc.Write(ctx, tx, outbox.Event{
			Type: "order.status_changed",
			Payload: map[string]interface{}{
				"order_id":      order.ID,
				"old_status":    oldStatus,
				"new_status":    next,
				"pharmacy_id":   order.PharmacyID,
				"wholesaler_id": order.WholesalerID,
			},
		})
		if err != nil {
			return err
		}
		return s.outboxSvc.Notify(ctx, tx, out.ID)
	}); err != nil {
		return nil, err
	}

	return &order, nil
}

func isValidTransition(from, to model.OrderStatus) bool {
	if from == to {
		return true
	}

	allowed := map[model.OrderStatus][]model.OrderStatus{
		model.OrderStatusCreated:   {model.OrderStatusConfirmed, model.OrderStatusCanceled},
		model.OrderStatusConfirmed: {model.OrderStatusPacking, model.OrderStatusCanceled},
		model.OrderStatusPacking:   {model.OrderStatusShipped, model.OrderStatusCanceled},
		model.OrderStatusShipped:   {model.OrderStatusDelivered},
		model.OrderStatusDelivered: {},
		model.OrderStatusCanceled:  {},
	}

	for _, item := range allowed[from] {
		if item == to {
			return true
		}
	}
	return false
}
