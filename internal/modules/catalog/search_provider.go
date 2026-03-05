package catalog

import (
	"context"
	"strings"

	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/http/pagination"

	"gorm.io/gorm"
)

type SearchProvider interface {
	SearchMedicines(ctx context.Context, input ListMedicinesInput) ([]model.Medicine, error)
}

type PostgresSearchProvider struct {
	db *gorm.DB
}

func NewPostgresSearchProvider(db *gorm.DB) *PostgresSearchProvider {
	return &PostgresSearchProvider{db: db}
}

func (p *PostgresSearchProvider) SearchMedicines(ctx context.Context, input ListMedicinesInput) ([]model.Medicine, error) {
	q := p.db.WithContext(ctx).Model(&model.Medicine{}).Where("is_active = TRUE")

	if trimmed := strings.TrimSpace(input.Query); trimmed != "" {
		like := "%" + trimmed + "%"
		q = q.Where(`
            generic_name ILIKE ?
            OR brand_name ILIKE ?
            OR similarity(generic_name, ?) > 0.2
            OR similarity(brand_name, ?) > 0.2
        `, like, like, trimmed, trimmed)
	}

	if input.Cursor != nil {
		q = q.Where("(updated_at, id) < (?, ?)", input.Cursor.Timestamp, input.Cursor.ID)
	}

	q = q.Order("updated_at DESC").Order("id DESC").Limit(input.Limit)

	var rows []model.Medicine
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Reserved type for future switch to OpenSearch while preserving endpoint contracts.
type OpenSearchProvider struct{}

func (o OpenSearchProvider) SearchMedicines(ctx context.Context, input ListMedicinesInput) ([]model.Medicine, error) {
	return nil, gorm.ErrUnsupportedDriver
}

func decodeCursorOrNil(raw string) (*pagination.Cursor, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	cur, err := pagination.Decode(raw)
	if err != nil {
		return nil, err
	}
	return &cur, nil
}

