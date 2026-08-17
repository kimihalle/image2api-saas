package service

import (
	"strings"
	"testing"
)

func TestPgDumpDSNRemovesOnlyGORMTimeZone(t *testing.T) {
	tests := map[string]struct {
		input string
		want  []string
	}{
		"keyword": {
			input: "host=postgres user=northstar password=secret dbname=northstar port=5432 sslmode=disable TimeZone=Asia/Shanghai",
			want:  []string{"host=postgres", "user=northstar", "sslmode=disable"},
		},
		"quoted keyword": {
			input: "host='db internal' user=northstar timezone='Asia/Shanghai' sslmode=require",
			want:  []string{"host='db internal'", "user=northstar", "sslmode=require"},
		},
		"url": {
			input: "postgres://northstar:secret@postgres:5432/northstar?sslmode=disable&TimeZone=Asia%2FShanghai",
			want:  []string{"postgres://northstar:secret@postgres:5432/northstar", "sslmode=disable"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := pgDumpDSN(test.input)
			if strings.Contains(strings.ToLower(got), "timezone") {
				t.Fatalf("pgDumpDSN left TimeZone in %q", got)
			}
			for _, fragment := range test.want {
				if !strings.Contains(got, fragment) {
					t.Fatalf("pgDumpDSN(%q) = %q, missing %q", test.input, got, fragment)
				}
			}
		})
	}
}
