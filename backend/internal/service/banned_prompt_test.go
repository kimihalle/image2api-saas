package service

import "testing"

func TestNormalizeBannedTextClosesSimpleBypasses(t *testing.T) {
	cases := map[string]string{
		"习 近 平":   "习近平",
		"Ｐ．Ｏ．Ｒ．Ｎ": "porn",
		"n-s-f-w": "nsfw",
		"六・四事件":   "六四事件",
	}
	for input, want := range cases {
		if got := normalizeBannedText(input); got != want {
			t.Fatalf("normalizeBannedText(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeBannedTextDoesNotTreatYellowAsPornography(t *testing.T) {
	if got := normalizeBannedText("一辆黄色汽车"); got != "一辆黄色汽车" {
		t.Fatalf("unexpected normalization: %q", got)
	}
}
