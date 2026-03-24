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

type OrderSummary struct {
	ID                uuid.UUID          `json:"ID"`
	PharmacyID        uuid.UUID          `json:"PharmacyID"`
	PharmacyName      *string            `json:"PharmacyName,omitempty"`
	PharmacyCity      *string            `json:"PharmacyCity,omitempty"`
	PharmacyAddress   *string            `json:"PharmacyAddress,omitempty"`
	PharmacyLicenseNo *string            `json:"PharmacyLicenseNo,omitempty"`
	PharmacyEmail     *string            `json:"PharmacyEmail,omitempty"`
	PharmacyPhone     *string            `json:"PharmacyPhone,omitempty"`
	WholesalerID      uuid.UUID          `json:"WholesalerID"`
	Status            model.OrderStatus  `json:"Status"`
	TotalAmount       decimal.Decimal    `json:"TotalAmount"`
	Currency          string             `json:"Currency"`
	Items             []OrderItemSummary `json:"Items"`
	CreatedAt         time.Time          `json:"CreatedAt"`
	UpdatedAt         time.Time          `json:"UpdatedAt"`
}

type OrderItemSummary struct {
	ID        uuid.UUID       `json:"ID"`
	OfferID   uuid.UUID       `json:"OfferID"`
	ItemName  string          `json:"ItemName"`
	Producer  *string         `json:"Producer,omitempty"`
	Qty       int             `json:"Qty"`
	UnitPrice decimal.Decimal `json:"UnitPrice"`
	LineTotal decimal.Decimal `json:"LineTotal"`
}

func (s *Service) CreateOrder(ctx context.Context, input CreateOrderInput) (*OrderSummary, error) {
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

			available, err := s.inventorySvc.CurrentAvailable(ctx, tx, offer.WholesalerID, offer.ID)
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
				ID:        uuid.New(),
				OrderID:   order.ID,
				OfferID:   offer.ID,
				ItemName:  offer.Name,
				Producer:  offer.Producer,
				Qty:       item.Qty,
				UnitPrice: offer.DisplayPrice,
				LineTotal: lineTotal,
			}
			if err := tx.Create(&orderItem).Error; err != nil {
				return appErr.Internal("failed to create order item")
			}

			refType := "order"
			refID := order.ID
			if _, err := s.inventorySvc.AddMovement(ctx, tx, inventory.MovementInput{
				WholesalerID: offer.WholesalerID,
				OfferID:      offer.ID,
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
					"name":          offer.Name,
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

	out, err := s.loadOrderSummaries(ctx, []model.Order{*order})
	if err != nil {
		return nil, err
	}
	return &out[0], nil
}

type ListInput struct {
	ForPharmacyID   *uuid.UUID
	ForWholesalerID *uuid.UUID
	Limit           int
	Cursor          *pagination.Cursor
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[OrderSummary], error) {
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
		return pagination.Result[OrderSummary]{}, appErr.Internal("failed to list orders")
	}

	items, err := s.loadOrderSummaries(ctx, rows)
	if err != nil {
		return pagination.Result[OrderSummary]{}, err
	}

	return pagination.BuildResult(items, input.Limit, func(item OrderSummary) (time.Time, uuid.UUID) {
		return item.CreatedAt, item.ID
	}), nil
}

func (s *Service) GetByID(ctx context.Context, orderID uuid.UUID, requesterID uuid.UUID, role model.UserRole) (*OrderSummary, []OrderItemSummary, error) {
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

	out, err := s.loadOrderSummaries(ctx, []model.Order{order})
	if err != nil {
		return nil, nil, err
	}
	return &out[0], out[0].Items, nil
}

func (s *Service) UpdateStatus(ctx context.Context, orderID uuid.UUID, actorRole model.UserRole, actorID uuid.UUID, next model.OrderStatus) (*OrderSummary, error) {
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

	out, err := s.loadOrderSummaries(ctx, []model.Order{order})
	if err != nil {
		return nil, err
	}
	return &out[0], nil
}

func (s *Service) loadOrderSummaries(ctx context.Context, orders []model.Order) ([]OrderSummary, error) {
	if len(orders) == 0 {
		return []OrderSummary{}, nil
	}

	pharmacyIDs := make([]uuid.UUID, 0, len(orders))
	seen := make(map[uuid.UUID]struct{}, len(orders))
	for _, order := range orders {
		if _, ok := seen[order.PharmacyID]; ok {
			continue
		}
		seen[order.PharmacyID] = struct{}{}
		pharmacyIDs = append(pharmacyIDs, order.PharmacyID)
	}

	type pharmacyRow struct {
		UserID    uuid.UUID
		Name      string
		City      *string
		Address   *string
		LicenseNo *string
		Email     *string
		Phone     *string
	}

	var pharmacyRows []pharmacyRow
	if err := s.db.WithContext(ctx).
		Table("pharmacies").
		Select("pharmacies.user_id, pharmacies.name, pharmacies.city, pharmacies.address, pharmacies.license_no, users.email, users.phone").
		Joins("LEFT JOIN users ON users.id = pharmacies.user_id").
		Where("pharmacies.user_id IN ?", pharmacyIDs).
		Scan(&pharmacyRows).Error; err != nil {
		return nil, appErr.Internal("failed to load pharmacy details")
	}

	pharmacyDetails := make(map[uuid.UUID]pharmacyRow, len(pharmacyRows))
	for _, row := range pharmacyRows {
		pharmacyDetails[row.UserID] = row
	}

	orderIDs := make([]uuid.UUID, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
	}

	type orderItemRow struct {
		ID        uuid.UUID
		OrderID   uuid.UUID
		OfferID   uuid.UUID
		ItemName  string
		Producer  *string
		Qty       int
		UnitPrice decimal.Decimal
		LineTotal decimal.Decimal
	}

	var itemRows []orderItemRow
	if err := s.db.WithContext(ctx).
		Table("order_items").
		Select("order_items.id, order_items.order_id, order_items.offer_id, order_items.item_name, order_items.producer, order_items.qty, order_items.unit_price, order_items.line_total").
		Where("order_items.order_id IN ?", orderIDs).
		Order("order_items.id ASC").
		Scan(&itemRows).Error; err != nil {
		return nil, appErr.Internal("failed to load order item details")
	}

	itemsByOrder := make(map[uuid.UUID][]OrderItemSummary, len(orders))
	for _, row := range itemRows {
		itemsByOrder[row.OrderID] = append(itemsByOrder[row.OrderID], OrderItemSummary{
			ID:        row.ID,
			OfferID:   row.OfferID,
			ItemName:  row.ItemName,
			Producer:  row.Producer,
			Qty:       row.Qty,
			UnitPrice: row.UnitPrice,
			LineTotal: row.LineTotal,
		})
	}

	out := make([]OrderSummary, 0, len(orders))
	for _, order := range orders {
		summary := OrderSummary{
			ID:           order.ID,
			PharmacyID:   order.PharmacyID,
			WholesalerID: order.WholesalerID,
			Status:       order.Status,
			TotalAmount:  order.TotalAmount,
			Currency:     order.Currency,
			Items:        itemsByOrder[order.ID],
			CreatedAt:    order.CreatedAt,
			UpdatedAt:    order.UpdatedAt,
		}
		if details, ok := pharmacyDetails[order.PharmacyID]; ok {
			nameCopy := details.Name
			summary.PharmacyName = &nameCopy
			summary.PharmacyCity = details.City
			summary.PharmacyAddress = details.Address
			summary.PharmacyLicenseNo = details.LicenseNo
			summary.PharmacyEmail = details.Email
			summary.PharmacyPhone = details.Phone
		}
		out = append(out, summary)
	}

	return out, nil
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
