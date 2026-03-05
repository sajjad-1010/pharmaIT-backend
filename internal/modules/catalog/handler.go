package catalog

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

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler, adminOnly fiber.Handler) {
	api.Get("/medicines", h.list)

	admin := api.Group("/admin/medicines", authMW, adminOnly)
	admin.Post("/", h.create)
	admin.Patch("/:id", h.update)
}

// list godoc
// @Summary List/search medicines
// @Tags catalog
// @Produce json
// @Param query query string false "search query"
// @Param limit query int false "limit"
// @Param cursor query string false "cursor"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Router /medicines [get]
func (h *Handler) list(c *fiber.Ctx) error {
	limit, err := middleware.ParseLimit(c, 20, 100)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_LIMIT", "limit must be valid integer", nil))
	}

	var cur *pagination.Cursor
	cursorRaw := strings.TrimSpace(c.Query("cursor"))
	if cursorRaw != "" {
		parsed, err := pagination.Decode(cursorRaw)
		if err != nil {
			return response.Fail(c, appErr.BadRequest("INVALID_CURSOR", "cursor is invalid", nil))
		}
		cur = &parsed
	}

	items, err := h.svc.ListMedicines(c.UserContext(), ListMedicinesInput{
		Query:  c.Query("query"),
		Limit:  limit,
		Cursor: cur,
	})
	if err != nil {
		return response.Fail(c, err)
	}

	return response.JSON(c, fiber.StatusOK, items)
}

// create godoc
// @Summary Create medicine (Admin)
// @Tags catalog
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body UpsertMedicineInput true "medicine payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Router /admin/medicines/ [post]
func (h *Handler) create(c *fiber.Ctx) error {
	var req UpsertMedicineInput
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}
	if req.ManufacturerID == uuid.Nil {
		return response.Fail(c, appErr.BadRequest("INVALID_MANUFACTURER", "manufacturer_id is required", nil))
	}

	medicine, err := h.svc.CreateMedicine(c.UserContext(), req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, medicine)
}

// update godoc
// @Summary Update medicine (Admin)
// @Tags catalog
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "medicine id"
// @Param request body UpsertMedicineInput true "medicine payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Router /admin/medicines/{id} [patch]
func (h *Handler) update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_MEDICINE_ID", "invalid medicine id", nil))
	}

	var req UpsertMedicineInput
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	medicine, err := h.svc.UpdateMedicine(c.UserContext(), id, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, medicine)
}

