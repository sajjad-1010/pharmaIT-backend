package middleware

import (
	"pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/http/response"

	"github.com/gofiber/fiber/v2"
)

func ErrorHandler() fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		if err == nil {
			return nil
		}

		if appErr, ok := err.(errors.AppError); ok {
			return response.Fail(c, appErr)
		}

		if fe, ok := err.(*fiber.Error); ok {
			return response.Fail(c, errors.New(fe.Code, "HTTP_ERROR", fe.Message, nil))
		}

		return response.Fail(c, errors.Internal("internal server error"))
	}
}

