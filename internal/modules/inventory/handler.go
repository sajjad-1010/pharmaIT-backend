package inventory

import (
	"strings"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/http/middleware"
	"pharmalink/server/internal/http/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler, wholesalerOnly fiber.Handler) {
	protected := api.Group("/inventory", authMW, wholesalerOnly)
	protected.Post("/movements", h.addMovement)
	protected.Get("/stock", h.getStock)
}

// addMovement godoc
// @Summary Add inventory movement (Wholesaler)
// @Tags inventory
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "movement payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /inventory/movements [post]
func (h *Handler) addMovement(c *fiber.Ctx) error {
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	wholesalerID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}

	var req struct {
		OfferID uuid.UUID                   `json:"offer_id"`
		Type    model.InventoryMovementType `json:"type"`
		Qty     int                         `json:"qty"`
		RefType *string                     `json:"ref_type"`
		RefID   *uuid.UUID                  `json:"ref_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}
	if req.OfferID == uuid.Nil {
		return response.Fail(c, appErr.BadRequest("INVALID_OFFER_ID", "offer_id is required", nil))
	}
	switch req.Type {
	case model.InventoryMovementTypeIn,
		model.InventoryMovementTypeOut,
		model.InventoryMovementTypeReserved,
		model.InventoryMovementTypeReleased,
		model.InventoryMovementTypeAdjust:
	default:
		return response.Fail(c, appErr.BadRequest("INVALID_MOVEMENT_TYPE", "invalid inventory movement type", nil))
	}

	row, available, err := h.svc.RecordMovement(c.UserContext(), MovementInput{
		WholesalerID: wholesalerID,
		OfferID:      req.OfferID,
		Type:         req.Type,
		Qty:          req.Qty,
		RefType:      req.RefType,
		RefID:        req.RefID,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, fiber.Map{
		"movement":      row,
		"available_qty": available,
	})
}

// getStock godoc
// @Summary Get current stock by offer (Wholesaler)
// @Tags inventory
// @Security BearerAuth
// @Produce json
// @Param offer_id query string true "offer id"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /inventory/stock [get]
func (h *Handler) getStock(c *fiber.Ctx) error {
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	wholesalerID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}
	raw := strings.TrimSpace(c.Query("offer_id"))
	if raw == "" {
		return response.Fail(c, appErr.BadRequest("INVALID_OFFER_ID", "offer_id is required", nil))
	}
	offerID, err := uuid.Parse(raw)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_OFFER_ID", "offer_id is invalid", nil))
	}

	available, err := h.svc.CurrentAvailable(c.UserContext(), nil, wholesalerID, offerID)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"wholesaler_id": wholesalerID,
		"offer_id":      offerID,
		"available_qty": available,
	})
}
