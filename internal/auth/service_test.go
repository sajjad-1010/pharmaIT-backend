package auth

import (
	"testing"

	"pharmalink/server/internal/config"
	"pharmalink/server/internal/db/model"

	"github.com/google/uuid"
)

func TestGenerateAndParse(t *testing.T) {
	svc := NewService(config.JWTConfig{
		Issuer:           "test",
		AccessSecret:     "access-secret",
		RefreshSecret:    "refresh-secret",
		AccessTTLMinutes: 15,
		RefreshTTLHours:  24,
	})

	userID := uuid.New()
	pair, err := svc.GeneratePair(userID, model.UserRolePharmacy, model.UserStatusActive)
	if err != nil {
		t.Fatalf("GeneratePair error: %v", err)
	}

	accessClaims, err := svc.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken error: %v", err)
	}
	if accessClaims.UserID != userID.String() {
		t.Fatalf("unexpected user id: %s", accessClaims.UserID)
	}
	if accessClaims.Type != string(TokenTypeAccess) {
		t.Fatalf("unexpected token type: %s", accessClaims.Type)
	}

	if _, err := svc.ParseAccessToken(pair.RefreshToken); err == nil {
		t.Fatal("expected parse access token to fail with refresh token")
	}
}

