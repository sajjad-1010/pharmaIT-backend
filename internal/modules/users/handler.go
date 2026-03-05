package users

import (
	"strings"

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

func (h *Handler) RegisterRoutes(api fiber.Router, authMW fiber.Handler, adminOnly fiber.Handler) {
	authGroup := api.Group("/auth")
	authGroup.Post("/register", h.register)
	authGroup.Post("/login", h.login)
	authGroup.Post("/refresh", h.refresh)
	authGroup.Post("/logout", h.logout)
	authGroup.Get("/me", authMW, h.me)

	admin := api.Group("/admin", authMW, adminOnly)
	admin.Get("/users", h.listUsers)
	admin.Patch("/users/:id/status", h.updateUserStatus)
}

type registerResponse struct {
	ID     string `json:"id"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// register godoc
// @Summary Register new user
// @Description Register PHARMACY/WHOLESALER/MANUFACTURER user with profile
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "register payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /auth/register [post]
func (h *Handler) register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	user, err := h.svc.Register(c.UserContext(), req)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.JSON(c, fiber.StatusCreated, registerResponse{
		ID:     user.ID.String(),
		Role:   string(user.Role),
		Status: string(user.Status),
	})
}

// login godoc
// @Summary Login
// @Description Login with email/phone and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "login payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Router /auth/login [post]
func (h *Handler) login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	tokens, user, err := h.svc.Login(c.UserContext(), req)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"user": fiber.Map{
			"id":     user.ID,
			"email":  user.Email,
			"phone":  user.Phone,
			"role":   user.Role,
			"status": user.Status,
		},
		"tokens": tokens,
	})
}

// refresh godoc
// @Summary Refresh access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "refresh payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Router /auth/refresh [post]
func (h *Handler) refresh(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		return response.Fail(c, appErr.BadRequest("REFRESH_TOKEN_REQUIRED", "refresh_token is required", nil))
	}

	pair, err := h.svc.Refresh(c.UserContext(), req.RefreshToken)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, pair)
}

// logout godoc
// @Summary Logout
// @Tags auth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /auth/logout [post]
func (h *Handler) logout(c *fiber.Ctx) error {
	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"ok": true,
	})
}

// me godoc
// @Summary Current user profile
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorEnvelope
// @Router /auth/me [get]
func (h *Handler) me(c *fiber.Ctx) error {
	userIDRaw, err := middleware.MustCurrentUserID(c)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "missing user context"))
	}

	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return response.Fail(c, appErr.Unauthorized("UNAUTHORIZED", "invalid user id in token"))
	}

	user, err := h.svc.Me(c.UserContext(), userID)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"id":         user.ID,
		"email":      user.Email,
		"phone":      user.Phone,
		"role":       user.Role,
		"status":     user.Status,
		"created_at": user.CreatedAt,
	})
}

// updateUserStatus godoc
// @Summary Update user status (Admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "user id"
// @Param request body object true "status payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Router /admin/users/{id}/status [patch]
func (h *Handler) updateUserStatus(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_USER_ID", "invalid user id", nil))
	}

	var req struct {
		Status model.UserStatus `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_BODY", "invalid request body", nil))
	}

	user, err := h.svc.UpdateStatus(c.UserContext(), id, req.Status)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"id":     user.ID,
		"status": user.Status,
	})
}

// listUsers godoc
// @Summary List users (Admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param role query string false "role"
// @Param status query string false "status"
// @Param limit query int false "limit"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorEnvelope
// @Router /admin/users [get]
func (h *Handler) listUsers(c *fiber.Ctx) error {
	var filter ListUsersFilter

	role := strings.TrimSpace(c.Query("role"))
	if role != "" {
		roleCast := model.UserRole(role)
		filter.Role = &roleCast
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		statusCast := model.UserStatus(status)
		filter.Status = &statusCast
	}
	limit, err := middleware.ParseLimit(c, 50, 200)
	if err != nil {
		return response.Fail(c, appErr.BadRequest("INVALID_LIMIT", "invalid limit", nil))
	}
	filter.Limit = limit

	users, err := h.svc.ListUsers(c.UserContext(), filter)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.JSON(c, fiber.StatusOK, fiber.Map{"items": users})
}
