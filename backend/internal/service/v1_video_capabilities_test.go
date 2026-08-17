package service

import (
	"strings"
	"testing"
)

func TestContainsDurationRequiresAdvertisedValue(t *testing.T) {
	supported := []string{"5s", "10s"}
	if !containsDuration(supported, "5") {
		t.Fatal("numeric duration should match its normalized seconds value")
	}
	if containsDuration(supported, "7s") {
		t.Fatal("duration inside the min/max range must not pass unless advertised")
	}
}

func TestEnsureCapabilityMediaSizes(t *testing.T) {
	oneMB := strings.Repeat("A", 1024*1024*4/3)
	if err := ensureCapabilityMediaSizes([]string{"data:image/png;base64," + oneMB}, 2, "image"); err != nil {
		t.Fatalf("1MB input should fit a 2MB model limit: %v", err)
	}
	threeMB := strings.Repeat("A", 3*1024*1024*4/3)
	if err := ensureCapabilityMediaSizes([]string{threeMB}, 2, "image"); err == nil {
		t.Fatal("3MB input should exceed a 2MB model limit")
	}
}

func TestCapabilityIntsFromJSONValues(t *testing.T) {
	got := capabilityInts(map[string]any{"concurrency_options": []any{float64(1), float64(2), float64(4)}}, "concurrency_options")
	for _, wanted := range []int{1, 2, 4} {
		if !containsInt(got, wanted) {
			t.Fatalf("missing concurrency option %d in %v", wanted, got)
		}
	}
}
