package repo

import (
	"context"
	"errors"
	"time"

	"backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrCDKBatchLimit is returned when a user tries to redeem a second code from
// the same marketing batch (one per user per batch).
var ErrCDKBatchLimit = errors.New("cdk marketing batch already redeemed by this user")

type CDKRepository struct {
	db *gorm.DB
}

func NewCDKRepository(db *gorm.DB) *CDKRepository {
	return &CDKRepository{db: db}
}

func (r *CDKRepository) List(ctx context.Context) ([]model.CDKCode, error) {
	var items []model.CDKCode
	if err := r.db.WithContext(ctx).Order("created_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *CDKRepository) Stats(ctx context.Context) (map[string]any, error) {
	var total, active, redeemed int64
	if err := r.db.WithContext(ctx).Model(&model.CDKCode{}).Count(&total).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.CDKCode{}).Where("status = ?", "active").Count(&active).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.CDKCode{}).Where("status = ?", "redeemed").Count(&redeemed).Error; err != nil {
		return nil, err
	}

	type sumRow struct {
		Total *float64 `gorm:"column:total"`
	}
	var activeAmount, redeemedAmount sumRow
	if err := r.db.WithContext(ctx).
		Model(&model.CDKCode{}).
		Select("SUM(amount) AS total").
		Where("status = ?", "active").
		Scan(&activeAmount).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CDKCode{}).
		Select("SUM(amount) AS total").
		Where("status = ?", "redeemed").
		Scan(&redeemedAmount).Error; err != nil {
		return nil, err
	}

	activeAmt := 0.0
	if activeAmount.Total != nil {
		activeAmt = *activeAmount.Total
	}
	redeemedAmt := 0.0
	if redeemedAmount.Total != nil {
		redeemedAmt = *redeemedAmount.Total
	}

	return map[string]any{
		"total":           total,
		"active":          active,
		"redeemed":        redeemed,
		"active_amount":   activeAmt,
		"redeemed_amount": redeemedAmt,
	}, nil
}

func (r *CDKRepository) CreateBatch(ctx context.Context, items []model.CDKCode) error {
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *CDKRepository) Delete(ctx context.Context, code string) (int64, error) {
	res := r.db.WithContext(ctx).Delete(&model.CDKCode{}, "code = ?", code)
	return res.RowsAffected, res.Error
}

func (r *CDKRepository) DeleteByCodes(ctx context.Context, codes []string) (int64, error) {
	if len(codes) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Delete(&model.CDKCode{}, "code IN ?", codes)
	return res.RowsAffected, res.Error
}

func (r *CDKRepository) Redeem(ctx context.Context, code, userID string) (*model.CDKCode, error) {
	var out *model.CDKCode
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.CDKCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "code = ?", code).Error; err != nil {
			return err
		}
		if item.Status == "redeemed" {
			return gorm.ErrDuplicatedKey
		}
		// Marketing codes: a user may redeem only ONE code per batch. The partial
		// unique index (batch_id, redeemed_by) is the hard backstop against
		// concurrent double-redeems; this check gives a friendly error first.
		if item.Type == "marketing" && item.BatchID != "" {
			var cnt int64
			if err := tx.Model(&model.CDKCode{}).
				Where("batch_id = ? AND type = 'marketing' AND redeemed_by = ?", item.BatchID, userID).
				Count(&cnt).Error; err != nil {
				return err
			}
			if cnt > 0 {
				return ErrCDKBatchLimit
			}
		}
		now := time.Now()
		item.Status = "redeemed"
		item.RedeemedBy = &userID
		item.RedeemedAt = &now
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		out = &item
		return nil
	})
	return out, err
}

// RedeemAndCredit atomically consumes a code, credits the user and records the
// paid billing entry. If any write fails, the code remains redeemable.
func (r *CDKRepository) RedeemAndCredit(ctx context.Context, code, userID, orderID, remark string) (*model.CDKCode, float64, error) {
	var out *model.CDKCode
	var balance float64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.CDKCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "code = ?", code).Error; err != nil {
			return err
		}
		if item.Status != "active" {
			return gorm.ErrDuplicatedKey
		}
		if item.Type == "marketing" && item.BatchID != "" {
			var count int64
			if err := tx.Model(&model.CDKCode{}).
				Where("batch_id = ? AND type = 'marketing' AND redeemed_by = ?", item.BatchID, userID).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrCDKBatchLimit
			}
		}

		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			return err
		}
		balance = user.Credits + float64(item.Amount)
		now := time.Now()
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"credits":    balance,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}

		item.Status = "redeemed"
		item.RedeemedBy = &userID
		item.RedeemedAt = &now
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Order{
			ID:        orderID,
			UserID:    userID,
			Points:    item.Amount,
			PayType:   "cdk",
			Status:    "paid",
			Source:    "cdk",
			Remark:    remark,
			ExpiresAt: now,
			PaidAt:    &now,
		}).Error; err != nil {
			return err
		}
		out = &item
		return nil
	})
	return out, balance, err
}
