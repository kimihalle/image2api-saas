package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"backend/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openFinancialIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CREDIT_REPO_INTEGRATION_DSN")
	if dsn == "" {
		t.Skip("CREDIT_REPO_INTEGRATION_DSN is not configured")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	return db
}

func TestOrderMarkPaidAndCreditIsAtomicAndIdempotent(t *testing.T) {
	db := openFinancialIntegrationDB(t)
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	userID := fmt.Sprintf("pay-test-%d", suffix)
	orderID := fmt.Sprintf("PTEST%d", suffix)
	lateOrderID := fmt.Sprintf("PLATE%d", suffix)
	orphanOrderID := fmt.Sprintf("PORPHAN%d", suffix)
	user := model.User{ID: userID, Email: fmt.Sprintf("pay-test-%d@example.invalid", suffix), Role: "user", Status: "active", Credits: 10}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := time.Now()
	for _, order := range []model.Order{
		{ID: orderID, UserID: userID, Amount: 12.34, Points: 1234, Status: "pending", Source: "epay", ExpiresAt: now.Add(time.Hour)},
		{ID: lateOrderID, UserID: userID, Amount: 1, Points: 100, Status: "cancelled", Source: "epay", ExpiresAt: now.Add(-time.Hour)},
		{ID: orphanOrderID, UserID: "missing-user", Amount: 2, Points: 200, Status: "pending", Source: "epay", ExpiresAt: now.Add(time.Hour)},
	} {
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("create order %s: %v", order.ID, err)
		}
	}
	t.Cleanup(func() {
		db.Delete(&model.Order{}, "id IN ?", []string{orderID, lateOrderID, orphanOrderID})
		db.Delete(&model.User{}, "id = ?", userID)
	})

	repository := NewOrderRepository(db)
	credited, err := repository.MarkPaidAndCredit(ctx, orderID, "trade-first", now)
	if err != nil || !credited {
		t.Fatalf("first callback = (%v, %v), want credited", credited, err)
	}
	credited, err = repository.MarkPaidAndCredit(ctx, orderID, "trade-duplicate", now)
	if err != nil || credited {
		t.Fatalf("duplicate callback = (%v, %v), want no-op", credited, err)
	}
	credited, err = repository.MarkPaidAndCredit(ctx, lateOrderID, "trade-late", now)
	if err != nil || !credited {
		t.Fatalf("late callback = (%v, %v), want credited", credited, err)
	}

	var updated model.User
	if err := db.First(&updated, "id = ?", userID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.Credits != 1344 || updated.RechargeTotal != 13.34 {
		t.Fatalf("user totals = credits %.2f recharge %.2f, want 1344 and 13.34", updated.Credits, updated.RechargeTotal)
	}

	credited, err = repository.MarkPaidAndCredit(ctx, orphanOrderID, "trade-orphan", now)
	if !errors.Is(err, gorm.ErrRecordNotFound) || credited {
		t.Fatalf("orphan callback = (%v, %v), want rollback", credited, err)
	}
	var orphan model.Order
	if err := db.First(&orphan, "id = ?", orphanOrderID).Error; err != nil || orphan.Status != "pending" {
		t.Fatalf("orphan order status = %q, err %v; want pending", orphan.Status, err)
	}
}

func TestCDKRedeemAndCreditRollsBackAsAUnit(t *testing.T) {
	db := openFinancialIntegrationDB(t)
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	userID := fmt.Sprintf("cdk-test-%d", suffix)
	codeOK := fmt.Sprintf("CDK-OK-%d", suffix)
	codeRollback := fmt.Sprintf("CDK-RB-%d", suffix)
	orderID := fmt.Sprintf("CTEST%d", suffix)
	conflictOrderID := fmt.Sprintf("CCONFLICT%d", suffix)
	user := model.User{ID: userID, Email: fmt.Sprintf("cdk-test-%d@example.invalid", suffix), Role: "user", Status: "active", Credits: 5}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	for _, code := range []model.CDKCode{
		{Code: codeOK, Amount: 20, Status: "active", Type: "normal"},
		{Code: codeRollback, Amount: 30, Status: "active", Type: "normal"},
	} {
		if err := db.Create(&code).Error; err != nil {
			t.Fatalf("create cdk: %v", err)
		}
	}
	now := time.Now()
	if err := db.Create(&model.Order{ID: conflictOrderID, UserID: userID, Status: "paid", Source: "cdk", ExpiresAt: now}).Error; err != nil {
		t.Fatalf("create conflicting order: %v", err)
	}
	t.Cleanup(func() {
		db.Delete(&model.Order{}, "id IN ?", []string{orderID, conflictOrderID})
		db.Delete(&model.CDKCode{}, "code IN ?", []string{codeOK, codeRollback})
		db.Delete(&model.User{}, "id = ?", userID)
	})

	repository := NewCDKRepository(db)
	item, balance, err := repository.RedeemAndCredit(ctx, codeOK, userID, orderID, "test redemption")
	if err != nil || item == nil || balance != 25 {
		t.Fatalf("redeem = (%#v, %.2f, %v), want balance 25", item, balance, err)
	}
	if _, _, err := repository.RedeemAndCredit(ctx, codeRollback, userID, conflictOrderID, "must rollback"); !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("conflicting billing insert error = %v, want duplicate key", err)
	}

	var rolledBack model.CDKCode
	if err := db.First(&rolledBack, "code = ?", codeRollback).Error; err != nil || rolledBack.Status != "active" {
		t.Fatalf("rollback cdk status = %q, err %v; want active", rolledBack.Status, err)
	}
	var updated model.User
	if err := db.First(&updated, "id = ?", userID).Error; err != nil || updated.Credits != 25 {
		t.Fatalf("balance after rollback = %.2f, err %v; want 25", updated.Credits, err)
	}
}

func TestAdminCreditAdjustmentAndBillingEntryAreAtomic(t *testing.T) {
	db := openFinancialIntegrationDB(t)
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	userID := fmt.Sprintf("adjust-test-%d", suffix)
	conflictID := fmt.Sprintf("MADJCONFLICT%d", suffix)
	successID := fmt.Sprintf("MADJOK%d", suffix)
	user := model.User{ID: userID, Email: fmt.Sprintf("adjust-test-%d@example.invalid", suffix), Role: "user", Status: "active", Credits: 50}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := time.Now()
	if err := db.Create(&model.Order{ID: conflictID, UserID: userID, Status: "paid", Source: "admin", ExpiresAt: now}).Error; err != nil {
		t.Fatalf("create conflicting order: %v", err)
	}
	t.Cleanup(func() {
		db.Delete(&model.Order{}, "id IN ?", []string{conflictID, successID})
		db.Delete(&model.User{}, "id = ?", userID)
	})

	repository := NewOrderRepository(db)
	if _, err := repository.AdjustCreditsAndRecord(ctx, userID, 10, nil, &model.Order{ID: conflictID, Remark: "rollback"}); !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("conflicting adjustment error = %v, want duplicate key", err)
	}
	var unchanged model.User
	if err := db.First(&unchanged, "id = ?", userID).Error; err != nil || unchanged.Credits != 50 {
		t.Fatalf("balance after rollback = %.2f, err %v; want 50", unchanged.Credits, err)
	}

	setTo := 25.0
	adjusted, err := repository.AdjustCreditsAndRecord(ctx, userID, 0, &setTo, &model.Order{ID: successID, Remark: "set"})
	if err != nil || adjusted.Credits != 25 {
		t.Fatalf("set adjustment = %.2f, err %v; want 25", adjusted.Credits, err)
	}
	var order model.Order
	if err := db.First(&order, "id = ?", successID).Error; err != nil || order.Points != -25 {
		t.Fatalf("billing points = %d, err %v; want -25", order.Points, err)
	}
}
