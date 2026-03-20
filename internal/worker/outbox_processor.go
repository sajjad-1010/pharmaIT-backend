package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pharmalink/server/internal/asynqjobs"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/modules/outbox"
	"pharmalink/server/internal/modules/sse"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type OutboxProcessor struct {
	outboxSvc  *outbox.Service
	redis      *redis.Client
	broker     *sse.Broker
	enqueuer   *asynqjobs.Enqueuer
	log        zerolog.Logger
	sseChannel string
}

func NewOutboxProcessor(
	outboxSvc *outbox.Service,
	redisClient *redis.Client,
	broker *sse.Broker,
	enqueuer *asynqjobs.Enqueuer,
	log zerolog.Logger,
) *OutboxProcessor {
	return &OutboxProcessor{
		outboxSvc:  outboxSvc,
		redis:      redisClient,
		broker:     broker,
		enqueuer:   enqueuer,
		log:        log,
		sseChannel: "sse_offers",
	}
}

func (p *OutboxProcessor) ProcessPending(ctx context.Context, limit int) error {
	rows, err := p.outboxSvc.ListPending(ctx, limit)
	if err != nil {
		return err
	}

	for _, row := range rows {
		if err := p.process(ctx, row); err != nil {
			p.log.Error().Err(err).Str("outbox_id", row.ID.String()).Msg("failed to process outbox row")
		}
	}
	return nil
}

func (p *OutboxProcessor) ProcessByID(ctx context.Context, id uuid.UUID) error {
	row, err := p.outboxSvc.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if row.Status != model.OutboxStatusNew {
		return nil
	}
	return p.process(ctx, *row)
}

func (p *OutboxProcessor) process(ctx context.Context, row model.Outbox) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(row.PayloadJSON, &payload); err != nil {
		_ = p.outboxSvc.MarkFailed(ctx, row.ID)
		return fmt.Errorf("decode outbox payload: %w", err)
	}

	if err := p.handleEvent(ctx, row.EventType, payload); err != nil {
		_ = p.outboxSvc.MarkFailed(ctx, row.ID)
		if p.enqueuer != nil {
			_ = p.enqueuer.EnqueueOutboxRetry(ctx, row.ID.String())
		}
		return err
	}

	if err := p.outboxSvc.MarkProcessed(ctx, row.ID); err != nil {
		return err
	}
	return nil
}

func (p *OutboxProcessor) handleEvent(ctx context.Context, eventType string, payload map[string]interface{}) error {
	switch eventType {
	case "offer.updated":
		p.invalidateByPrefix(ctx, "offers:")
		p.invalidateByPrefix(ctx, "medicines:")
		p.publishRealtime(ctx, "offer.updated", payload)
	case "inventory.changed":
		p.invalidateByPrefix(ctx, "offers:")
		p.publishRealtime(ctx, "inventory.changed", payload)
	case "order.status_changed":
		p.invalidateByPrefix(ctx, "orders:")
		p.publishRealtime(ctx, "order.status_changed", payload)
	case "payment.verified", "access.updated":
		p.invalidateByPrefix(ctx, "access:")
	case "rare.bid_created", "rare.bid_selected":
		p.invalidateByPrefix(ctx, "rare:")
	case "manufacturer.quote_created", "manufacturer.request_created":
		p.invalidateByPrefix(ctx, "manufacturer:")
	default:
		p.log.Info().Str("event_type", eventType).Msg("outbox event has no explicit handler")
	}
	return nil
}

func (p *OutboxProcessor) publishRealtime(ctx context.Context, eventType string, payload map[string]interface{}) {
	if p.broker != nil {
		p.broker.Publish(eventType, payload)
	}

	if p.redis != nil {
		packet := map[string]interface{}{
			"event": eventType,
			"data":  payload,
		}
		raw, err := json.Marshal(packet)
		if err == nil {
			_ = p.redis.Publish(ctx, p.sseChannel, raw).Err()
		}
	}
}

func (p *OutboxProcessor) invalidateByPrefix(ctx context.Context, prefix string) {
	if p.redis == nil {
		return
	}
	iter := p.redis.Scan(ctx, 0, prefix+"*", 300).Iterator()
	keys := make([]string, 0, 64)
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 300 {
			_ = p.redis.Del(ctx, keys...).Err()
			keys = keys[:0]
		}
	}
	if len(keys) > 0 {
		_ = p.redis.Del(ctx, keys...).Err()
	}
	if err := iter.Err(); err != nil {
		p.log.Warn().Err(err).Str("prefix", prefix).Msg("redis scan failed")
	}
}

func (p *OutboxProcessor) HandleRetryTask(ctx context.Context, payload asynqjobs.OutboxRetryPayload) error {
	id, err := uuid.Parse(strings.TrimSpace(payload.OutboxID))
	if err != nil {
		return fmt.Errorf("invalid outbox id: %w", err)
	}
	return p.ProcessByID(ctx, id)
}

func (p *OutboxProcessor) HandleCacheInvalidateTask(ctx context.Context, payload asynqjobs.CacheInvalidatePayload) error {
	if p.redis == nil {
		return nil
	}
	if len(payload.Keys) == 0 {
		return nil
	}
	return p.redis.Del(ctx, payload.Keys...).Err()
}

func (p *OutboxProcessor) StartSafetyPolling(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.ProcessPending(ctx, 200); err != nil {
				p.log.Error().Err(err).Msg("outbox safety polling failed")
			}
		}
	}
}
