package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/http/pagination"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Service struct {
	db    *gorm.DB
	redis *redis.Client
	search SearchProvider
}

func NewService(db *gorm.DB, redis *redis.Client) *Service {
	return &Service{
		db:     db,
		redis:  redis,
		search: NewPostgresSearchProvider(db),
	}
}

func NewServiceWithSearchProvider(db *gorm.DB, redis *redis.Client, provider SearchProvider) *Service {
	if provider == nil {
		provider = NewPostgresSearchProvider(db)
	}
	return &Service{
		db:     db,
		redis:  redis,
		search: provider,
	}
}

type ListMedicinesInput struct {
	Query  string
	Limit  int
	Cursor *pagination.Cursor
}

type UpsertMedicineInput struct {
	ID             *uuid.UUID `json:"id,omitempty"`
	ManufacturerID uuid.UUID  `json:"manufacturer_id"`
	GenericName    string     `json:"generic_name"`
	BrandName      *string    `json:"brand_name"`
	Form           string     `json:"form"`
	Strength       *string    `json:"strength"`
	PackSize       *string    `json:"pack_size"`
	ATCCode        *string    `json:"atc_code"`
	IsActive       *bool      `json:"is_active"`
}

func (s *Service) ListMedicines(ctx context.Context, input ListMedicinesInput) (pagination.Result[model.Medicine], error) {
	cacheKey := fmt.Sprintf("medicines:query=%s:limit=%d:cursor=%v", strings.ToLower(strings.TrimSpace(input.Query)), input.Limit, input.Cursor)
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
			var out pagination.Result[model.Medicine]
			if jsonErr := json.Unmarshal([]byte(cached), &out); jsonErr == nil {
				return out, nil
			}
		}
	}

	rows, err := s.search.SearchMedicines(ctx, input)
	if err != nil {
		return pagination.Result[model.Medicine]{}, appErr.Internal("failed to query medicines")
	}

	out := pagination.BuildResult(rows, input.Limit, func(item model.Medicine) (time.Time, uuid.UUID) {
		return item.UpdatedAt, item.ID
	})

	if s.redis != nil {
		if payload, err := json.Marshal(out); err == nil {
			_ = s.redis.Set(ctx, cacheKey, payload, 90*time.Second).Err()
		}
	}

	return out, nil
}

func (s *Service) CreateMedicine(ctx context.Context, input UpsertMedicineInput) (*model.Medicine, error) {
	if strings.TrimSpace(input.GenericName) == "" || strings.TrimSpace(input.Form) == "" {
		return nil, appErr.BadRequest("INVALID_MEDICINE", "generic_name and form are required", nil)
	}

	medicine := &model.Medicine{
		ID:             uuid.New(),
		ManufacturerID: input.ManufacturerID,
		GenericName:    strings.TrimSpace(input.GenericName),
		BrandName:      trimPtr(input.BrandName),
		Form:           strings.TrimSpace(input.Form),
		Strength:       trimPtr(input.Strength),
		PackSize:       trimPtr(input.PackSize),
		ATCCode:        trimPtr(input.ATCCode),
		IsActive:       true,
	}
	if input.IsActive != nil {
		medicine.IsActive = *input.IsActive
	}

	if err := s.db.WithContext(ctx).Create(medicine).Error; err != nil {
		return nil, appErr.Internal("failed to create medicine")
	}
	return medicine, nil
}

func (s *Service) UpdateMedicine(ctx context.Context, id uuid.UUID, input UpsertMedicineInput) (*model.Medicine, error) {
	var medicine model.Medicine
	if err := s.db.WithContext(ctx).First(&medicine, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, appErr.NotFound("MEDICINE_NOT_FOUND", "medicine not found")
		}
		return nil, appErr.Internal("failed to query medicine")
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(input.GenericName) != "" {
		updates["generic_name"] = strings.TrimSpace(input.GenericName)
	}
	if input.BrandName != nil {
		updates["brand_name"] = trimPtr(input.BrandName)
	}
	if strings.TrimSpace(input.Form) != "" {
		updates["form"] = strings.TrimSpace(input.Form)
	}
	if input.Strength != nil {
		updates["strength"] = trimPtr(input.Strength)
	}
	if input.PackSize != nil {
		updates["pack_size"] = trimPtr(input.PackSize)
	}
	if input.ATCCode != nil {
		updates["atc_code"] = trimPtr(input.ATCCode)
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	if len(updates) == 0 {
		return &medicine, nil
	}

	if err := s.db.WithContext(ctx).Model(&model.Medicine{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, appErr.Internal("failed to update medicine")
	}

	if err := s.db.WithContext(ctx).First(&medicine, "id = ?", id).Error; err != nil {
		return nil, appErr.Internal("failed to load updated medicine")
	}
	return &medicine, nil
}

func trimPtr(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	if t == "" {
		return nil
	}
	return &t
}
