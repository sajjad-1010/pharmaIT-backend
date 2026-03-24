package manufacturer

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
	protected := api.Group("/manufacturer-requests", authMW)
	protected.Post("/", h.createRequest)
	protected.Get("/", h.listRequests)
	protected.Post("/:id/quotes", h.createQuote)
}

// createRequest godoc
// @Summary Create manufacturer request (Wholesaler)
// @Tags manufacturer
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "manufacturer request payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Router /manufacturer-requests [post]
func (h *Handler) createRequest(c *fiber.Ctx) error {
	roleRaw, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	if roleRaw != string(model.UserRoleWholesaler) {
		return response.Fail(c, appErr.Forbidden("FORBIDDEN", "only wholesaler can create manufacturer request"))
	}
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	wholesalerID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}

	var req struct {
		ManufacturerID    uuid.UUID `json:"manufacturer_id"`
		RequestedNameText *string   `json:"requested_name_text"`
		Qty               int       `json:"qty"`
		NeededBy          *string   `json:"needed_by"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	var neededBy *time.Time
	if req.NeededBy != nil && strings.TrimSpace(*req.NeededBy) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.NeededBy))
		if err != nil {
			return response.Fail(c, appErr.BadRequest("INVALID_NEEDED_BY", "needed_by must be RFC3339", nil))
		}
		neededBy = &t
	}

	out, err := h.svc.CreateRequest(c.UserContext(), CreateRequestInput{
		WholesalerID:      wholesalerID,
		ManufacturerID:    req.ManufacturerID,
		RequestedNameText: req.RequestedNameText,
		Qty:               req.Qty,
		NeededBy:          neededBy,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, out)
}

// listRequests godoc
// @Summary List manufacturer requests
// @Tags manufacturer
// @Security BearerAuth
// @Produce json
// @Param status query string false "status"
// @Param limit query int false "limit"
// @Param cursor query string false "cursor"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Router /manufacturer-requests [get]
func (h *Handler) listRequests(c *fiber.Ctx) error {
	roleRaw, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	uid, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}

	limit, err := middleware.ParseLimit(c, 20, 100)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_LIMIT", "invalid limit", nil))
	}

	var cursor *pagination.Cursor
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cur, err := pagination.Decode(raw)
		if err != nil {
			return response.Fail(c, appErr.BadRequest("INVALID_CURSOR", "invalid cursor", nil))
		}
		cursor = &cur
	}
	var status *model.ManufacturerRequestStatus
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		s := model.ManufacturerRequestStatus(raw)
		status = &s
	}

	input := ListRequestsInput{
		Status: status,
		Limit:  limit,
		Cursor: cursor,
	}
	switch model.UserRole(roleRaw) {
	case model.UserRoleManufacturer:
		input.ManufacturerID = &uid
	case model.UserRoleWholesaler:
		input.WholesalerID = &uid
	case model.UserRoleAdmin:
	default:
		return response.Fail(c, appErr.Forbidden("FORBIDDEN", "role cannot list manufacturer requests"))
	}

	out, err := h.svc.ListRequests(c.UserContext(), input)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, out)
}

// createQuote godoc
// @Summary Create manufacturer quote (Manufacturer)
// @Tags manufacturer
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "manufacturer request id"
// @Param request body object true "quote payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Router /manufacturer-requests/{id}/quotes [post]
func (h *Handler) createQuote(c *fiber.Ctx) error {
	roleRaw, _ := c.Locals(middleware.CtxKeyUserRole).(string)
	if roleRaw != string(model.UserRoleManufacturer) {
		return response.Fail(c, appErr.Forbidden("FORBIDDEN", "only manufacturer can create quote"))
	}
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	manufacturerID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}
	requestID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_REQUEST_ID", "invalid request id", nil))
	}

	var req struct {
		UnitPriceFinal string  `json:"unit_price_final"`
		Currency       string  `json:"currency"`
		LeadTimeDays   *int    `json:"lead_time_days"`
		ValidUntil     *string `json:"valid_until"`
		Notes          *string `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	var validUntil *time.Time
	if req.ValidUntil != nil && strings.TrimSpace(*req.ValidUntil) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ValidUntil))
		if err != nil {
			return response.Fail(c, appErr.BadRequest("INVALID_VALID_UNTIL", "valid_until must be RFC3339", nil))
		}
		validUntil = &parsed
	}

	out, err := h.svc.CreateQuote(c.UserContext(), CreateQuoteInput{
		RequestID:      requestID,
		ManufacturerID: manufacturerID,
		UnitPriceFinal: req.UnitPriceFinal,
		Currency:       req.Currency,
		LeadTimeDays:   req.LeadTimeDays,
		ValidUntil:     validUntil,
		Notes:          req.Notes,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, out)
}
