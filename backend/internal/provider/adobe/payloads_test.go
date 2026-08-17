package adobe

import "testing"

func TestGPTImagePayloadUsesRequestedResolutionAndRatio(t *testing.T) {
	wants := map[string]map[string]string{
		"1K": {
			"1:1": "1024x1024", "5:4": "1280x1024", "9:16": "720x1280",
			"21:9": "1680x720", "16:9": "1280x720", "4:3": "1024x768",
			"3:2": "1200x800", "4:5": "1024x1280", "3:4": "768x1024", "2:3": "800x1200",
		},
		"2K": {
			"1:1": "2048x2048", "5:4": "2560x2048", "9:16": "1152x2048",
			"21:9": "2520x1080", "16:9": "2048x1152", "4:3": "2048x1536",
			"3:2": "2400x1600", "4:5": "2048x2560", "3:4": "1536x2048", "2:3": "1600x2400",
		},
		"4K": {
			"1:1": "4096x4096", "5:4": "3840x3072", "9:16": "2304x4096",
			"21:9": "5040x2160", "16:9": "4096x2304", "4:3": "4096x3072",
			"3:2": "3600x2400", "4:5": "3072x3840", "3:4": "3072x4096", "2:3": "2400x3600",
		},
	}

	for resolution, ratios := range wants {
		for ratio, want := range ratios {
			t.Run(resolution+"/"+ratio, func(t *testing.T) {
				assertGPTImageSize(t, resolution, ratio, want)
			})
		}
	}
	assertGPTImageSize(t, "", "1:1", "2048x2048")
}

func assertGPTImageSize(t *testing.T, resolution, ratio, want string) {
	t.Helper()
	payloads := BuildImagePayloadCandidates("firefly-gpt-image-2", "test", ratio, resolution, nil)
	if len(payloads) != 1 {
		t.Fatalf("payload count = %d, want 1", len(payloads))
	}
	specific, ok := payloads[0]["modelSpecificPayload"].(map[string]any)
	if !ok {
		t.Fatalf("modelSpecificPayload = %#v", payloads[0]["modelSpecificPayload"])
	}
	if got := specific["size"]; got != want {
		t.Fatalf("size = %#v, want %q", got, want)
	}
}

func TestGPTImagePayloadKeepsSubjectReferences(t *testing.T) {
	payload := BuildImagePayloadCandidates("firefly-gpt-image-2", "test", "4:5", "4K", []string{"blob-1"})[0]
	refs, ok := payload["referenceBlobs"].([]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("referenceBlobs = %#v", payload["referenceBlobs"])
	}
	ref, ok := refs[0].(map[string]any)
	if !ok || ref["id"] != "blob-1" || ref["usage"] != "subject" {
		t.Fatalf("reference = %#v", refs[0])
	}
}
