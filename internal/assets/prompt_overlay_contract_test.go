package assets_test

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

var promptRefRe = regexp.MustCompile(`\{file:\./prompts/([^}]+)\}`)

func TestPromptOverlayContract(t *testing.T) {
	overlays := []string{
		"opencode/sdd-overlay-single.json",
		"opencode/sdd-overlay-multi.json",
	}
	for _, overlay := range overlays {
		data, err := assets.FS.ReadFile(overlay)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", overlay, err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal %s error = %v", overlay, err)
		}
		matches := promptRefRe.FindAllStringSubmatch(string(data), -1)
		if len(matches) == 0 {
			t.Fatalf("%s has no prompt file refs", overlay)
		}
		for _, m := range matches {
			rel := m[1] // e.g. sdd/sdd-ff.md
			path := "prompts/" + rel
			if _, err := assets.FS.ReadFile(path); err != nil {
				t.Fatalf("%s references {file:./prompts/%s} but %s not in embedded FS (embed.go must include it)", overlay, rel, path)
			}
		}
	}

	// Also ensure every file under prompts/sdd is reachable at least from one overlay or is explicitly allowed as standalone (e.g. research)
	// For now just ensure no overlay references a missing file — the above covers the regression.
}
