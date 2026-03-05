package rare

import (
	"strings"
	"time"

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
	protected := api.Group("/rare-requests", authMW)
	protected.Post("/", h.createRequest)
	protected.Get("/", h.listRequests)
	protected.Post("/:id/bids", h.createBid)
	protected.Post("/bids/:id/select", h.selectBid)
}

// createRequest godoc
// @Summary Create rare request (Pharmacy)
// @Tags rare
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "rare request payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Router /rare-requests [post]
func (h *Handler) createRequest(c *fiber.Ctx) error {
	roleRaw, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	if roleRaw != string(model.UserRolePharmacy) {
		return response.Fail(c, appErr.Forbidden("FORBIDDEN", "only pharmacy can create rare request"))
	}
	userIDRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	pharmacyID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}

	var req struct {
		MedicineID        *uuid.UUID `json:"medicine_id"`
		RequestedNameText *string    `json:"requested_name_text"`
		Qty               int        `json:"qty"`
		DeadlineAt        string     `json:"deadline_at"`
		Notes             *string    `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	deadline, err := time.Parse(time.RFC3339, req.DeadlineAt)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_DEADLINE", "deadline_at must be RFC3339", nil))
	}

	out, err := h.svc.CreateRequest(c.UserContext(), CreateRequestInput{
		PharmacyID:        pharmacyID,
		MedicineID:        req.MedicineID,
		RequestedNameText: req.RequestedNameText,
		Qty:               req.Qty,
		DeadlineAt:        deadline,
		Notes:             req.Notes,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, out)
}

// listRequests godoc
// @Summary List rare requests
// @Tags rare
// @Security BearerAuth
// @Produce json
// @Param status query string false "status"
// @Param limit query int false "limit"
// @Param cursor query string false "cursor"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /rare-requests [get]
func (h *Handler) listRequests(c *fiber.Ctx) error {
	limit, err := middleware.ParseLimit(c, 20, 100)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_LIMIT", "invalid limit", nil))
	}

	var status *model.RareRequestStatus
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		s := model.RareRequestStatus(raw)
		status = &s
	}

	var cursor *pagination.Cursor
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cur, err := pagination.Decode(raw)
		if err != nil {
			return response.Fail(c, appErr.BadRequest("INVALID_CURSOR", "invalid cursor", nil))
		}
		cursor = &cur
	}

	result, err := h.svc.ListRequests(c.UserContext(), ListRequestsInput{
		Status: status,
		Limit:  limit,
		Cursor: cursor,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, result)
}

// createBid godoc
// @Summary Create rare bid (Wholesaler)
// @Tags rare
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "rare request id"
// @Param request body object true "rare bid payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Router /rare-requests/{id}/bids [post]
func (h *Handler) createBid(c *fiber.Ctx) error {
	roleRaw, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	if roleRaw != string(model.UserRoleWholesaler) {
		return response.Fail(c, appErr.Forbidden("FORBIDDEN", "only wholesaler can create bids"))
	}
	userIDRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	wholesalerID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}
	rareRequestID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_RARE_REQUEST_ID", "invalid rare request id", nil))
	}

	var req struct {
		Price            string  `json:"price"`
		Currency         string  `json:"currency"`
		AvailableQty     int     `json:"available_qty"`
		DeliveryETAHours *int    `json:"delivery_eta_hours"`
		Notes            *string `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	out, err := h.svc.CreateBid(c.UserContext(), CreateBidInput{
		RareRequestID:    rareRequestID,
		WholesalerID:     wholesalerID,
		Price:            req.Price,
		Currency:         req.Currency,
		AvailableQty:     req.AvailableQty,
		DeliveryETAHours: req.DeliveryETAHours,
		Notes:            req.Notes,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, out)
}

// selectBid godoc
// @Summary Select rare bid (Pharmacy)
// @Tags rare
// @Security BearerAuth
// @Produce json
// @Param id path string true "rare bid id"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Router /rare-requests/bids/{id}/select [post]
func (h *Handler) selectBid(c *fiber.Ctx) error {
	roleRaw, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	if roleRaw != string(model.UserRolePharmacy) {
		return response.Fail(c, appErr.Forbidden("FORBIDDEN", "only pharmacy can select bid"))
	}
	userIDRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	pharmacyID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}
	bidID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_RARE_BID_ID", "invalid rare bid id", nil))
	}

	if err := h.svc.SelectBid(c.UserContext(), bidID, pharmacyID); err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, fiber.Map{"ok": true})
}

