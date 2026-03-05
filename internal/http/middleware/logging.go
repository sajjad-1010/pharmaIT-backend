package middleware

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

func Logging(log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		requestID, _ := c.Locals(CtxKeyRequestID).(string)
		userID, _ := c.Locals(CtxKeyUserID).(string)
		role, _ := c.Locals(CtxKeyUserRole).(string)

		event := log.Info().
			Str("request_id", requestID).
			Str("user_id", userID).
			Str("role", role).
			Str("path", c.Path()).
			Str("method", c.Method()).
			Int("status", c.Response().StatusCode()).
			Int64("latency_ms", latency.Milliseconds()).
			Str("ip", c.IP())

		if err != nil {
			event.Str("error", err.Error())
		}

		event.Msg("http_request")
		return err
	}
}

func Recover(log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error().
					Interface("panic", recovered).
					Str("path", c.Path()).
					Str("method", c.Method()).
					Str("request_id", c.Get(fiber.HeaderXRequestID)).
					Msg("panic recovered")
				err = fiber.NewError(fiber.StatusInternalServerError, "internal error")
			}
		}()
		return c.Next()
	}
}

func RateLimitedLog(log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status := c.Response().StatusCode()
		if status == fiber.StatusTooManyRequests {
			log.Warn().
				Str("request_id", c.Get(fiber.HeaderXRequestID)).
				Str("path", c.Path()).
				Str("method", c.Method()).
				Str("ip", c.IP()).
				Int("status", status).
				Str("retry_after", c.Get(fiber.HeaderRetryAfter)).
				Msg("rate_limited_request")
		}
		return nil
	}
}

func CurrentUserID(c *fiber.Ctx) string {
	userID, _ := c.Locals(CtxKeyUserID).(string)
	return userID
}

func MustCurrentUserID(c *fiber.Ctx) (string, error) {
	userID := CurrentUserID(c)
	if userID == "" {
		return "", fiber.NewError(fiber.StatusUnauthorized, "missing auth context")
	}
	return userID, nil
}

func ParseLimit(c *fiber.Ctx, defaultLimit, maxLimit int) (int, error) {
	limitStr := c.Query("limit")
	if limitStr == "" {
		return defaultLimit, nil
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid limit")
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	return limit, nil
}

