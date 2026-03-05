package asynqjobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TaskCacheInvalidate = "cache:invalidate"
	TaskOutboxRetry     = "outbox:retry"
)

type CacheInvalidatePayload struct {
	Keys []string `json:"keys"`
}

func NewCacheInvalidateTask(payload CacheInvalidatePayload) (*asynq.Task, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskCacheInvalidate, b), nil
}

type OutboxRetryPayload struct {
	OutboxID string `json:"outbox_id"`
}

func NewOutboxRetryTask(payload OutboxRetryPayload) (*asynq.Task, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskOutboxRetry, b), nil
}

type Enqueuer struct {
	client *asynq.Client
}

func NewEnqueuer(client *asynq.Client) *Enqueuer {
	return &Enqueuer{client: client}
}

func (e *Enqueuer) EnqueueCacheInvalidation(ctx context.Context, keys []string) error {
	task, err := NewCacheInvalidateTask(CacheInvalidatePayload{Keys: keys})
	if err != nil {
		return err
	}
	_, err = e.client.EnqueueContext(ctx, task,
		asynq.Queue("default"),
		asynq.MaxRetry(10),
		asynq.Timeout(30*time.Second),
	)
	return err
}

func (e *Enqueuer) EnqueueOutboxRetry(ctx context.Context, outboxID string) error {
	task, err := NewOutboxRetryTask(OutboxRetryPayload{OutboxID: outboxID})
	if err != nil {
		return err
	}
	_, err = e.client.EnqueueContext(ctx, task,
		asynq.Queue("critical"),
		asynq.MaxRetry(15),
		asynq.Timeout(30*time.Second),
		asynq.ProcessIn(10*time.Second),
	)
	return err
}

