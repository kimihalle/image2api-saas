package repo

import "testing"

func TestNormalizeBannedWordPolicy(t *testing.T) {
	tests := []struct {
		name         string
		category     string
		autoBan      bool
		wantCategory string
		wantAutoBan  bool
		wantErr      bool
	}{
		{name: "default", wantCategory: "custom"},
		{name: "trim and normalize", category: " Sexual ", wantCategory: "sexual"},
		{name: "custom auto ban", category: "custom", autoBan: true, wantCategory: "custom", wantAutoBan: true},
		{name: "politics is always auto ban", category: "politics", wantCategory: "politics", wantAutoBan: true},
		{name: "unsupported", category: "spam", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category, autoBan, err := NormalizeBannedWordPolicy(tt.category, tt.autoBan)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if category != tt.wantCategory || autoBan != tt.wantAutoBan {
				t.Fatalf("policy = (%q, %v), want (%q, %v)", category, autoBan, tt.wantCategory, tt.wantAutoBan)
			}
		})
	}
}
