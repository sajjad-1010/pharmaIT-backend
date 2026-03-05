package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"pharmalink/server/internal/config"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
)

type OutboxListener struct {
	cfg       config.PostgresConfig
	channel   string
	processor *OutboxProcessor
	log       zerolog.Logger
}

func NewOutboxListener(
	cfg config.PostgresConfig,
	channel string,
	processor *OutboxProcessor,
	log zerolog.Logger,
) *OutboxListener {
	return &OutboxListener{
		cfg:       cfg,
		channel:   channel,
		processor: processor,
		log:       log,
	}
}

func (l *OutboxListener) Run(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, l.cfg.URL())
	if err != nil {
		return fmt.Errorf("connect for LISTEN: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "LISTEN "+l.channel); err != nil {
		return fmt.Errorf("LISTEN %s failed: %w", l.channel, err)
	}

	l.log.Info().Str("channel", l.channel).Msg("outbox listener started")

	go l.processor.StartSafetyPolling(ctx, 20*time.Second)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			notification, err := conn.WaitForNotification(waitCtx)
			cancel()
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "timeout") {
					continue
				}
				l.log.Error().Err(err).Msg("wait for notification failed")
				continue
			}

			id, err := uuid.Parse(strings.TrimSpace(notification.Payload))
			if err != nil {
				l.log.Warn().Err(err).Str("payload", notification.Payload).Msg("invalid outbox notification payload")
				continue
			}
			if err := l.processor.ProcessByID(ctx, id); err != nil {
				l.log.Error().Err(err).Str("outbox_id", id.String()).Msg("failed processing notified outbox row")
			}
		}
	}
}
