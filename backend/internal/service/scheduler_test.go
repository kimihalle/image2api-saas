package service

import (
	"testing"
	"time"

	"backend/internal/model"
)

func TestSchedulerV2HonorsWeightThenHealth(t *testing.T) {
	svc := &V1Service{}
	items := []model.TokenAccount{
		{ID: "healthy", Weight: 1, HealthScore: 98},
		{ID: "degraded", Weight: 1, HealthScore: 35, ConsecutiveFails: 4},
		{ID: "operator-priority", Weight: 2, HealthScore: 10, ConsecutiveFails: 8},
	}
	svc.rotateRoundRobin("test", items)
	if items[0].ID != "operator-priority" || items[1].ID != "healthy" {
		t.Fatalf("unexpected scheduler order: %s, %s, %s", items[0].ID, items[1].ID, items[2].ID)
	}
}

func TestSchedulerV2ExcludesCoolingAccountsButAllowsPinnedTest(t *testing.T) {
	until := time.Now().Add(time.Minute)
	all := []model.TokenAccount{{ID: "cooling", Value: "token", CooldownUntil: &until}, {ID: "ready", Value: "token"}}
	active := append([]model.TokenAccount(nil), all...)
	filtered := pinTestAccount(all, active, "")
	if len(filtered) != 1 || filtered[0].ID != "ready" {
		t.Fatalf("unexpected active set: %#v", filtered)
	}
	pinned := pinTestAccount(all, active, "cooling")
	if len(pinned) != 1 || pinned[0].ID != "cooling" {
		t.Fatalf("admin pin did not bypass cooldown: %#v", pinned)
	}
}
