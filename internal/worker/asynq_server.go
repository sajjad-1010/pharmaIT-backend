package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"pharmalink/server/internal/asynqjobs"
	"pharmalink/server/internal/config"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

type AsynqRunner struct {
	cfg       config.RedisConfig
	processor *OutboxProcessor
	log       zerolog.Logger
}

func NewAsynqRunner(cfg config.RedisConfig, processor *OutboxProcessor, log zerolog.Logger) *AsynqRunner {
	return &AsynqRunner{
		cfg:       cfg,
		processor: processor,
		log:       log,
	}
}

func (r *AsynqRunner) Run(ctx context.Context, concurrency int) error {
	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     r.cfg.Addr,
			Password: r.cfg.Password,
			DB:       r.cfg.DB,
		},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"critical": 10,
				"default":  8,
				"low":      3,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(asynqjobs.TaskOutboxRetry, func(ctx context.Context, task *asynq.Task) error {
		var payload asynqjobs.OutboxRetryPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("decode outbox retry payload: %w", err)
		}
		return r.processor.HandleRetryTask(ctx, payload)
	})
	mux.HandleFunc(asynqjobs.TaskCacheInvalidate, func(ctx context.Context, task *asynq.Task) error {
		var payload asynqjobs.CacheInvalidatePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("decode cache invalidate payload: %w", err)
		}
		return r.processor.HandleCacheInvalidateTask(ctx, payload)
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(mux)
	}()

	select {
	case <-ctx.Done():
		server.Shutdown()
		return nil
	case err := <-errCh:
		return err
	}
}

