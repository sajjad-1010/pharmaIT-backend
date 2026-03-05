package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := c.Get(fiber.HeaderXRequestID)
		if requestID == "" {
			requestID = uuid.NewString()
		}

		c.Locals(CtxKeyRequestID, requestID)
		c.Set(fiber.HeaderXRequestID, requestID)

		return c.Next()
	}
}

