package rbac

import (
	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/http/middleware"

	"github.com/gofiber/fiber/v2"
)

func Allow(roles ...model.UserRole) fiber.Handler {
	allowed := map[string]struct{}{}
	for _, r := range roles {
		allowed[string(r)] = struct{}{}
	}

	return func(c *fiber.Ctx) error {
		role, _ := c.Locals(middleware.CtxKeyUserRole).(string)
		if role == "" {
			return appErr.Unauthorized("UNAUTHORIZED", "missing role in auth context")
		}

		if _, ok := allowed[role]; !ok {
			return appErr.Forbidden("FORBIDDEN", "role is not allowed for this route")
		}

		return c.Next()
	}
}

func AllowAll() fiber.Handler {
	return Allow(
		model.UserRolePharmacy,
		model.UserRoleWholesaler,
		model.UserRoleManufacturer,
		model.UserRoleAdmin,
	)
}
