package catalog

import (
	"context"
	"strings"
	"time"

	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	suggestionLimit         = 5
	minSuggestionScore      = 0.25
	suggestedConfidenceGap  = 0.12
	suggestedConfidenceHigh = 0.80
)

type scoredMedicineRow struct {
	ID          uuid.UUID `gorm:"column:id"`
	GenericName string    `gorm:"column:generic_name"`
	BrandName   *string   `gorm:"column:brand_name"`
	Form        string    `gorm:"column:form"`
	Strength    *string   `gorm:"column:strength"`
	PackSize    *string   `gorm:"column:pack_size"`
	ATCCode     *string   `gorm:"column:atc_code"`
	IsActive    bool      `gorm:"column:is_active"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
	Score       float64   `gorm:"column:score"`
}

type ListMedicineCandidatesFilter struct {
	Status *model.MedicineCandidateStatus
	Limit  int
}

func (s *Service) ValidateMedicineImport(ctx context.Context, input MedicineImportPayload) (*ImportValidationResponse, error) {
	identity := buildNormalizedMedicineIdentity(input)
	if err := validateNormalizedIdentity(identity); err != nil {
		return nil, err
	}

	resp := &ImportValidationResponse{
		Normalized: identity.Response(),
		Warnings:   []ValidationWarning{},
		Candidates: []MedicineSuggestion{},
	}

	// Exact catalog match must win before any pending/fuzzy logic.
	exactMatches, err := s.findExactMedicineMatches(ctx, identity, nil)
	if err != nil {
		return nil, err
	}
	if len(exactMatches) > 0 {
		resp.Status = MatchStatusMatched
		match := toMedicineSummary(exactMatches[0])
		resp.MatchedMedicine = &match
		return resp, nil
	}

	resp.Warnings = buildValidationWarnings(identity)

	pending, err := s.findPendingCandidate(ctx, identity)
	if err != nil {
		return nil, err
	}
	if pending != nil {
		resp.Status = MatchStatusPendingReview
		resp.PendingCandidate = &PendingCandidateSummary{
			ID:        pending.ID,
			Status:    pending.Status,
			CreatedAt: pending.CreatedAt,
		}
		return resp, nil
	}

	suggestions, err := s.findSuggestedMedicines(ctx, identity, suggestionLimit)
	if err != nil {
		return nil, err
	}
	if len(suggestions) == 0 {
		resp.Status = MatchStatusNewMedicine
		return resp, nil
	}

	resp.Candidates = suggestions
	if shouldSuggestSingleMatch(suggestions) {
		resp.Status = MatchStatusSuggestedMatch
		resp.SuggestedMedicine = &suggestions[0]
		return resp, nil
	}

	resp.Status = MatchStatusAmbiguous
	return resp, nil
}

func (s *Service) CreateMedicineCandidate(ctx context.Context, wholesalerID uuid.UUID, input CreateMedicineCandidateRequest) (*model.MedicineCandidate, error) {
	payload := MedicineImportPayload{
		GenericName: input.GenericName,
		BrandName:   input.BrandName,
		Form:        input.Form,
		Strength:    input.Strength,
		PackSize:    input.PackSize,
		ATCCode:     input.ATCCode,
	}

	validation, err := s.ValidateMedicineImport(ctx, payload)
	if err != nil {
		return nil, err
	}

	switch validation.Status {
	case MatchStatusMatched:
		return nil, appErr.Conflict("MEDICINE_ALREADY_EXISTS", "medicine already exists in catalog", validation)
	case MatchStatusPendingReview:
		return nil, appErr.Conflict("MEDICINE_CANDIDATE_ALREADY_PENDING", "medicine is already waiting for admin review", validation)
	case MatchStatusSuggestedMatch, MatchStatusAmbiguous:
		if !input.ForceSubmit {
			return nil, appErr.Conflict("MEDICINE_MATCH_REVIEW_REQUIRED", "review existing suggestions before submitting as new medicine", validation)
		}
	}

	identity := buildNormalizedMedicineIdentity(payload)
	candidate := &model.MedicineCandidate{
		ID:                    uuid.New(),
		WholesalerID:          wholesalerID,
		GenericName:           strings.TrimSpace(input.GenericName),
		BrandName:             trimPtr(input.BrandName),
		Form:                  strings.TrimSpace(input.Form),
		Strength:              trimPtr(input.Strength),
		PackSize:              trimPtr(input.PackSize),
		ATCCode:               trimPtr(input.ATCCode),
		NormalizedGenericName: identity.GenericName,
		NormalizedBrandName:   identity.BrandName,
		NormalizedForm:        identity.Form,
		NormalizedStrength:    identity.Strength,
		Status:                model.MedicineCandidateStatusPending,
	}

	if err := s.db.WithContext(ctx).Create(candidate).Error; err != nil {
		if mapped := mapPendingCandidateConflict(err); mapped != nil {
			return nil, mapped
		}
		return nil, appErr.Internal("failed to create medicine candidate")
	}

	return candidate, nil
}

func (s *Service) ListMedicineCandidates(ctx context.Context, filter ListMedicineCandidatesFilter) ([]model.MedicineCandidate, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	q := s.db.WithContext(ctx).Model(&model.MedicineCandidate{})
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}

	var rows []model.MedicineCandidate
	if err := q.Order("created_at DESC").Limit(filter.Limit).Find(&rows).Error; err != nil {
		return nil, appErr.Internal("failed to list medicine candidates")
	}
	return rows, nil
}

func (s *Service) ApproveMedicineCandidate(ctx context.Context, adminID, candidateID uuid.UUID, input ApproveMedicineCandidateRequest) (*ApproveMedicineCandidateResponse, error) {
	var response ApproveMedicineCandidateResponse

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidate model.MedicineCandidate
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&candidate, "id = ?", candidateID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return appErr.NotFound("MEDICINE_CANDIDATE_NOT_FOUND", "medicine candidate not found")
			}
			return appErr.Internal("failed to load medicine candidate")
		}
		if candidate.Status != model.MedicineCandidateStatusPending {
			return appErr.Conflict("MEDICINE_CANDIDATE_NOT_PENDING", "medicine candidate is already reviewed", nil)
		}

		var medicine *model.Medicine
		if input.MedicineID != nil && *input.MedicineID != uuid.Nil {
			var existing model.Medicine
			if err := tx.First(&existing, "id = ?", *input.MedicineID).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return appErr.NotFound("MEDICINE_NOT_FOUND", "medicine not found")
				}
				return appErr.Internal("failed to load medicine")
			}
			medicine = &existing
		} else {
			createInput := UpsertMedicineInput{
				GenericName: pickApprovedText(input.GenericName, candidate.GenericName),
				BrandName:   pickApprovedPtr(input.BrandName, candidate.BrandName),
				Form:        pickApprovedText(input.Form, candidate.Form),
				Strength:    pickApprovedPtr(input.Strength, candidate.Strength),
				PackSize:    pickApprovedPtr(input.PackSize, candidate.PackSize),
				ATCCode:     pickApprovedPtr(input.ATCCode, candidate.ATCCode),
				IsActive:    input.IsActive,
			}

			created, err := s.createMedicineWithDB(tx, createInput)
			if err != nil {
				return err
			}
			medicine = created
		}

		decisionNote := trimPtr(input.DecisionNote)
		update := map[string]interface{}{
			"status":              model.MedicineCandidateStatusApproved,
			"matched_medicine_id": medicine.ID,
			"reviewed_by":         adminID,
			"reviewed_at":         time.Now().UTC(),
			"admin_decision_note": decisionNote,
		}
		if err := tx.Model(&model.MedicineCandidate{}).Where("id = ?", candidate.ID).Updates(update).Error; err != nil {
			return appErr.Internal("failed to approve medicine candidate")
		}
		if err := tx.First(&candidate, "id = ?", candidate.ID).Error; err != nil {
			return appErr.Internal("failed to load approved medicine candidate")
		}

		response = ApproveMedicineCandidateResponse{
			Candidate: toMedicineCandidateResponse(candidate),
			Medicine:  toMedicineSummary(*medicine),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *Service) RejectMedicineCandidate(ctx context.Context, adminID, candidateID uuid.UUID, input RejectMedicineCandidateRequest) (*model.MedicineCandidate, error) {
	var candidate model.MedicineCandidate
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&candidate, "id = ?", candidateID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return appErr.NotFound("MEDICINE_CANDIDATE_NOT_FOUND", "medicine candidate not found")
			}
			return appErr.Internal("failed to load medicine candidate")
		}
		if candidate.Status != model.MedicineCandidateStatusPending {
			return appErr.Conflict("MEDICINE_CANDIDATE_NOT_PENDING", "medicine candidate is already reviewed", nil)
		}

		if err := tx.Model(&model.MedicineCandidate{}).Where("id = ?", candidate.ID).Updates(map[string]interface{}{
			"status":              model.MedicineCandidateStatusRejected,
			"reviewed_by":         adminID,
			"reviewed_at":         time.Now().UTC(),
			"admin_decision_note": trimPtr(input.DecisionNote),
		}).Error; err != nil {
			return appErr.Internal("failed to reject medicine candidate")
		}
		if err := tx.First(&candidate, "id = ?", candidate.ID).Error; err != nil {
			return appErr.Internal("failed to load rejected medicine candidate")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &candidate, nil
}

func (s *Service) findExactMedicineMatches(ctx context.Context, identity normalizedMedicineIdentity, excludeID *uuid.UUID) ([]model.Medicine, error) {
	q := s.db.WithContext(ctx).Model(&model.Medicine{}).
		Where("normalize_catalog_text(generic_name) = ?", identity.GenericName).
		Where("COALESCE(normalize_catalog_text(brand_name), '') = ?", normalizedValue(identity.BrandName)).
		Where("normalize_catalog_text(form) = ?", identity.Form).
		Where("COALESCE(normalize_catalog_text(strength), '') = ?", normalizedValue(identity.Strength))
	if excludeID != nil {
		q = q.Where("id <> ?", *excludeID)
	}

	var rows []model.Medicine
	if err := q.Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, appErr.Internal("failed to find exact medicine matches")
	}
	return rows, nil
}

func (s *Service) findPendingCandidate(ctx context.Context, identity normalizedMedicineIdentity) (*model.MedicineCandidate, error) {
	var candidate model.MedicineCandidate
	err := s.db.WithContext(ctx).First(&candidate,
		"status = ? AND normalized_generic_name = ? AND COALESCE(normalized_brand_name, '') = ? AND normalized_form = ? AND COALESCE(normalized_strength, '') = ?",
		model.MedicineCandidateStatusPending,
		identity.GenericName,
		normalizedValue(identity.BrandName),
		identity.Form,
		normalizedValue(identity.Strength),
	).Error
	switch err {
	case nil:
		return &candidate, nil
	case gorm.ErrRecordNotFound:
		return nil, nil
	default:
		return nil, appErr.Internal("failed to query pending medicine candidates")
	}
}

func (s *Service) findSuggestedMedicines(ctx context.Context, identity normalizedMedicineIdentity, limit int) ([]MedicineSuggestion, error) {
	if limit <= 0 {
		limit = suggestionLimit
	}

	brand := normalizedValue(identity.BrandName)
	strength := normalizedValue(identity.Strength)
	args := []interface{}{
		identity.GenericName, identity.GenericName,
		brand, brand,
		identity.Form, identity.Form,
		strength, strength,
	}

	whereParts := make([]string, 0, 4)
	whereArgs := make([]interface{}, 0, 8)
	if identity.GenericName != "" {
		whereParts = append(whereParts, "(COALESCE(normalize_catalog_text(generic_name), '') % ? OR COALESCE(normalize_catalog_text(generic_name), '') LIKE ?)")
		whereArgs = append(whereArgs, identity.GenericName, "%"+identity.GenericName+"%")
	}
	if brand != "" {
		whereParts = append(whereParts, "(COALESCE(normalize_catalog_text(brand_name), '') % ? OR COALESCE(normalize_catalog_text(brand_name), '') LIKE ?)")
		whereArgs = append(whereArgs, brand, "%"+brand+"%")
	}
	if identity.Form != "" {
		whereParts = append(whereParts, "(COALESCE(normalize_catalog_text(form), '') = ? OR COALESCE(normalize_catalog_text(form), '') % ?)")
		whereArgs = append(whereArgs, identity.Form, identity.Form)
	}
	if strength != "" {
		whereParts = append(whereParts, "(COALESCE(normalize_catalog_text(strength), '') = ? OR COALESCE(normalize_catalog_text(strength), '') % ?)")
		whereArgs = append(whereArgs, strength, strength)
	}
	if len(whereParts) == 0 {
		return nil, nil
	}

	var rows []scoredMedicineRow
	selectSQL := `
		medicines.*,
		(
			CASE WHEN ? <> '' THEN similarity(COALESCE(normalize_catalog_text(generic_name), ''), ?) * 0.55 ELSE 0 END +
			CASE WHEN ? <> '' THEN similarity(COALESCE(normalize_catalog_text(brand_name), ''), ?) * 0.25 ELSE 0 END +
			CASE WHEN ? <> '' THEN similarity(COALESCE(normalize_catalog_text(form), ''), ?) * 0.15 ELSE 0 END +
			CASE WHEN ? <> '' THEN similarity(COALESCE(normalize_catalog_text(strength), ''), ?) * 0.05 ELSE 0 END
		) AS score
	`
	if err := s.db.WithContext(ctx).
		Table("medicines").
		Select(selectSQL, args...).
		Where(strings.Join(whereParts, " OR "), whereArgs...).
		Order("score DESC").
		Order("updated_at DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, appErr.Internal("failed to query similar medicines")
	}

	suggestions := make([]MedicineSuggestion, 0, len(rows))
	for _, row := range rows {
		if row.Score < minSuggestionScore {
			continue
		}
		suggestions = append(suggestions, MedicineSuggestion{
			Medicine: toMedicineSummary(row.toMedicine()),
			Score:    row.Score,
		})
	}
	return suggestions, nil
}

func shouldSuggestSingleMatch(candidates []MedicineSuggestion) bool {
	if len(candidates) == 0 {
		return false
	}
	if len(candidates) == 1 {
		return true
	}
	return candidates[0].Score >= suggestedConfidenceHigh && (candidates[0].Score-candidates[1].Score) >= suggestedConfidenceGap
}

func medicineSuggestionsFromMatches(matches []model.Medicine, score float64) []MedicineSuggestion {
	out := make([]MedicineSuggestion, 0, len(matches))
	for _, medicine := range matches {
		out = append(out, MedicineSuggestion{
			Medicine: toMedicineSummary(medicine),
			Score:    score,
		})
	}
	return out
}

func toMedicineSummary(medicine model.Medicine) MedicineSummary {
	return MedicineSummary{
		ID:          medicine.ID,
		GenericName: medicine.GenericName,
		BrandName:   medicine.BrandName,
		Form:        medicine.Form,
		Strength:    medicine.Strength,
		PackSize:    medicine.PackSize,
		ATCCode:     medicine.ATCCode,
		IsActive:    medicine.IsActive,
	}
}

func toMedicineCandidateResponse(candidate model.MedicineCandidate) MedicineCandidateResponse {
	return MedicineCandidateResponse{
		ID:                candidate.ID,
		WholesalerID:      candidate.WholesalerID,
		GenericName:       candidate.GenericName,
		BrandName:         candidate.BrandName,
		Form:              candidate.Form,
		Strength:          candidate.Strength,
		PackSize:          candidate.PackSize,
		ATCCode:           candidate.ATCCode,
		Status:            candidate.Status,
		MatchedMedicineID: candidate.MatchedMedicineID,
		AdminDecisionNote: candidate.AdminDecisionNote,
		ReviewedBy:        candidate.ReviewedBy,
		ReviewedAt:        candidate.ReviewedAt,
		CreatedAt:         candidate.CreatedAt,
		UpdatedAt:         candidate.UpdatedAt,
	}
}

func mapPendingCandidateConflict(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "uq_medicine_candidates_pending_identity") {
		return appErr.Conflict("MEDICINE_CANDIDATE_ALREADY_PENDING", "medicine is already waiting for admin review", nil)
	}
	return nil
}

func pickApprovedText(override *string, fallback string) string {
	if override != nil && strings.TrimSpace(*override) != "" {
		return strings.TrimSpace(*override)
	}
	return fallback
}

func pickApprovedPtr(override, fallback *string) *string {
	if override != nil {
		return trimPtr(override)
	}
	return fallback
}

func (r scoredMedicineRow) toMedicine() model.Medicine {
	return model.Medicine{
		ID:          r.ID,
		GenericName: r.GenericName,
		BrandName:   r.BrandName,
		Form:        r.Form,
		Strength:    r.Strength,
		PackSize:    r.PackSize,
		ATCCode:     r.ATCCode,
		IsActive:    r.IsActive,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func buildValidationWarnings(identity normalizedMedicineIdentity) []ValidationWarning {
	var warnings []ValidationWarning
	if identity.BrandName == nil || strings.TrimSpace(*identity.BrandName) == "" {
		warnings = append(warnings, ValidationWarning{
			Field:   "brand_name",
			Code:    "BRAND_NAME_RECOMMENDED",
			Message: "brand_name should be filled to improve matching accuracy and reduce duplicates",
		})
	}
	return warnings
}
