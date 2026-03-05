package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"pharmalink/server/internal/config"
	appErr "pharmalink/server/internal/common/errors"
	"pharmalink/server/internal/db/model"
	"pharmalink/server/internal/modules/outbox"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db      *gorm.DB
	cfg     config.PaymentConfig
	outbox  *outbox.Service
}

func NewService(db *gorm.DB, cfg config.PaymentConfig, outboxSvc *outbox.Service) *Service {
	return &Service{
		db:     db,
		cfg:    cfg,
		outbox: outboxSvc,
	}
}

func (s *Service) CreateInvoice(ctx context.Context, userID uuid.UUID) (*model.Payment, error) {
	amount, err := decimal.NewFromString(s.cfg.AccessFee)
	if err != nil {
		return nil, appErr.Internal("invalid payment access fee configuration")
	}

	row := &model.Payment{
		ID:        uuid.New(),
		UserID:    userID,
		Amount:    amount,
		Currency:  s.cfg.Currency,
		InvoiceID: "inv_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		Status:    model.PaymentStatusPending,
	}

	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, appErr.Internal("failed to create payment invoice")
	}
	return row, nil
}

type WebhookInput struct {
	InvoiceID     string `json:"invoice_id"`
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

func (s *Service) VerifyWebhook(ctx context.Context, input WebhookInput, signature string) (*model.Payment, *model.AccessPass, error) {
	if !s.verifySignature(input, signature) {
		return nil, nil, appErr.Unauthorized("INVALID_WEBHOOK_SIGNATURE", "webhook signature validation failed")
	}

	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		return nil, nil, appErr.BadRequest("INVALID_WEBHOOK", "status is required", nil)
	}

	var payment model.Payment
	var access model.AccessPass

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&payment, "invoice_id = ?", input.InvoiceID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return appErr.NotFound("INVOICE_NOT_FOUND", "invoice not found")
			}
			return appErr.Internal("failed to query invoice")
		}

		if payment.Status == model.PaymentStatusPaid {
			if err := tx.First(&access, "user_id = ?", payment.UserID).Error; err != nil && err != gorm.ErrRecordNotFound {
				return appErr.Internal("failed to query access pass")
			}
			return nil
		}

		if status != "PAID" {
			if err := tx.Model(&model.Payment{}).
				Where("id = ?", payment.ID).
				Updates(map[string]interface{}{
					"status":         model.PaymentStatusFailed,
					"transaction_id": nullableString(input.TransactionID),
				}).Error; err != nil {
				return appErr.Internal("failed to update failed payment")
			}
			payment.Status = model.PaymentStatusFailed
			return nil
		}

		now := time.Now().UTC()
		if err := tx.Model(&model.Payment{}).
			Where("id = ?", payment.ID).
			Updates(map[string]interface{}{
				"status":         model.PaymentStatusPaid,
				"transaction_id": nullableString(input.TransactionID),
				"paid_at":        &now,
			}).Error; err != nil {
			return appErr.Internal("failed to update payment status")
		}
		payment.Status = model.PaymentStatusPaid
		payment.PaidAt = &now

		grantUntil := now.Add(time.Duration(s.cfg.AccessGrantHours) * time.Hour)
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&access, "user_id = ?", payment.UserID).Error
		switch err {
		case nil:
			base := access.AccessUntil
			if base.Before(now) {
				base = now
			}
			access.AccessUntil = base.Add(time.Duration(s.cfg.AccessGrantHours) * time.Hour)
			if err := tx.Model(&model.AccessPass{}).
				Where("user_id = ?", payment.UserID).
				Update("access_until", access.AccessUntil).Error; err != nil {
				return appErr.Internal("failed to extend access pass")
			}
		case gorm.ErrRecordNotFound:
			access = model.AccessPass{
				UserID:      payment.UserID,
				AccessUntil: grantUntil,
			}
			if err := tx.Create(&access).Error; err != nil {
				return appErr.Internal("failed to create access pass")
			}
		default:
			return appErr.Internal("failed to query access pass")
		}

		outPay, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "payment.verified",
			Payload: map[string]interface{}{
				"payment_id":      payment.ID,
				"user_id":         payment.UserID,
				"invoice_id":      payment.InvoiceID,
				"transaction_id":  nullableString(input.TransactionID),
				"status":          payment.Status,
				"paid_at":         payment.PaidAt,
			},
		})
		if err != nil {
			return err
		}
		if err := s.outbox.Notify(ctx, tx, outPay.ID); err != nil {
			return fmt.Errorf("notify payment event: %w", err)
		}

		outAccess, err := s.outbox.Write(ctx, tx, outbox.Event{
			Type: "access.updated",
			Payload: map[string]interface{}{
				"user_id":      access.UserID,
				"access_until": access.AccessUntil,
				"source":       "payment",
				"invoice_id":   payment.InvoiceID,
			},
		})
		if err != nil {
			return err
		}
		if err := s.outbox.Notify(ctx, tx, outAccess.ID); err != nil {
			return fmt.Errorf("notify access event: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return &payment, &access, nil
}

func (s *Service) GetAccess(ctx context.Context, userID uuid.UUID) (*model.AccessPass, time.Duration, error) {
	var access model.AccessPass
	if err := s.db.WithContext(ctx).First(&access, "user_id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, 0, nil
		}
		return nil, 0, appErr.Internal("failed to query access pass")
	}

	remaining := time.Until(access.AccessUntil)
	if remaining < 0 {
		remaining = 0
	}
	return &access, remaining, nil
}

func (s *Service) verifySignature(input WebhookInput, signature string) bool {
	payload := fmt.Sprintf("%s:%s:%s", input.InvoiceID, input.TransactionID, strings.ToUpper(strings.TrimSpace(input.Status)))
	mac := hmac.New(sha256.New, []byte(s.cfg.WebhookSecret))
	_, _ = mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(strings.TrimSpace(signature))))
}

func nullableString(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
