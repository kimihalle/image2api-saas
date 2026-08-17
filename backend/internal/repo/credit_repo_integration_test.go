package repo

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"backend/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCreditRepositoryConcurrentReserve(t *testing.T) {
	dsn := os.Getenv("CREDIT_REPO_INTEGRATION_DSN")
	if dsn == "" {
		t.Skip("CREDIT_REPO_INTEGRATION_DSN is not configured")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	suffix := time.Now().UnixNano()
	userID := fmt.Sprintf("credit-test-%d", suffix)
	user := model.User{
		ID: userID, Email: fmt.Sprintf("credit-test-%d@example.invalid", suffix),
		Role: "user", Status: "active", Credits: 100,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() {
		db.Where("user_id = ?", userID).Delete(&model.CreditTransaction{})
		db.Delete(&model.User{}, "id = ?", userID)
	})

	const requests = 4
	repository := NewCreditRepository(db)
	ids := make([]string, requests)
	errs := make([]error, requests)
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			ids[index] = fmt.Sprintf("cr-test-%d-%d", suffix, index)
			_, _, errs[index] = repository.Reserve(
				context.Background(), ids[index], userID, 1, "integration:image",
			)
		}(i)
	}
	wg.Wait()
	for i, reserveErr := range errs {
		if reserveErr != nil {
			t.Fatalf("reserve %d failed: %v", i, reserveErr)
		}
	}

	var nullEvents int64
	if err := db.Model(&model.CreditTransaction{}).
		Where("user_id = ? AND event_id IS NULL", userID).
		Count(&nullEvents).Error; err != nil {
		t.Fatalf("count unbound reservations: %v", err)
	}
	if nullEvents != requests {
		t.Fatalf("unbound reservations = %d, want %d", nullEvents, requests)
	}

	// An event insert can fail before AttachEvent runs. The nullable EventID must
	// still scan cleanly so this early-failure path refunds the reservation.
	earlyRefundID := fmt.Sprintf("cr-test-%d-early-refund", suffix)
	if _, _, err := repository.Reserve(context.Background(), earlyRefundID, userID, 1, "integration:image"); err != nil {
		t.Fatalf("reserve early-refund transaction: %v", err)
	}
	refundedUser, refunded, err := repository.Refund(context.Background(), earlyRefundID)
	if err != nil {
		t.Fatalf("refund unbound reservation: %v", err)
	}
	if !refunded || refundedUser == nil || refundedUser.Credits != 96 {
		t.Fatalf("unbound refund = (%v, %#v), want refunded user with 96 credits", refunded, refundedUser)
	}

	for i, id := range ids {
		if err := repository.AttachEvent(context.Background(), id, fmt.Sprintf("evt-test-%d-%d", suffix, i)); err != nil {
			t.Fatalf("attach event %d: %v", i, err)
		}
	}

	var updated model.User
	if err := db.First(&updated, "id = ?", userID).Error; err != nil {
		t.Fatalf("reload test user: %v", err)
	}
	if updated.Credits != 96 {
		t.Fatalf("credits = %v, want 96", updated.Credits)
	}

	var attached int64
	if err := db.Model(&model.CreditTransaction{}).
		Where("user_id = ? AND event_id IS NOT NULL", userID).
		Count(&attached).Error; err != nil {
		t.Fatalf("count attached reservations: %v", err)
	}
	if attached != requests {
		t.Fatalf("attached reservations = %d, want %d", attached, requests)
	}
}
