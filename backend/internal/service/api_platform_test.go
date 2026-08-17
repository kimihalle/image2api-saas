package service

import (
	"strings"
	"testing"
)

func TestWebhookSignatureIsDeterministicAndPayloadBound(t *testing.T) {
	first := webhookSignature("whsec_test", "1786800000", []byte(`{"id":"evt_1"}`))
	second := webhookSignature("whsec_test", "1786800000", []byte(`{"id":"evt_1"}`))
	changed := webhookSignature("whsec_test", "1786800000", []byte(`{"id":"evt_2"}`))
	if first != second {
		t.Fatalf("same input produced different signatures: %q != %q", first, second)
	}
	if first == changed || !strings.HasPrefix(first, "v1=") || len(first) != 67 {
		t.Fatalf("unexpected signature behavior: first=%q changed=%q", first, changed)
	}
}

func TestValidateWebhookURLRejectsPrivateDestinations(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1/hook", "http://localhost/hook", "ftp://example.com/hook"} {
		if err := validateWebhookURL(raw); err == nil {
			t.Fatalf("validateWebhookURL(%q) unexpectedly succeeded", raw)
		}
	}
}
