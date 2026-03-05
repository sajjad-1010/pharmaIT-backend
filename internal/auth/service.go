package auth

import (
	"fmt"
	"time"

	"pharmalink/server/internal/config"
	"pharmalink/server/internal/db/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type Claims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	Status string `json:"status"`
	Type   string `json:"typ"`
	jwt.RegisteredClaims
}

type Pair struct {
	AccessToken          string    `json:"access_token"`
	RefreshToken         string    `json:"refresh_token"`
	AccessTokenExpiresAt time.Time `json:"access_token_expires_at"`
}

type Service struct {
	cfg config.JWTConfig
}

func NewService(cfg config.JWTConfig) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) GeneratePair(userID uuid.UUID, role model.UserRole, status model.UserStatus) (Pair, error) {
	now := time.Now().UTC()
	accessExp := now.Add(s.cfg.AccessTTL())
	refreshExp := now.Add(s.cfg.RefreshTTL())

	accessClaims := Claims{
		UserID: userID.String(),
		Role:   string(role),
		Status: string(status),
		Type:   string(TokenTypeAccess),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExp),
			ID:        uuid.NewString(),
		},
	}

	refreshClaims := Claims{
		UserID: userID.String(),
		Role:   string(role),
		Status: string(status),
		Type:   string(TokenTypeRefresh),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExp),
			ID:        uuid.NewString(),
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.AccessSecret))
	if err != nil {
		return Pair{}, err
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.cfg.RefreshSecret))
	if err != nil {
		return Pair{}, err
	}

	return Pair{
		AccessToken:          accessToken,
		RefreshToken:         refreshToken,
		AccessTokenExpiresAt: accessExp,
	}, nil
}

func (s *Service) ParseAccessToken(token string) (*Claims, error) {
	return s.parse(token, s.cfg.AccessSecret, TokenTypeAccess)
}

func (s *Service) ParseRefreshToken(token string) (*Claims, error) {
	return s.parse(token, s.cfg.RefreshSecret, TokenTypeRefresh)
}

func (s *Service) parse(raw string, secret string, expectedType TokenType) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.Type != string(expectedType) {
		return nil, fmt.Errorf("invalid token type")
	}

	return claims, nil
}

