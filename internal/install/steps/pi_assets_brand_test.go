package steps

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// brandAssetExtensions are the binary/image extensions that must never appear
// under pi/. The upstream logo is the upstream project's mark, so copying it
// into the product would present upstream branding as biggz's own.
var brandAssetExtensions = []string{
	".png", ".jpg", ".jpeg", ".svg", ".ico", ".webp", ".gif", ".bmp",
}

// brandStringTokens are the upstream product names that must never appear
// inside a string literal in a pi asset source. This is a deliberate
// over-approximation: a string literal reaches users or the wire, so it is a
// brand surface, while a comment can name the source truthfully because the
// upstream policy permits nominative references. The comment heuristic is
// intentionally line-local and simple; it is a guard, not a parser.
var brandStringTokens = []string{"gentle-shell", "gentle-pi"}

// TestPiAssets_NoBrandAssets walks every file under pi/ and fails when one is a
// brand or image asset (.png, .jpg, .jpeg, .svg, .ico, .webp, .gif, .bmp).
//
// The invariant exists because the port copies visual behavior, not marks: the
// upstream logo must never be copied into the product, where it would read as
// biggz-ai presenting the upstream project's branding as its own.
func TestPiAssets_NoBrandAssets(t *testing.T) {
	err := fs.WalkDir(assets.FS, "pi", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(path)
		for _, ext := range brandAssetExtensions {
			if strings.HasSuffix(lower, ext) {
				t.Errorf("pi asset %s is a brand/image asset (%s): the upstream logo must never be copied into the product", path, ext)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk pi assets: %v", err)
	}
}

// TestPiAssets_NoBrandInStringLiterals scans exactly the deployed pi asset set
// returned by piExtensionsDeployList() and fails when the token "gentle-shell"
// or "gentle-pi" appears after a quote character (double quote, single quote or
// backtick) on a line whose first non-space token does not start a comment
// (//, /* or *).
//
// The invariant is about the shipped surface, and piExtensionsDeployList() is
// the single source of truth for what ships. Iterating it means the guard
// covers every deployed asset automatically and never a file that is never
// deployed; a future asset added to the deploy list is covered here with no
// edit to this test.
//
// Files that are not deployed, such as *.test.mjs and the test/ helpers, are
// deliberately out of scope: a test fixture may quote the upstream name in a
// parity assertion exactly as a comment may name the source truthfully. The
// two "(gentle-shell parity)" assertion messages in
// biggz-subagent-runtime.test.mjs are legitimate under this rule and must stay
// as they are. The heuristic encodes one rule: a comment may name the source
// truthfully, because the upstream trademark policy permits nominative
// references, but a string literal may not, because string literals reach users
// or the wire and would present the upstream name as biggz-ai's own surface.
// The line-local comment skip is a deliberate approximation; it keeps the guard
// cheap and readable while still catching the brand surfaces that ship.
func TestPiAssets_NoBrandInStringLiterals(t *testing.T) {
	deployed := piExtensionsDeployList()
	if len(deployed) == 0 {
		t.Fatal("piExtensionsDeployList() is empty: the guard would scan nothing, so a brand literal could ship unnoticed")
	}
	scanned := 0
	for _, entry := range deployed {
		path := entry.asset
		data, err := fs.ReadFile(assets.FS, path)
		if err != nil {
			t.Fatalf("read deployed asset %s: %v", path, err)
		}
		scanned++
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
				// Comment line: a truthful nominative reference is allowed.
				continue
			}
			for _, quote := range []byte{'"', '\'', '`'} {
				qi := strings.IndexByte(line, quote)
				if qi == -1 {
					continue
				}
				rest := line[qi+1:]
				for _, token := range brandStringTokens {
					if strings.Contains(rest, token) {
						t.Errorf("%s:%d uses the upstream brand token %q inside a string literal; a comment may name the source truthfully but a string literal reaches users or the wire: %s", path, i+1, token, trimmed)
					}
				}
			}
		}
	}
	t.Logf("scanned %d deployed pi assets", scanned)
}

// TestPortedFiles_DeclareProvenance fails unless each ported theme file carries
// a non-empty top-level $comment that names both the copyright holder and the
// license.
//
// The invariant exists because MIT requires the notice to travel with copies
// and substantial portions, and because a future edit that drops the comment
// would silently strip attribution from a file whose palette is upstream's.
func TestPortedFiles_DeclareProvenance(t *testing.T) {
	for _, name := range portedThemeNames {
		t.Run(name, func(t *testing.T) {
			path := "pi/themes/" + name + ".json"
			data, err := fs.ReadFile(assets.FS, path)
			if err != nil {
				t.Fatalf("read ported theme %s: %v", path, err)
			}
			var doc map[string]any
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatalf("ported theme %s is not valid JSON: %v", path, err)
			}
			comment, _ := doc["$comment"].(string)
			if strings.TrimSpace(comment) == "" {
				t.Fatalf("ported theme %s has no non-empty top-level $comment: the port must declare its source and license", path)
			}
			for _, want := range []string{"Mario Zechner", "MIT"} {
				if !strings.Contains(comment, want) {
					t.Errorf("ported theme %s $comment does not mention %q: provenance must name the copyright holder and the license, got %q", path, want, comment)
				}
			}
		})
	}
}
