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

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler, wholesalerOnly fiber.Handler) {
	protected := api.Group("/discount-campaigns", authMW, wholesalerOnly)
	protected.Post("/", h.createCampaign)
	protected.Get("/", h.listCampaigns)
	protected.Post("/:id/items", h.addItem)
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
		MedicineID    uuid.UUID          `json:"medicine_id"`
		DiscountType  model.DiscountType `json:"discount_type"`
		DiscountValue string             `json:"discount_value"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	out, err := h.svc.AddItem(c.UserContext(), wholesalerID, CreateItemInput{
		CampaignID:    campaignID,
		MedicineID:    req.MedicineID,
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

