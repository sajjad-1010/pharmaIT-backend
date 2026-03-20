package orders

import (
	"strings"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/http/middleware"
	"pharmalink/server/internal/http/pagination"
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

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler) {
	protected := api.Group("/orders", authMW)
	protected.Post("/", h.create)
	protected.Get("/", h.list)
	protected.Get("/:id", h.getByID)
	protected.Patch("/:id/status", h.updateStatus)
}

// create godoc
// @Summary Create order (Pharmacy)
// @Tags orders
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "order payload"
// @Success 201 {object} OrderSummary
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Router /orders/ [post]
func (h *Handler) create(c *fiber.Ctx) error {
	role, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	if role != string(model.UserRolePharmacy) {
		return response.Fail(c, appErr.Forbidden("FORBIDDEN", "only pharmacy can create order"))
	}

	userIDRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	pharmacyID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid auth user id"))
	}

	var req struct {
		WholesalerID uuid.UUID    `json:"wholesaler_id"`
		Currency     string       `json:"currency"`
		Items        []CreateItem `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid body", nil))
	}

	order, err := h.svc.CreateOrder(c.UserContext(), CreateOrderInput{
		PharmacyID:   pharmacyID,
		WholesalerID: req.WholesalerID,
		Currency:     strings.ToUpper(strings.TrimSpace(req.Currency)),
		Items:        req.Items,
	})
	if err != nil {
		return response.Fail(c, err)
	}

	return response.JSON(c, fiber.StatusCreated, order)
}

// list godoc
// @Summary List orders
// @Tags orders
// @Security BearerAuth
// @Produce json
// @Param limit query int false "limit"
// @Param cursor query string false "cursor"
// @Success 200 {object} OrderSummary
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /orders/ [get]
func (h *Handler) list(c *fiber.Ctx) error {
	userIDRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid auth user id"))
	}
	roleRaw, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	role := model.UserRole(roleRaw)

	limit, err := middleware.ParseLimit(c, 20, 100)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_LIMIT", "invalid limit", nil))
	}

	var cur *pagination.Cursor
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		parsed, err := pagination.Decode(raw)
		if err != nil {
			return response.Fail(c, appErr.BadRequest("INVALID_CURSOR", "invalid cursor", nil))
		}
		cur = &parsed
	}

	input := ListInput{Limit: limit, Cursor: cur}
	switch role {
	case model.UserRolePharmacy:
		input.ForPharmacyID = &userID
	case model.UserRoleWholesaler:
		input.ForWholesalerID = &userID
	case model.UserRoleAdmin:
	default:
		return response.Fail(c, appErr.Forbidden("FORBIDDEN", "role cannot view order list"))
	}

	out, err := h.svc.List(c.UserContext(), input)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, out)
}

// getByID godoc
// @Summary Get order by id
// @Tags orders
// @Security BearerAuth
// @Produce json
// @Param id path string true "order id"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Router /orders/{id} [get]
func (h *Handler) getByID(c *fiber.Ctx) error {
	orderID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_ORDER_ID", "invalid order id", nil))
	}

	userIDRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid auth user id"))
	}
	roleRaw, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	role := model.UserRole(roleRaw)

	order, items, err := h.svc.GetByID(c.UserContext(), orderID, userID, role)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"order": order,
		"items": items,
	})
}

// updateStatus godoc
// @Summary Update order status
// @Tags orders
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "order id"
// @Param request body object true "status payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Router /orders/{id}/status [patch]
func (h *Handler) updateStatus(c *fiber.Ctx) error {
	orderID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_ORDER_ID", "invalid order id", nil))
	}

	userIDRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid auth user id"))
	}
	roleRaw, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	role := model.UserRole(roleRaw)

	var req struct {
		Status model.OrderStatus `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	order, err := h.svc.UpdateStatus(c.UserContext(), orderID, role, userID, req.Status)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, order)
}
