package sse

import (
	"bufio"
	"context"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/http/response"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	stream *StreamHandler
}

func NewHandler(stream *StreamHandler) *Handler {
	return &Handler{stream: stream}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	api.Get("/stream/offers", h.streamOffers)
}

// streamOffers godoc
// @Summary SSE stream for offer and inventory updates
// @Tags stream
// @Produce text/event-stream
// @Success 200 {string} string "SSE stream"
// @Router /stream/offers [get]
func (h *Handler) streamOffers(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		write := func(payload []byte) error {
			_, err := w.Write(payload)
			return err
		}
		flush := func() error {
			return w.Flush()
		}

		_ = h.stream.StreamOffers(context.Background(), write, flush)
	})

	return nil
}

func WriteSSEError(c *fiber.Ctx, code, message string) error {
	return response.Fail(c, appErr.New(fiber.StatusBadRequest, code, message, nil))
}
