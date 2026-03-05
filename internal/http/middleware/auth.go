package middleware

import (
	"strings"

	"pharmalink/server/internal/auth"
	appErr "pharmalink/server/internal/common/errors"

	"github.com/gofiber/fiber/v2"
)

func JWTAuth(authSvc *auth.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		if header == "" {
			return appErr.Unauthorized("UNAUTHORIZED", "missing authorization header")
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return appErr.Unauthorized("UNAUTHORIZED", "invalid authorization scheme")
		}

		claims, err := authSvc.ParseAccessToken(parts[1])
		if err != nil {
			return appErr.Unauthorized("UNAUTHORIZED", "invalid or expired access token")
		}

		c.Locals(CtxKeyUserID, claims.UserID)
		c.Locals(CtxKeyUserRole, claims.Role)
		c.Locals(CtxKeyUserStat, claims.Status)

		return c.Next()
	}
}

