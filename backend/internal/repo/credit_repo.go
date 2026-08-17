package repo

import (
	"context"
	"errors"
	"time"

	"backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInsufficientCreditBalance = errors.New("insufficient credit balance")

type CreditRepository struct {
	db *gorm.DB
}

func NewCreditRepository(db *gorm.DB) *CreditRepository {
	return &CreditRepository{db: db}
}

func (r *CreditRepository) Reserve(ctx context.Context, id, userID string, amount float64, reason string) (*model.CreditTransaction, *model.User, error) {
	var txItem model.CreditTransaction
	var user model.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			return err
		}
		if user.Credits < amount {
			return ErrInsufficientCreditBalance
		}
		user.Credits -= amount
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
			"credits": user.Credits, "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		txItem = model.CreditTransaction{
			ID: id, UserID: userID, Amount: amount, BalanceAfter: user.Credits,
			Status: "reserved", Reason: reason,
		}
		// EventID is attached only after the pending generation event exists.
		// Omit it here so PostgreSQL stores NULL instead of Go's empty string:
		// the unique index permits concurrent unbound reservations as NULLs.
		return tx.Omit("EventID").Create(&txItem).Error
	})
	if err != nil {
		return nil, &user, err
	}
	return &txItem, &user, nil
}

func (r *CreditRepository) AttachEvent(ctx context.Context, id, eventID string) error {
	if id == "" || eventID == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.CreditTransaction{}).
		Where("id = ? AND (event_id IS NULL OR event_id = '')", id).
		Update("event_id", eventID).Error
}

func (r *CreditRepository) Capture(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&model.CreditTransaction{}).
		Where("id = ? AND status = ?", id, "reserved").
		Updates(map[string]any{"status": "captured", "captured_at": &now, "updated_at": now})
	return res.RowsAffected == 1, res.Error
}

func (r *CreditRepository) Refund(ctx context.Context, id string) (*model.User, bool, error) {
	if id == "" {
		return nil, false, nil
	}
	var updated model.User
	refunded := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.CreditTransaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if item.Status != "reserved" {
			return nil
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&updated, "id = ?", item.UserID).Error; err != nil {
			return err
		}
		updated.Credits += item.Amount
		now := time.Now()
		if err := tx.Model(&model.User{}).Where("id = ?", updated.ID).Updates(map[string]any{
			"credits": updated.Credits, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CreditTransaction{}).Where("id = ? AND status = ?", id, "reserved").Updates(map[string]any{
			"status": "refunded", "refunded_at": &now, "balance_after": updated.Credits, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		refunded = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if !refunded {
		return nil, false, nil
	}
	return &updated, true, nil
}

// RefundCapturedMismatch is reserved for the reconciliation service. It repairs
// the impossible state "generation failed but its reservation was captured".
// The row lock and captured-only guard keep repeated reconciliation runs
// idempotent; normal request paths must continue using Refund.
func (r *CreditRepository) RefundCapturedMismatch(ctx context.Context, id string) (*model.User, bool, error) {
	if id == "" {
		return nil, false, nil
	}
	var updated model.User
	refunded := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.CreditTransaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", id).Error; err != nil {
			return err
		}
		if item.Status != "captured" {
			return nil
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&updated, "id = ?", item.UserID).Error; err != nil {
			return err
		}
		now := time.Now()
		updated.Credits += item.Amount
		if err := tx.Model(&model.User{}).Where("id = ?", updated.ID).Updates(map[string]any{
			"credits": updated.Credits, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CreditTransaction{}).Where("id = ? AND status = ?", id, "captured").Updates(map[string]any{
			"status": "refunded", "refunded_at": &now, "balance_after": updated.Credits,
			"reason": gorm.Expr("reason || ?", " [automatic reconciliation]"), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		refunded = true
		return nil
	})
	return &updated, refunded, err
}

func (r *CreditRepository) GetByEvent(ctx context.Context, eventID string) (*model.CreditTransaction, error) {
	var item model.CreditTransaction
	if err := r.db.WithContext(ctx).First(&item, "event_id = ?", eventID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CreditRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]model.CreditTransaction, int64, error) {
	var items []model.CreditTransaction
	var total int64
	q := r.db.WithContext(ctx).Model(&model.CreditTransaction{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *CreditRepository) List(ctx context.Context, status, userID string, limit, offset int) ([]model.CreditTransaction, int64, error) {
	var items []model.CreditTransaction
	var total int64
	q := r.db.WithContext(ctx).Model(&model.CreditTransaction{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
