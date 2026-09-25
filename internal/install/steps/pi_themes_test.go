package steps

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// statusLineKeyPattern matches the statusLine* theme color keys that deployed
// pi assets read through ctx.ui.theme.fg(...). The string literal in the asset
// is the only contract between the painter and the theme file.
var statusLineKeyPattern = regexp.MustCompile(`"(statusLine[A-Za-z]+)"`)

// portedThemeNames lists the Gentle Shell themes this port ships. Declared
// explicitly so a silently dropped asset fails here instead of at runtime.
var portedThemeNames = []string{"Gentle", "Gentleman-Cute", "Gentleman-Sexy"}

type piThemeFile struct {
	Name   string         `json:"name"`
	Vars   map[string]any `json:"vars"`
	Colors map[string]any `json:"colors"`
}

// TestPiThemes_StatusLineKeysResolvable guards the contract between
// internal/assets/pi/themes/*.json and the deployed assets that paint the
// status line.
//
// pi's Theme.fg throws "Unknown theme color: X" for a key the active theme does
// not define, and pi's own theme fallbacks (withThemeColorFallbacks) do not
// cover statusLine*, so a missing key degrades the status line to its fallback
// instead of failing loudly. The second half of the test pins the stricter
// rule from pi's resolveVarRefs: a non-hex, non-empty string color value must
// name a declared var, or the whole theme fails to load with "Variable
// reference not found".
func TestPiThemes_StatusLineKeysResolvable(t *testing.T) {
	referenced := referencedStatusLineKeys(t)
	if len(referenced) == 0 {
		t.Fatal("no statusLine* key is referenced by any deployed pi asset: the scanner is stale, or the status line was removed without updating this guard")
	}

	themes := readPiThemes(t)
	if len(themes) == 0 {
		t.Fatal("no theme assets found under pi/themes")
	}

	byName := make(map[string]bool, len(themes))
	for _, th := range themes {
		byName[th.Name] = true
	}
	for _, want := range portedThemeNames {
		if !byName[want] {
			t.Errorf("ported theme %q is missing from internal/assets/pi/themes", want)
		}
	}

	for _, th := range themes {
		t.Run(th.Name, func(t *testing.T) {
			for _, key := range referenced {
				if _, ok := th.Colors[key]; !ok {
					t.Errorf("theme %s does not define %s, which a deployed asset reads via theme.fg: pi throws \"Unknown theme color\" and the status line degrades to its fallback", th.Name, key)
				}
			}
			for key, value := range th.Colors {
				s, ok := value.(string)
				if !ok {
					// Numbers are valid 256-color indices.
					continue
				}
				if s == "" || strings.HasPrefix(s, "#") {
					continue
				}
				if _, ok := th.Vars[s]; !ok {
					t.Errorf("theme %s color %q references %q, which is not declared in vars: pi's resolveVarRefs throws \"Variable reference not found\" and the theme fails to load", th.Name, key, s)
				}
			}
		})
	}
}

// referencedStatusLineKeys scans every deployed pi asset source for statusLine*
// color keys. Test files are scanned too: they encode expected theme keys.
func referencedStatusLineKeys(t *testing.T) []string {
	t.Helper()
	seen := make(map[string]bool)
	err := fs.WalkDir(assets.FS, "pi", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isPiAssetSource(path) {
			return nil
		}
		data, err := fs.ReadFile(assets.FS, path)
		if err != nil {
			return err
		}
		for _, m := range statusLineKeyPattern.FindAllStringSubmatch(string(data), -1) {
			seen[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk pi assets: %v", err)
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isPiAssetSource(path string) bool {
	for _, ext := range []string{".js", ".ts", ".mjs"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func readPiThemes(t *testing.T) []piThemeFile {
	t.Helper()
	entries, err := fs.ReadDir(assets.FS, "pi/themes")
	if err != nil {
		t.Fatalf("read pi/themes: %v", err)
	}
	themes := make([]piThemeFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := fs.ReadFile(assets.FS, "pi/themes/"+e.Name())
		if err != nil {
			t.Fatalf("read theme %s: %v", e.Name(), err)
		}
		var th piThemeFile
		if err := json.Unmarshal(data, &th); err != nil {
			t.Fatalf("theme %s is not valid JSON: %v", e.Name(), err)
		}
		if th.Name == "" {
			t.Errorf("theme %s has no name field", e.Name())
			continue
		}
		themes = append(themes, th)
	}
	return themes
}
