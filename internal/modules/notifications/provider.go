package notifications

import (
	"context"
	"errors"
	"strings"

	"pharmalink/server/internal/config"
	"pharmalink/server/internal/db/model"

	"github.com/rs/zerolog"
)

var ErrPushNotConfigured = errors.New("push provider not configured")
var ErrPushTokenInvalid = errors.New("push token invalid")

type PushMessage struct {
	Notification model.Notification
	Device       model.NotificationDevice
}

type PushProvider interface {
	Send(ctx context.Context, msg PushMessage) error
}

type NoopPushProvider struct {
	log zerolog.Logger
}

func NewNoopPushProvider(log zerolog.Logger) *NoopPushProvider {
	return &NoopPushProvider{log: log}
}

func (p *NoopPushProvider) Send(ctx context.Context, msg PushMessage) error {
	p.log.Info().
		Str("user_id", msg.Notification.UserID.String()).
		Str("notification_id", msg.Notification.ID.String()).
		Str("device_id", msg.Device.ID.String()).
		Str("platform", string(msg.Device.Platform)).
		Msg("push delivery skipped because provider is not configured")
	return ErrPushNotConfigured
}

func NewPushProvider(cfg config.NotificationConfig, log zerolog.Logger) PushProvider {
	switch strings.ToLower(strings.TrimSpace(cfg.PushProvider)) {
	case "", "noop":
		return NewNoopPushProvider(log)
	case "fcm":
		provider, err := NewFCMPushProvider(cfg, log)
		if err != nil {
			log.Warn().Err(err).Msg("failed to initialize FCM push provider, falling back to noop")
			return NewNoopPushProvider(log)
		}
		return provider
	default:
		log.Warn().Str("provider", cfg.PushProvider).Msg("unknown push provider configured, falling back to noop")
		return NewNoopPushProvider(log)
	}
}
