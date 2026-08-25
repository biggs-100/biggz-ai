package lens

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubLens is a minimal Lens for registry ordering tests.
type stubLens struct {
	id string
}

func (s *stubLens) ID() string { return s.id }
func (s *stubLens) Analyze(_ context.Context, _ LensInput) (LensResult, error) {
	return LensResult{LensID: s.id, Findings: nil, Evidence: []string{"ok"}}, nil
}

func TestRegistry_OrderedPreservesOrder(t *testing.T) {
	ResetRegistry()
	ids := []string{"risk", "resilience", "readability", "reliability"}
	for _, id := range ids {
		RegisterLens(&stubLens{id: id})
	}
	ordered := Ordered([]string{"risk", "resilience", "readability", "reliability"})
	if len(ordered) != 4 {
		t.Fatalf("ordered len = %d, want 4", len(ordered))
	}
	for i, want := range ids {
		if got := ordered[i].ID(); got != want {
			t.Errorf("ordered[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestRegistry_SkipUnknown(t *testing.T) {
	ResetRegistry()
	RegisterLens(&stubLens{id: "risk"})
	RegisterLens(&stubLens{id: "reliability"})
	ordered := Ordered([]string{"risk", "unknown-lens", "reliability", "nope"})
	if len(ordered) != 2 {
		t.Fatalf("ordered len = %d, want 2 (unknown skipped)", len(ordered))
	}
	if ordered[0].ID() != "risk" || ordered[1].ID() != "reliability" {
		t.Errorf("ordered = [%q, %q], want [risk, reliability]", ordered[0].ID(), ordered[1].ID())
	}
}

func TestRegistry_LastWin(t *testing.T) {
	ResetRegistry()
	first := &stubLens{id: "readability"}
	second := &stubLens{id: "readability"}
	RegisterLens(first)
	RegisterLens(second)
	ordered := Ordered([]string{"readability"})
	if len(ordered) != 1 {
		t.Fatalf("ordered len = %d, want 1", len(ordered))
	}
	if ordered[0] != second {
		t.Error("last-win: expected second registration to win")
	}
	// Registry copy should also reflect last-win.
	reg := Registry()
	if reg["readability"] != second {
		t.Error("Registry() should reflect last-win")
	}
}

func TestRegistry_RegistryCopyIsolation(t *testing.T) {
	ResetRegistry()
	RegisterLens(&stubLens{id: "risk"})
	copy := Registry()
	copy["risk"] = &stubLens{id: "risk-mutated"}
	original := Registry()
	if original["risk"].ID() == "risk-mutated" {
		t.Error("Registry() copy mutation leaked into registry")
	}
}

func TestRegistry_NoPluginLens(t *testing.T) {
	// Guard: Lens must not be in plugin/.
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin", "interfaces.go"))
	if err != nil {
		// plugin/interfaces.go moved or absent is acceptable only if no LensPlugin exists anywhere in plugin/
		// Fall back to directory walk.
		fallback, globErr := filepath.Glob(filepath.Join("..", "..", "..", "plugin", "*.go"))
		if globErr != nil {
			t.Fatalf("glob plugin/*.go: %v", globErr)
		}
		for _, p := range fallback {
			b, readErr := os.ReadFile(p)
			if readErr != nil {
				continue
			}
			if strings.Contains(string(b), "LensPlugin") || strings.Contains(string(b), "type Lens ") {
				t.Errorf("plugin file %s contains LensPlugin/Lens — must stay in internal/review/lens", p)
			}
		}
		return
	}
	content := string(data)
	if strings.Contains(content, "LensPlugin") {
		t.Error("plugin/interfaces.go contains LensPlugin — forbidden; Lens lives in internal/review/lens/types.go")
	}
	if strings.Contains(content, "type Lens ") || strings.Contains(content, "type Lens\t") {
		t.Error("plugin/interfaces.go contains type Lens — forbidden")
	}
}

func TestRegistry_InternalLensAbsent(t *testing.T) {
	// Guard: internal/lens/ must not exist; lenses live under internal/review/lens/
	if _, err := os.Stat(filepath.Join("..", "..", "..", "internal", "lens")); !os.IsNotExist(err) {
		t.Error("internal/lens/ must not exist; lenses live under internal/review/lens/")
	}
}

func TestRegistry_EmptyOrdered(t *testing.T) {
	ResetRegistry()
	RegisterLens(&stubLens{id: "risk"})
	if got := Ordered(nil); len(got) != 0 {
		t.Errorf("Ordered(nil) = %d, want 0", len(got))
	}
	if got := Ordered([]string{}); len(got) != 0 {
		t.Errorf("Ordered([]) = %d, want 0", len(got))
	}
}
