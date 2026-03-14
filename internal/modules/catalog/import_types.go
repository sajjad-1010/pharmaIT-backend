package catalog

import (
	"time"

	"pharmalink/server/internal/db/model"

	"github.com/google/uuid"
)

type MatchStatus string

const (
	MatchStatusMatched        MatchStatus = "MATCHED"
	MatchStatusAmbiguous      MatchStatus = "AMBIGUOUS"
	MatchStatusSuggestedMatch MatchStatus = "SUGGESTED_MATCH"
	MatchStatusNewMedicine    MatchStatus = "NEW_MEDICINE"
	MatchStatusPendingReview  MatchStatus = "PENDING_REVIEW"
)

type MedicineImportPayload struct {
	GenericName string  `json:"generic_name"`
	BrandName   *string `json:"brand_name"`
	Form        string  `json:"form"`
	Strength    *string `json:"strength"`
	PackSize    *string `json:"pack_size"`
	ATCCode     *string `json:"atc_code"`
}

type NormalizedMedicineInput struct {
	GenericName string  `json:"generic_name"`
	BrandName   *string `json:"brand_name,omitempty"`
	Form        string  `json:"form"`
	Strength    *string `json:"strength,omitempty"`
}

type MedicineSummary struct {
	ID             uuid.UUID `json:"id"`
	ManufacturerID uuid.UUID `json:"manufacturer_id"`
	GenericName    string    `json:"generic_name"`
	BrandName      *string   `json:"brand_name,omitempty"`
	Form           string    `json:"form"`
	Strength       *string   `json:"strength,omitempty"`
	PackSize       *string   `json:"pack_size,omitempty"`
	ATCCode        *string   `json:"atc_code,omitempty"`
	IsActive       bool      `json:"is_active"`
}

type MedicineSuggestion struct {
	Medicine MedicineSummary `json:"medicine"`
	Score    float64         `json:"score"`
}

type PendingCandidateSummary struct {
	ID        uuid.UUID                     `json:"id"`
	Status    model.MedicineCandidateStatus `json:"status"`
	CreatedAt time.Time                     `json:"created_at"`
}

type ImportValidationResponse struct {
	Status            MatchStatus              `json:"status"`
	Normalized        NormalizedMedicineInput  `json:"normalized"`
	MatchedMedicine   *MedicineSummary         `json:"matched_medicine,omitempty"`
	SuggestedMedicine *MedicineSuggestion      `json:"suggested_medicine,omitempty"`
	Candidates        []MedicineSuggestion     `json:"candidates,omitempty"`
	PendingCandidate  *PendingCandidateSummary `json:"pending_candidate,omitempty"`
}

type CreateMedicineCandidateRequest struct {
	GenericName string  `json:"generic_name"`
	BrandName   *string `json:"brand_name"`
	Form        string  `json:"form"`
	Strength    *string `json:"strength"`
	PackSize    *string `json:"pack_size"`
	ATCCode     *string `json:"atc_code"`
	ForceSubmit bool    `json:"force_submit"`
}

type MedicineCandidateResponse struct {
	ID                uuid.UUID                     `json:"id"`
	WholesalerID      uuid.UUID                     `json:"wholesaler_id"`
	GenericName       string                        `json:"generic_name"`
	BrandName         *string                       `json:"brand_name,omitempty"`
	Form              string                        `json:"form"`
	Strength          *string                       `json:"strength,omitempty"`
	PackSize          *string                       `json:"pack_size,omitempty"`
	ATCCode           *string                       `json:"atc_code,omitempty"`
	Status            model.MedicineCandidateStatus `json:"status"`
	MatchedMedicineID *uuid.UUID                    `json:"matched_medicine_id,omitempty"`
	AdminDecisionNote *string                       `json:"admin_decision_note,omitempty"`
	ReviewedBy        *uuid.UUID                    `json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time                    `json:"reviewed_at,omitempty"`
	CreatedAt         time.Time                     `json:"created_at"`
	UpdatedAt         time.Time                     `json:"updated_at"`
}

type ListMedicineCandidatesResponse struct {
	Items []MedicineCandidateResponse `json:"items"`
}

type ApproveMedicineCandidateRequest struct {
	MedicineID     *uuid.UUID `json:"medicine_id,omitempty"`
	ManufacturerID *uuid.UUID `json:"manufacturer_id,omitempty"`
	GenericName    *string    `json:"generic_name,omitempty"`
	BrandName      *string    `json:"brand_name,omitempty"`
	Form           *string    `json:"form,omitempty"`
	Strength       *string    `json:"strength,omitempty"`
	PackSize       *string    `json:"pack_size,omitempty"`
	ATCCode        *string    `json:"atc_code,omitempty"`
	IsActive       *bool      `json:"is_active,omitempty"`
	DecisionNote   *string    `json:"decision_note,omitempty"`
}

type RejectMedicineCandidateRequest struct {
	DecisionNote *string `json:"decision_note,omitempty"`
}

type ApproveMedicineCandidateResponse struct {
	Candidate MedicineCandidateResponse `json:"candidate"`
	Medicine  MedicineSummary           `json:"medicine"`
}
