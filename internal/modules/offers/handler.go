package offers

import (
	"strings"

	appErr "pharmalink/server/internal/common/errors"
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

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler, wholesalerOnly fiber.Handler) {
	api.Get("/offers", h.list)

	protected := api.Group("/offers", authMW, wholesalerOnly)
	protected.Post("/batch", h.createBatch)
	protected.Post("/", h.create)
	protected.Patch("/:id", h.update)
}

// list godoc
// @Summary List offers
// @Tags offers
// @Produce json
// @Param query query string false "offer name query"
// @Param limit query int false "limit"
// @Param cursor query string false "cursor"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Router /offers [get]
func (h *Handler) list(c *fiber.Ctx) error {
	limit, err := middleware.ParseLimit(c, 20, 100)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_LIMIT", "invalid limit", nil))
	}

	var cursor *pagination.Cursor
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cur, err := pagination.Decode(raw)
		if err != nil {
			return response.Fail(c, appErr.BadRequest("INVALID_CURSOR", "cursor is invalid", nil))
		}
		cursor = &cur
	}

	result, err := h.svc.List(c.UserContext(), ListInput{
		Query:  strings.TrimSpace(c.Query("query")),
		Limit:  limit,
		Cursor: cursor,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, result)
}

// create godoc
// @Summary Create offer (Wholesaler)
// @Tags offers
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body UpsertOfferInput true "offer payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /offers/ [post]
func (h *Handler) create(c *fiber.Ctx) error {
	userID, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}

	wholesalerID, err := uuid.Parse(userID)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid auth user id"))
	}

	var req UpsertOfferInput
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	out, err := h.svc.Create(c.UserContext(), wholesalerID, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, out)
}

// createBatch godoc
// @Summary Create offers in batch (Wholesaler)
// @Tags offers
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body BatchCreateInput true "batch offer payload"
// @Success 201 {object} BatchCreateResult
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /offers/batch [post]
func (h *Handler) createBatch(c *fiber.Ctx) error {
	userID, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}

	wholesalerID, err := uuid.Parse(userID)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid auth user id"))
	}

	var req BatchCreateInput
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	out, err := h.svc.CreateBatch(c.UserContext(), wholesalerID, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, out)
}

// update godoc
// @Summary Update offer (Wholesaler)
// @Tags offers
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "offer id"
// @Param request body UpsertOfferInput true "offer payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Router /offers/{id} [patch]
func (h *Handler) update(c *fiber.Ctx) error {
	userID, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}

	wholesalerID, err := uuid.Parse(userID)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid auth user id"))
	}

	offerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_OFFER_ID", "offer id is invalid", nil))
	}

	var req UpsertOfferInput
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	out, err := h.svc.Update(c.UserContext(), wholesalerID, offerID, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, out)
}
