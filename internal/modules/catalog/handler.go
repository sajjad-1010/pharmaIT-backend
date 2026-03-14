package catalog

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

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler, adminOnly fiber.Handler, wholesalerOnly fiber.Handler) {
	api.Get("/medicines", h.list)
	api.Group("/medicines", authMW, wholesalerOnly).Post("/validate", h.validateImport)
	api.Group("/medicine-candidates", authMW, wholesalerOnly).Post("/", h.createCandidate)

	admin := api.Group("/admin/medicines", authMW, adminOnly)
	admin.Post("/", h.create)
	admin.Patch("/:id", h.update)

	adminCandidates := api.Group("/admin/medicine-candidates", authMW, adminOnly)
	adminCandidates.Get("/", h.listCandidates)
	adminCandidates.Post("/:id/approve", h.approveCandidate)
	adminCandidates.Post("/:id/reject", h.rejectCandidate)
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

// validateImport godoc
// @Summary Validate medicine import row (Wholesaler)
// @Tags catalog
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body MedicineImportPayload true "medicine import payload"
// @Success 200 {object} ImportValidationResponse
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /medicines/validate [post]
func (h *Handler) validateImport(c *fiber.Ctx) error {
	var req MedicineImportPayload
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	result, err := h.svc.ValidateMedicineImport(c.UserContext(), req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, result)
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

// createCandidate godoc
// @Summary Submit new medicine candidate for admin review (Wholesaler)
// @Tags catalog
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateMedicineCandidateRequest true "medicine candidate payload"
// @Success 201 {object} MedicineCandidateResponse
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /medicine-candidates/ [post]
func (h *Handler) createCandidate(c *fiber.Ctx) error {
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	wholesalerID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}

	var req CreateMedicineCandidateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	candidate, err := h.svc.CreateMedicineCandidate(c.UserContext(), wholesalerID, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, toMedicineCandidateResponse(*candidate))
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

// listCandidates godoc
// @Summary List medicine candidates waiting for review (Admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param status query string false "candidate status"
// @Param limit query int false "limit"
// @Success 200 {object} ListMedicineCandidatesResponse
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /admin/medicine-candidates/ [get]
func (h *Handler) listCandidates(c *fiber.Ctx) error {
	limit, err := middleware.ParseLimit(c, 50, 200)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_LIMIT", "invalid limit", nil))
	}

	var status *model.MedicineCandidateStatus
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		candidateStatus := model.MedicineCandidateStatus(raw)
		if !isValidMedicineCandidateStatus(candidateStatus) {
			return response.Fail(c, appErr.BadRequest("INVALID_STATUS", "invalid medicine candidate status", nil))
		}
		status = &candidateStatus
	}

	items, err := h.svc.ListMedicineCandidates(c.UserContext(), ListMedicineCandidatesFilter{
		Status: status,
		Limit:  limit,
	})
	if err != nil {
		return response.Fail(c, err)
	}

	resp := ListMedicineCandidatesResponse{Items: make([]MedicineCandidateResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toMedicineCandidateResponse(item))
	}
	return response.JSON(c, fiber.StatusOK, resp)
}

// approveCandidate godoc
// @Summary Approve medicine candidate (Admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "medicine candidate id"
// @Param request body ApproveMedicineCandidateRequest true "approve payload"
// @Success 200 {object} ApproveMedicineCandidateResponse
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /admin/medicine-candidates/{id}/approve [post]
func (h *Handler) approveCandidate(c *fiber.Ctx) error {
	adminID, err := mustCurrentUUID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	candidateID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_MEDICINE_CANDIDATE_ID", "invalid medicine candidate id", nil))
	}

	var req ApproveMedicineCandidateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	result, err := h.svc.ApproveMedicineCandidate(c.UserContext(), adminID, candidateID, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, result)
}

// rejectCandidate godoc
// @Summary Reject medicine candidate (Admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "medicine candidate id"
// @Param request body RejectMedicineCandidateRequest true "reject payload"
// @Success 200 {object} MedicineCandidateResponse
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /admin/medicine-candidates/{id}/reject [post]
func (h *Handler) rejectCandidate(c *fiber.Ctx) error {
	adminID, err := mustCurrentUUID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	candidateID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_MEDICINE_CANDIDATE_ID", "invalid medicine candidate id", nil))
	}

	var req RejectMedicineCandidateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	candidate, err := h.svc.RejectMedicineCandidate(c.UserContext(), adminID, candidateID, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, toMedicineCandidateResponse(*candidate))
}

func mustCurrentUUID(c *fiber.Ctx) (uuid.UUID, error) {
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return uuid.Nil, appErr.Unauthorized("UNAUTHORIZED", "missing auth context")
	}
	uid, err := uuid.Parse(uidRaw)
	if err != nil {
		return uuid.Nil, appErr.Unauthorized("UNAUTHORIZED", "invalid user id")
	}
	return uid, nil
}

func isValidMedicineCandidateStatus(status model.MedicineCandidateStatus) bool {
	switch status {
	case model.MedicineCandidateStatusPending, model.MedicineCandidateStatusApproved, model.MedicineCandidateStatusRejected:
		return true
	default:
		return false
	}
}
