package app

import (
	"context"
	"strings"
	"time"

	"pharmalink/server/internal/auth"
	"pharmalink/server/internal/db/model"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type seedUser struct {
	Identifier string
	Phone      string
	Password   string
	Role       model.UserRole
	Name       string
}

func EnsureDevSeedUsers(ctx context.Context, db *gorm.DB, log zerolog.Logger) error {
	seeds := []seedUser{
		{Identifier: "admin", Password: "admin", Role: model.UserRoleAdmin, Name: "Admin User"},
		{Identifier: "pharmacy", Phone: "+992900000001", Password: "pharmacy", Role: model.UserRolePharmacy, Name: "Test Pharmacy"},
		{Identifier: "wholesaler", Password: "wholesaler", Role: model.UserRoleWholesaler, Name: "Test Wholesaler"},
		{Identifier: "manufacturer", Password: "manufacturer", Role: model.UserRoleManufacturer, Name: "Test Manufacturer"},
	}

	for _, s := range seeds {
		if err := upsertSeedUser(ctx, db, s); err != nil {
			return err
		}
	}

	log.Info().Msg("development seed users ensured")
	return nil
}

func upsertSeedUser(ctx context.Context, db *gorm.DB, seed seedUser) error {
	hash, err := auth.HashPassword(seed.Password)
	if err != nil {
		return err
	}

	var user model.User
	err = db.WithContext(ctx).Where("email = ?", seed.Identifier).First(&user).Error
	switch err {
	case nil:
		if err := db.WithContext(ctx).Model(&model.User{}).
			Where("id = ?", user.ID).
			Updates(map[string]interface{}{
				"password_hash": hash,
				"phone":         strPtr(seed.Phone),
				"role":          seed.Role,
				"status":        model.UserStatusActive,
				"updated_at":    time.Now().UTC(),
			}).Error; err != nil {
			return err
		}
		return ensureProfile(ctx, db, user.ID, seed)
	case gorm.ErrRecordNotFound:
		id := uuid.New()
		user = model.User{
			ID:           id,
			Email:        strPtr(seed.Identifier),
			Phone:        strPtr(seed.Phone),
			PasswordHash: hash,
			Role:         seed.Role,
			Status:       model.UserStatusActive,
		}
		if err := db.WithContext(ctx).Create(&user).Error; err != nil {
			return err
		}
		return ensureProfile(ctx, db, id, seed)
	default:
		return err
	}
}

func ensureProfile(ctx context.Context, db *gorm.DB, userID uuid.UUID, seed seedUser) error {
	switch seed.Role {
	case model.UserRolePharmacy:
		var row model.Pharmacy
		if err := db.WithContext(ctx).First(&row, "user_id = ?", userID).Error; err == nil {
			return db.WithContext(ctx).Model(&model.Pharmacy{}).
				Where("user_id = ?", userID).
				Updates(map[string]interface{}{
					"name":       seed.Name,
					"city":       strPtr("Test City"),
					"address":    strPtr("Test Address"),
					"license_no": strPtr("PHARM-TEST-001"),
				}).Error
		}
		return db.WithContext(ctx).Create(&model.Pharmacy{
			UserID: userID, Name: seed.Name, City: strPtr("Test City"), Address: strPtr("Test Address"), LicenseNo: strPtr("PHARM-TEST-001"),
		}).Error
	case model.UserRoleWholesaler:
		var row model.Wholesaler
		if err := db.WithContext(ctx).First(&row, "user_id = ?", userID).Error; err == nil {
			return nil
		}
		return db.WithContext(ctx).Create(&model.Wholesaler{
			UserID: userID, Name: seed.Name, Country: strPtr("TJ"), City: strPtr("Test City"), Address: strPtr("Test Address"),
		}).Error
	case model.UserRoleManufacturer:
		var row model.Manufacturer
		if err := db.WithContext(ctx).First(&row, "user_id = ?", userID).Error; err == nil {
			return nil
		}
		return db.WithContext(ctx).Create(&model.Manufacturer{
			UserID: userID, Name: seed.Name, Country: strPtr("TJ"),
		}).Error
	default:
		return nil
	}
}

func strPtr(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}
