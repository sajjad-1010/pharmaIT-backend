package users

import (
	"context"
	"strings"

	"pharmalink/server/internal/auth"
	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db      *gorm.DB
	authSvc *auth.Service
}

func NewService(db *gorm.DB, authSvc *auth.Service) *Service {
	return &Service{
		db:      db,
		authSvc: authSvc,
	}
}

type RegisterRequest struct {
	Email    *string        `json:"email"`
	Phone    *string        `json:"phone"`
	Password string         `json:"password"`
	Role     model.UserRole `json:"role"`

	Profile RegisterProfile `json:"profile"`
}

type RegisterProfile struct {
	Name           string  `json:"name"`
	City           *string `json:"city"`
	Address        *string `json:"address"`
	Country        *string `json:"country"`
	LicenseNo      *string `json:"license_no"`
	RegistrationNo *string `json:"registration_no"`
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*model.User, error) {
	if req.Email == nil && req.Phone == nil {
		return nil, appErr.BadRequest("IDENTIFIER_REQUIRED", "email or phone must be provided", nil)
	}
	if strings.TrimSpace(req.Password) == "" {
		return nil, appErr.BadRequest("PASSWORD_REQUIRED", "password is required", nil)
	}
	if strings.TrimSpace(req.Profile.Name) == "" {
		return nil, appErr.BadRequest("PROFILE_NAME_REQUIRED", "profile name is required", nil)
	}
	switch req.Role {
	case model.UserRolePharmacy, model.UserRoleWholesaler, model.UserRoleManufacturer:
	default:
		return nil, appErr.BadRequest("INVALID_ROLE", "registration role is invalid", map[string]string{
			"allowed": "PHARMACY | WHOLESALER | MANUFACTURER",
		})
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, appErr.Internal("failed to hash password")
	}

	user := &model.User{
		ID:           uuid.New(),
		Email:        normalizeOptional(req.Email),
		Phone:        normalizeOptional(req.Phone),
		PasswordHash: passwordHash,
		Role:         req.Role,
		Status:       model.UserStatusPending,
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
				return appErr.Conflict("USER_EXISTS", "email or phone already registered", nil)
			}
			return appErr.Internal("failed to create user")
		}

		switch req.Role {
		case model.UserRolePharmacy:
			profile := model.Pharmacy{
				UserID:    user.ID,
				Name:      req.Profile.Name,
				City:      req.Profile.City,
				Address:   req.Profile.Address,
				LicenseNo: req.Profile.LicenseNo,
			}
			if err := tx.Create(&profile).Error; err != nil {
				return appErr.Internal("failed to create pharmacy profile")
			}
		case model.UserRoleWholesaler:
			profile := model.Wholesaler{
				UserID:    user.ID,
				Name:      req.Profile.Name,
				Country:   req.Profile.Country,
				City:      req.Profile.City,
				Address:   req.Profile.Address,
				LicenseNo: req.Profile.LicenseNo,
			}
			if err := tx.Create(&profile).Error; err != nil {
				return appErr.Internal("failed to create wholesaler profile")
			}
		case model.UserRoleManufacturer:
			profile := model.Manufacturer{
				UserID:         user.ID,
				Name:           req.Profile.Name,
				Country:        req.Profile.Country,
				RegistrationNo: req.Profile.RegistrationNo,
			}
			if err := tx.Create(&profile).Error; err != nil {
				return appErr.Internal("failed to create manufacturer profile")
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return user, nil
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (auth.Pair, *model.User, error) {
	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" || strings.TrimSpace(req.Password) == "" {
		return auth.Pair{}, nil, appErr.BadRequest("INVALID_CREDENTIALS", "identifier and password are required", nil)
	}

	var user model.User
	err := s.db.WithContext(ctx).
		Where("email = ? OR phone = ?", identifier, identifier).
		First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return auth.Pair{}, nil, appErr.Unauthorized("INVALID_CREDENTIALS", "invalid credentials")
		}
		return auth.Pair{}, nil, appErr.Internal("failed to query user")
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		return auth.Pair{}, nil, appErr.Unauthorized("INVALID_CREDENTIALS", "invalid credentials")
	}

	if user.Status != model.UserStatusActive {
		return auth.Pair{}, nil, appErr.Forbidden("USER_NOT_ACTIVE", "user is not active")
	}

	tokenPair, err := s.authSvc.GeneratePair(user.ID, user.Role, user.Status)
	if err != nil {
		return auth.Pair{}, nil, appErr.Internal("failed to generate tokens")
	}

	return tokenPair, &user, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (auth.Pair, error) {
	claims, err := s.authSvc.ParseRefreshToken(refreshToken)
	if err != nil {
		return auth.Pair{}, appErr.Unauthorized("INVALID_REFRESH_TOKEN", "invalid or expired refresh token")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return auth.Pair{}, appErr.Unauthorized("INVALID_REFRESH_TOKEN", "invalid token subject")
	}

	var user model.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return auth.Pair{}, appErr.Unauthorized("INVALID_REFRESH_TOKEN", "user no longer exists")
		}
		return auth.Pair{}, appErr.Internal("failed to query user")
	}

	if user.Status != model.UserStatusActive {
		return auth.Pair{}, appErr.Forbidden("USER_NOT_ACTIVE", "user is not active")
	}

	pair, err := s.authSvc.GeneratePair(user.ID, user.Role, user.Status)
	if err != nil {
		return auth.Pair{}, appErr.Internal("failed to generate tokens")
	}
	return pair, nil
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, appErr.NotFound("USER_NOT_FOUND", "user not found")
		}
		return nil, appErr.Internal("failed to query user")
	}
	return &user, nil
}

func (s *Service) UpdateStatus(ctx context.Context, userID uuid.UUID, status model.UserStatus) (*model.User, error) {
	switch status {
	case model.UserStatusPending, model.UserStatusActive, model.UserStatusSuspended:
	default:
		return nil, appErr.BadRequest("INVALID_STATUS", "invalid user status", nil)
	}

	var user model.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, appErr.NotFound("USER_NOT_FOUND", "user not found")
		}
		return nil, appErr.Internal("failed to query user")
	}

	if err := s.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Update("status", status).Error; err != nil {
		return nil, appErr.Internal("failed to update user status")
	}

	user.Status = status
	return &user, nil
}

type ListUsersFilter struct {
	Role   *model.UserRole
	Status *model.UserStatus
	Limit  int
}

func (s *Service) ListUsers(ctx context.Context, filter ListUsersFilter) ([]model.User, error) {
	q := s.db.WithContext(ctx).Model(&model.User{}).Order("created_at DESC")

	if filter.Role != nil {
		q = q.Where("role = ?", *filter.Role)
	}
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	q = q.Limit(filter.Limit)

	var users []model.User
	if err := q.Find(&users).Error; err != nil {
		return nil, appErr.Internal("failed to list users")
	}
	return users, nil
}

func normalizeOptional(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

