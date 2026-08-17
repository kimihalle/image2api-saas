package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"backend/internal/model"
	"gorm.io/datatypes"
)

func TestGenerationQueueRejectsInvalidBatchSizeBeforeAccessingRepositories(t *testing.T) {
	svc := &GenerationQueueService{}
	user := &model.User{ID: "test-user"}
	for _, count := range []int{0, 5, -1} {
		if _, err := svc.SubmitImages(context.Background(), user, UserGenerateRequest{}, count); err == nil {
			t.Fatalf("count %d: expected validation error", count)
		}
	}
}

func TestShapeGenerationJob(t *testing.T) {
	now := time.Now()
	item := &model.GenerationJob{
		ID:            "job-test",
		Kind:          "image",
		Status:        "completed",
		Result:        datatypes.JSON(`{"url":"/images/test.png"}`),
		EstimatedCost: 2.5,
		CreatedAt:     now,
	}
	got := shapeGenerationJob(item, 0)
	if got["status"] != "completed" || got["progress"] != 100 {
		t.Fatalf("unexpected state: %#v", got)
	}
	result, ok := got["result"].(map[string]any)
	if !ok || result["url"] != "/images/test.png" {
		t.Fatalf("unexpected result: %#v", got["result"])
	}
}

func TestGenerationQueueHelpers(t *testing.T) {
	if got := rawReferenceBase64("data:image/png;base64,QUJD"); got != "QUJD" {
		t.Fatalf("rawReferenceBase64 = %q", got)
	}
	if got := friendlyQueueError(ErrInsufficientFunds); got != "可用额度不足，任务未扣费" {
		t.Fatalf("friendlyQueueError = %q", got)
	}
	if got := friendlyQueueError(errors.New("provider detail")); got != "provider detail" {
		t.Fatalf("friendlyQueueError passthrough = %q", got)
	}
}
