package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db      *gorm.DB
	channel string
}

func NewService(db *gorm.DB, channel string) *Service {
	return &Service{
		db:      db,
		channel: channel,
	}
}

type Event struct {
	Type    string
	Payload interface{}
}

func (s *Service) Write(ctx context.Context, tx *gorm.DB, event Event) (*model.Outbox, error) {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return nil, appErr.BadRequest("INVALID_OUTBOX_PAYLOAD", "failed to serialize outbox payload", nil)
	}

	row := &model.Outbox{
		ID:          uuid.New(),
		EventType:   event.Type,
		PayloadJSON: payload,
		Status:      model.OutboxStatusNew,
		CreatedAt:   time.Now().UTC(),
	}

	exec := s.db.WithContext(ctx)
	if tx != nil {
		exec = tx.WithContext(ctx)
	}

	if err := exec.Create(row).Error; err != nil {
		return nil, appErr.Internal("failed to insert outbox event")
	}

	if tx == nil {
		if err := s.Notify(ctx, nil, row.ID); err != nil {
			return nil, err
		}
	}

	return row, nil
}

func (s *Service) Notify(ctx context.Context, tx *gorm.DB, outboxID uuid.UUID) error {
	exec := s.db.WithContext(ctx)
	if tx != nil {
		exec = tx.WithContext(ctx)
	}
	return exec.Exec(
		"SELECT pg_notify(?, ?)",
		s.channel,
		outboxID.String(),
	).Error
}

func (s *Service) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).
		Model(&model.Outbox{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       model.OutboxStatusProcessed,
			"processed_at": &now,
		}).Error
}

func (s *Service) MarkFailed(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).
		Model(&model.Outbox{}).
		Where("id = ?", id).
		Update("status", model.OutboxStatusFailed).Error
}

func (s *Service) ListPending(ctx context.Context, limit int) ([]model.Outbox, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []model.Outbox
	if err := s.db.WithContext(ctx).
		Where("status = ?", model.OutboxStatusNew).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, appErr.Internal("failed to list outbox rows")
	}
	return rows, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.Outbox, error) {
	var row model.Outbox
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, appErr.NotFound("OUTBOX_NOT_FOUND", fmt.Sprintf("outbox %s not found", id.String()))
		}
		return nil, appErr.Internal("failed to query outbox")
	}
	return &row, nil
}
