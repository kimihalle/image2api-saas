package service

import "testing"

func TestSizeFromRatioResUsesResolutionAsShortEdge(t *testing.T) {
	tests := map[string]struct {
		ratio, resolution, want string
	}{
		"landscape 720p":   {"16:9", "720p", "1280x720"},
		"portrait 720p":    {"9:16", "720p", "720x1280"},
		"landscape 1080p":  {"4:3", "1080p", "1440x1080"},
		"portrait 1080p":   {"3:4", "1080p", "1080x1440"},
		"square":           {"1:1", "720p", "720x720"},
		"invalid fallback": {"bad", "720p", "1280x720"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := sizeFromRatioRes(test.ratio, test.resolution); got != test.want {
				t.Fatalf("sizeFromRatioRes(%q, %q) = %q, want %q", test.ratio, test.resolution, got, test.want)
			}
		})
	}
}
