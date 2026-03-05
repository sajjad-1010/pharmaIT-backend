package payments

import (
	appErr "pharmalink/server/internal/common/errors"
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

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler) {
	api.Post("/payments/webhook", h.webhook)

	protected := api.Group("/payments", authMW)
	protected.Post("/invoice", h.createInvoice)
	protected.Get("/access", h.getAccess)
}

// createInvoice godoc
// @Summary Create payment invoice
// @Tags payments
// @Security BearerAuth
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorEnvelope
// @Router /payments/invoice [post]
func (h *Handler) createInvoice(c *fiber.Ctx) error {
	userRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	userID, err := uuid.Parse(userRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}

	payment, err := h.svc.CreateInvoice(c.UserContext(), userID)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.JSON(c, fiber.StatusCreated, fiber.Map{
		"payment": payment,
		"hint":    "send invoice_id to payment gateway and then call webhook callback",
	})
}

// webhook godoc
// @Summary Payment webhook callback
// @Tags payments
// @Accept json
// @Produce json
// @Param X-Signature header string true "HMAC-SHA256 signature (hex)"
// @Param request body WebhookInput true "webhook payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /payments/webhook [post]
func (h *Handler) webhook(c *fiber.Ctx) error {
	var req struct {
		WebhookInput
		Signature *string `json:"signature,omitempty"` // deprecated compatibility fallback
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid webhook body", nil))
	}

	signature := c.Get("X-Signature")
	if signature == "" && req.Signature != nil {
		signature = *req.Signature
	}
	if signature == "" {
		return response.Fail(c, appErr.BadRequest("MISSING_SIGNATURE", "X-Signature header is required", nil))
	}

	payment, access, err := h.svc.VerifyWebhook(c.UserContext(), req.WebhookInput, signature)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"payment": payment,
		"access":  access,
	})
}

// getAccess godoc
// @Summary Get current access pass
// @Tags payments
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorEnvelope
// @Router /payments/access [get]
func (h *Handler) getAccess(c *fiber.Ctx) error {
	userRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing auth context"))
	}
	userID, err := uuid.Parse(userRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id"))
	}

	access, remaining, err := h.svc.GetAccess(c.UserContext(), userID)
	if err != nil {
		return response.Fail(c, err)
	}
	if access == nil {
		return response.JSON(c, fiber.StatusOK, fiber.Map{
			"has_access":        false,
			"remaining_seconds": 0,
		})
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"has_access":        remaining > 0,
		"access_until":      access.AccessUntil,
		"remaining_seconds": int64(remaining.Seconds()),
	})
}

