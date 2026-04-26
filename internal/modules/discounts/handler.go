package discounts

import (
	"strings"
	"time"

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

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler, wholesalerOnly fiber.Handler, pharmacyOnly fiber.Handler) {
	wholesaler := api.Group("/discount-campaigns", authMW, wholesalerOnly)
	wholesaler.Post("/", h.createCampaign)
	wholesaler.Get("/", h.listCampaigns)
	wholesaler.Post("/:id/items", h.addItem)
	wholesaler.Get("/:id/join-requests", h.listJoinRequests)

	pharmacy := api.Group("/discount-campaigns", authMW, pharmacyOnly)
	pharmacy.Post("/:id/join-requests", h.sendJoinRequest)
}

// createCampaign godoc
// @Summary Create discount campaign (Wholesaler)
// @Tags discounts
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "campaign payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /discount-campaigns/ [post]
func (h *Handler) createCampaign(c *fiber.Ctx) error {
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	wholesalerID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}

	var req struct {
		Title    string                       `json:"title"`
		StartsAt *string                      `json:"starts_at"`
		EndsAt   *string                      `json:"ends_at"`
		Status   model.DiscountCampaignStatus `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	startsAt, err := parseOptionalRFC3339(req.StartsAt)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_STARTS_AT", "starts_at must be RFC3339", nil))
	}
	endsAt, err := parseOptionalRFC3339(req.EndsAt)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_ENDS_AT", "ends_at must be RFC3339", nil))
	}

	out, err := h.svc.CreateCampaign(c.UserContext(), CreateCampaignInput{
		WholesalerID: wholesalerID,
		Title:        req.Title,
		StartsAt:     startsAt,
		EndsAt:       endsAt,
		Status:       req.Status,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, out)
}

// addItem godoc
// @Summary Add item to discount campaign (Wholesaler)
// @Tags discounts
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "campaign id"
// @Param request body object true "discount item payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /discount-campaigns/{id}/items [post]
func (h *Handler) addItem(c *fiber.Ctx) error {
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	wholesalerID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}
	campaignID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_CAMPAIGN_ID", "invalid campaign id", nil))
	}

	var req struct {
		OfferID       uuid.UUID          `json:"offer_id"`
		DiscountType  model.DiscountType `json:"discount_type"`
		DiscountValue string             `json:"discount_value"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	out, err := h.svc.AddItem(c.UserContext(), wholesalerID, CreateItemInput{
		CampaignID:    campaignID,
		OfferID:       req.OfferID,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, out)
}

// listCampaigns godoc
// @Summary List discount campaigns (Wholesaler)
// @Tags discounts
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorEnvelope
// @Router /discount-campaigns/ [get]
func (h *Handler) listCampaigns(c *fiber.Ctx) error {
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	wholesalerID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}

	rows, err := h.svc.ListCampaigns(c.UserContext(), wholesalerID)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, fiber.Map{"items": rows})
}

// sendJoinRequest godoc
// @Summary Send join request for a discount campaign (Pharmacy)
// @Tags discounts
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "campaign id"
// @Param request body object true "join request payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /discount-campaigns/{id}/join-requests [post]
func (h *Handler) sendJoinRequest(c *fiber.Ctx) error {
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	pharmacyID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}
	campaignID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_CAMPAIGN_ID", "invalid campaign id", nil))
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	out, err := h.svc.SendJoinRequest(c.UserContext(), SendJoinRequestInput{
		CampaignID: campaignID,
		PharmacyID: pharmacyID,
		Message:    req.Message,
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusCreated, out)
}

// listJoinRequests godoc
// @Summary List join requests for a campaign (Wholesaler)
// @Tags discounts
// @Security BearerAuth
// @Produce json
// @Param id path string true "campaign id"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Router /discount-campaigns/{id}/join-requests [get]
func (h *Handler) listJoinRequests(c *fiber.Ctx) error {
	uidRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	wholesalerID, err := uuid.Parse(uidRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}
	campaignID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_CAMPAIGN_ID", "invalid campaign id", nil))
	}

	items, err := h.svc.ListJoinRequests(c.UserContext(), wholesalerID, campaignID)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, fiber.Map{"items": items})
}

func parseOptionalRFC3339(input *string) (*time.Time, error) {
	if input == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(*input)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
