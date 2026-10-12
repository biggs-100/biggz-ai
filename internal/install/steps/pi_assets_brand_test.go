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

// brandLiteralAllowlist enumerates the interop and compatibility identifiers
// that may carry "gentle" inside a deployed string literal.
//
// A comment may name the upstream project truthfully because the trademark
// policy permits nominative references, but a string literal reaches users or
// the wire, so it is a brand surface. These entries are the narrow exceptions:
// each names an identifier biggz must speak to interoperate with a documented
// compatibility alias, an internal registry key, or the review protocol the
// ported sidebar is not yet wired to. Every entry states its reason in a
// comment, because an exception the reader cannot justify is indistinguishable
// from an oversight.
//
// Keep the list closed. Anything a rename can fix does not belong here, and the
// stale-entry sub-test below fails once an entry stops appearing, so an
// exception cannot quietly outlive its reason.
var brandLiteralAllowlist = []string{
	// GENTLE_AI_NO_ANIMATION: documented compatibility alias, BIGGZ_NO_ANIMATION
	// being the primary name; the deployed readers are
	// biggz-extension-api.js:68 and biggz-tool-pills.js:31, and it is recorded in
	// docs/comparison-with-gentle.md:110. (biggz-footer.js:114 used to read it
	// too; that asset was retired from the deploy list in unit 2.4, so it no
	// longer keeps this entry live.)
	"GENTLE_AI_NO_ANIMATION",
	// GENTLE_PI_NO_SKILL_REGISTRY: documented compatibility alias;
	// internal/skillregistry/watcher.go:31 reads BIGGZ_NO_SKILL_REGISTRY first
	// and this one second.
	"GENTLE_PI_NO_SKILL_REGISTRY",
	// __gentlePiSkillRegistryExtensionSource: internal global registry key at
	// skill-registry.ts:31, never user-visible.
	"__gentlePiSkillRegistryExtensionSource",
	// gentle-ai:review-sidebar: the upstream review-protocol event the ported
	// sidebar listens on. Biggz speaks its own contract, so task 2.11 rewires
	// this; renaming the string now would hide the mismatch instead of fixing
	// it.
	"gentle-ai:review-sidebar",
	// gentle-ai.review-last-event-closure/v1: the same, for the closure schema.
	"gentle-ai.review-last-event-closure/v1",
	// gentle-ai.review-integration.status: the same, for the status schema the
	// parser matches by regex.
	"gentle-ai.review-integration.status",
}

// brandLiteralAllowlisted reports whether an already-detected "gentle" literal
// is one of the allowlisted interop identifiers.
//
// The status schema ships inside a regex literal, so its dots are escaped
// (gentle-ai\.review-integration\.status). Unescaping only escaped dots keeps
// the allowlist entry readable as the identifier it names and lets the live
// check match its own source form; unescaping nothing else keeps the match
// narrow.
func brandLiteralAllowlisted(literal string) bool {
	unescaped := strings.ReplaceAll(literal, `\.`, ".")
	for _, entry := range brandLiteralAllowlist {
		if strings.Contains(unescaped, entry) {
			return true
		}
	}
	return false
}

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
// returned by piExtensionsDeployList() and fails when "gentle" appears inside a
// string literal (the text after a double quote, single quote or backtick) on a
// line whose first non-space token does not start a comment (//, /* or *),
// unless that same literal is an allowlisted interop identifier.
//
// The invariant is about the shipped surface, and piExtensionsDeployList() is
// the single source of truth for what ships. Iterating it means the guard
// covers every deployed asset automatically and never a file that is never
// deployed; a future asset added to the deploy list is covered here with no
// edit to this test.
//
// The rule is deliberately wider than the two upstream product names it used to
// name: the namespace is "gentle", not two spellings of it, and a guard that
// bans only the examples found on the day it was written reads as coverage
// while missing the rest of the family. A comment may name the source
// truthfully, because the upstream trademark policy permits nominative
// references, but a string literal may not, because string literals reach users
// or the wire and would present the upstream name as biggz-ai's own surface.
// The only exceptions are the enumerated interop identifiers in
// brandLiteralAllowlist, each carrying its reason. The line-local comment skip
// is a deliberate approximation; it keeps the guard cheap and readable while
// still catching the brand surfaces that ship.
//
// Files that are not deployed, such as *.test.mjs and the test/ helpers, are
// deliberately out of scope: a test fixture may quote the upstream name in a
// parity assertion exactly as a comment may name the source truthfully.
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
				if !strings.Contains(strings.ToLower(rest), "gentle") {
					continue
				}
				if brandLiteralAllowlisted(rest) {
					continue
				}
				t.Errorf("%s:%d carries \"gentle\" inside a string literal: %s\n"+
					"a comment may name the upstream source truthfully, because the trademark policy permits nominative references, but a string literal reaches users or the wire and must not carry upstream branding; if this literal is a genuine interop or compatibility identifier, add it to brandLiteralAllowlist with its reason, otherwise rename it to biggz's own name",
					path, i+1, trimmed)
				break
			}
		}
	}
	t.Logf("scanned %d deployed pi assets", scanned)

	// A stale exception protects nothing and is indistinguishable from an
	// oversight the next time someone reads the list, so fail until it is
	// deleted.
	t.Run("allowlist entries are live", func(t *testing.T) {
		for _, entry := range brandLiteralAllowlist {
			live := false
			for _, deployedEntry := range deployed {
				data, err := fs.ReadFile(assets.FS, deployedEntry.asset)
				if err != nil {
					t.Fatalf("read deployed asset %s: %v", deployedEntry.asset, err)
				}
				if strings.Contains(strings.ReplaceAll(string(data), `\.`, "."), entry) {
					live = true
					break
				}
			}
			if !live {
				t.Errorf("brand-literal allowlist entry %q no longer appears anywhere in the deployed pi asset set: delete it from brandLiteralAllowlist rather than leaving a stale exception to accrue", entry)
			}
		}
	})
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
