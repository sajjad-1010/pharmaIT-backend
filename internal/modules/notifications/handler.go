package notifications

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

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler) {
	protected := api.Group("/notifications", authMW)
	protected.Get("/", h.list)
	protected.Get("/devices", h.listDevices)
	protected.Get("/preferences", h.getPreferences)
	protected.Put("/preferences", h.updatePreferences)
	protected.Post("/devices", h.upsertDevice)
	protected.Delete("/devices/:id", h.deleteDevice)
	protected.Post("/:id/read", h.markRead)
	protected.Post("/read-all", h.markAllRead)
}

// list godoc
// @Summary List notifications
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Param limit query int false "limit"
// @Param cursor query string false "cursor"
// @Param unread_only query boolean false "unread only"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /notifications/ [get]
func (h *Handler) list(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	limit, err := middleware.ParseLimit(c, 20, 100)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_LIMIT", "invalid limit", nil))
	}
	var cursor *pagination.Cursor
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		parsed, err := pagination.Decode(raw)
		if err != nil {
			return response.Fail(c, appErr.BadRequest("INVALID_CURSOR", "invalid cursor", nil))
		}
		cursor = &parsed
	}
	result, err := h.svc.List(c.UserContext(), ListInput{
		UserID:     userID,
		Limit:      limit,
		Cursor:     cursor,
		UnreadOnly: strings.EqualFold(strings.TrimSpace(c.Query("unread_only")), "true"),
	})
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, result)
}

// listDevices godoc
// @Summary List notification devices
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {array} model.NotificationDevice
// @Failure 401 {object} response.ErrorEnvelope
// @Router /notifications/devices [get]
func (h *Handler) listDevices(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	items, err := h.svc.ListDevices(c.UserContext(), userID)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, items)
}

// getPreferences godoc
// @Summary Get notification preferences
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} model.NotificationPreference
// @Failure 401 {object} response.ErrorEnvelope
// @Router /notifications/preferences [get]
func (h *Handler) getPreferences(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	pref, err := h.svc.GetPreferences(c.UserContext(), userID)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, pref)
}

// updatePreferences godoc
// @Summary Update notification preferences
// @Tags notifications
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body PreferencePatch true "preferences payload"
// @Success 200 {object} model.NotificationPreference
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /notifications/preferences [put]
func (h *Handler) updatePreferences(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	var req PreferencePatch
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}
	pref, err := h.svc.UpdatePreferences(c.UserContext(), userID, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, pref)
}

// upsertDevice godoc
// @Summary Register or refresh notification device
// @Tags notifications
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body DeviceUpsertInput true "device payload"
// @Success 200 {object} model.NotificationDevice
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /notifications/devices [post]
func (h *Handler) upsertDevice(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	var req DeviceUpsertInput
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}
	row, err := h.svc.UpsertDevice(c.UserContext(), userID, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, row)
}

// deleteDevice godoc
// @Summary Deactivate notification device
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Param id path string true "device id"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Router /notifications/devices/{id} [delete]
func (h *Handler) deleteDevice(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	deviceID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_DEVICE_ID", "invalid device id", nil))
	}
	if err := h.svc.DeactivateDevice(c.UserContext(), userID, deviceID); err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, fiber.Map{"ok": true})
}

// markRead godoc
// @Summary Mark notification as read
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Param id path string true "notification id"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Router /notifications/{id}/read [post]
func (h *Handler) markRead(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	notificationID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_NOTIFICATION_ID", "invalid notification id", nil))
	}
	if err := h.svc.MarkRead(c.UserContext(), userID, notificationID); err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, fiber.Map{"ok": true})
}

// markAllRead godoc
// @Summary Mark all notifications as read
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]bool
// @Failure 401 {object} response.ErrorEnvelope
// @Router /notifications/read-all [post]
func (h *Handler) markAllRead(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return response.Fail(c, err)
	}
	if err := h.svc.MarkAllRead(c.UserContext(), userID); err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func currentUserID(c *fiber.Ctx) (uuid.UUID, error) {
	userIDRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return uuid.Nil, appErr.Unauthorized("UNAUTHORIZED", "missing auth context")
	}
	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return uuid.Nil, appErr.Unauthorized("UNAUTHORIZED", "invalid auth user id")
	}
	return userID, nil
}
