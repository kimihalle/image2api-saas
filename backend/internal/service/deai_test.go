package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestApplyDeAIProducesDecodableCroppedPNG(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 100, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 100; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 140, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}

	processed, err := applyDeAI(encoded.Bytes())
	if err != nil {
		t.Fatalf("applyDeAI failed: %v", err)
	}
	if !bytes.HasPrefix(processed, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("processed output is not PNG")
	}
	decoded, _, err := image.Decode(bytes.NewReader(processed))
	if err != nil {
		t.Fatalf("processed output cannot be decoded: %v", err)
	}
	if got, want := decoded.Bounds().Dx(), 96; got != want {
		t.Fatalf("width = %d, want %d", got, want)
	}
	if got, want := decoded.Bounds().Dy(), 76; got != want {
		t.Fatalf("height = %d, want %d", got, want)
	}
}

func TestApplyDeAIRejectsInvalidImage(t *testing.T) {
	if _, err := applyDeAI([]byte("not an image")); err == nil {
		t.Fatal("expected invalid image error")
	}
}
