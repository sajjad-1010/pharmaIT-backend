package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestWholesalerOfferJSONOmitsInternalFields(t *testing.T) {
	offer := WholesalerOffer{
		ID:           uuid.New(),
		WholesalerID: uuid.New(),
		Name:         "Test product 500mg #20",
		DisplayPrice: decimal.RequireFromString("10.5000"),
		AvailableQty: 12,
		ExpiryDate:   ptrTime(time.Date(2027, 9, 15, 0, 0, 0, 0, time.UTC)),
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	raw, err := json.Marshal(offer)
	if err != nil {
		t.Fatalf("marshal offer: %v", err)
	}

	payload := string(raw)
	if strings.Contains(payload, "MinOrderQty") {
		t.Fatalf("offer JSON must not expose MinOrderQty: %s", payload)
	}
	if strings.Contains(payload, "DeliveryETAHours") {
		t.Fatalf("offer JSON must not expose DeliveryETAHours: %s", payload)
	}
}

func TestRareBidJSONStillExposesDeliveryETAHours(t *testing.T) {
	deliveryHours := 18
	bid := RareBid{
		ID:               uuid.New(),
		RareRequestID:    uuid.New(),
		WholesalerID:     uuid.New(),
		Price:            decimal.RequireFromString("12.0000"),
		Currency:         "TJS",
		AvailableQty:     5,
		DeliveryETAHours: &deliveryHours,
		Status:           RareBidStatusSubmitted,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	raw, err := json.Marshal(bid)
	if err != nil {
		t.Fatalf("marshal rare bid: %v", err)
	}

	if !strings.Contains(string(raw), "DeliveryETAHours") {
		t.Fatalf("rare bid JSON must expose DeliveryETAHours: %s", string(raw))
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
