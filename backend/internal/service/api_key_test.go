package service

import (
	"strings"
	"testing"
)

func TestGeneratePlainAPIKeyUsesMixedAlphaNumericAlphabet(t *testing.T) {
	seen := make(map[string]struct{}, 256)
	for i := 0; i < 256; i++ {
		key, err := generatePlainAPIKey()
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		if len(key) != 41 || !strings.HasPrefix(key, "sk-") {
			t.Fatalf("unexpected key format: %q", key)
		}
		body := strings.TrimPrefix(key, "sk-")
		if !strings.ContainsAny(body, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") ||
			!strings.ContainsAny(body, "abcdefghijklmnopqrstuvwxyz") ||
			!strings.ContainsAny(body, "0123456789") {
			t.Fatalf("key does not contain all required character classes: %q", key)
		}
		for _, char := range body {
			if !strings.ContainsRune(apiKeyAlphabet, char) {
				t.Fatalf("key contains invalid character %q", char)
			}
		}
		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate key generated: %q", key)
		}
		seen[key] = struct{}{}
	}
}
