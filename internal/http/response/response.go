package response

import (
	"pharmalink/server/internal/common/errors"

	"github.com/gofiber/fiber/v2"
)

type ErrorEnvelope struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}

func JSON(c *fiber.Ctx, status int, payload interface{}) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	return c.Status(status).JSON(payload)
}

func Fail(c *fiber.Ctx, err error) error {
	switch v := err.(type) {
	case errors.AppError:
		return failWithAppError(c, v)
	case *errors.AppError:
		return failWithAppError(c, *v)
	default:
		return failWithAppError(c, errors.Internal("unexpected server error"))
	}
}

func failWithAppError(c *fiber.Ctx, appErr errors.AppError) error {
	var envelope ErrorEnvelope
	envelope.Error.Code = appErr.Code
	envelope.Error.Message = appErr.Message
	envelope.Error.Details = appErr.Details
	return c.Status(appErr.HTTPStatus).JSON(envelope)
}

