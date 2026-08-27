package skillregistry

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// ProviderPriority defines the explicit 7-provider ordered array (first match wins, oh-my-pi parity).
// Order mirrors oh-my-pi packages/skills provider priority:
//  1. user:opencode  -> $HOME/.config/opencode/skills
//  2. user:biggz     -> $HOME/.biggz/skills
//  3. user:claude    -> $HOME/.claude/skills
//  4. user:kilo      -> $HOME/.config/kilo/skills
//  5. project:skills -> <project>/skills
//  6. project:opencode -> <project>/.opencode/skills
//  7. project:github -> <project>/.github/skills
//
// First match wins deterministically; later providers with same skill name are ignored.
var ProviderPriority = [7]string{
	"user:opencode",
	"user:biggz",
	"user:claude",
	"user:kilo",
	"project:skills",
	"project:opencode",
	"project:github",
}

// ScanOpts controls filtering for ScanSkillsFromDir.
type ScanOpts struct {
	DisabledExtensions []string `json:"disabledExtensions"`
	Ignored            []string `json:"ignored"`
	Include            []string `json:"include"`
}

// ScanSkillsFromDir scans dir non-recursively (top-level only) via os.ReadDir.
// It filters by disabledExtensions (skill:<name>), ignored/include globs, and returns entries.
func ScanSkillsFromDir(dir string, opts ScanOpts) ([]Entry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	disabled := make(map[string]struct{}, len(opts.DisabledExtensions))
	for _, d := range opts.DisabledExtensions {
		if strings.HasPrefix(d, "skill:") {
			name := strings.TrimPrefix(d, "skill:")
			if name != "" {
				disabled[name] = struct{}{}
			}
		}
	}

	var result []Entry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "_shared" || name == "skill-registry" || strings.HasPrefix(name, "sdd-") {
			continue
		}
		if _, skip := disabled[name]; skip {
			continue
		}
		if matchesAnyGlob(name, opts.Ignored) {
			continue
		}
		if len(opts.Include) > 0 && !matchesAnyGlob(name, opts.Include) {
			continue
		}
		skillPath := filepath.Join(dir, name, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			continue
		}
		desc := extractDescription(skillPath)
		// Path is stored as relative or absolute depending on caller; for ScanSkillsFromDir we store the skillPath as-is.
		result = append(result, Entry{
			Name:        name,
			Path:        filepath.ToSlash(skillPath),
			Description: desc,
		})
	}
	return result, nil
}

func matchesAnyGlob(name string, patterns []string) bool {
	for _, pat := range patterns {
		// Use path.Match for slash-free name globs; fallback to filepath.Match for OS-specific.
		matched, err := path.Match(pat, name)
		if err == nil && matched {
			return true
		}
		// Try filepath.Match as well for broader compatibility (e.g., *_test*).
		if m, err2 := filepath.Match(pat, name); err2 == nil && m {
			return true
		}
		// Simple substring fallback for patterns like *_test* if Match fails due to invalid pattern.
		// path.Match already handles *_test*, but keep for robustness.
		if strings.Contains(pat, "*") {
			trimmed := strings.ReplaceAll(pat, "*", "")
			if trimmed != "" && strings.Contains(name, trimmed) {
				// Only use fallback when pattern is trivial wildcard wrapper.
				// Ensure we don't over-match; check prefix/suffix heuristics.
				if strings.HasPrefix(pat, "*") && strings.HasSuffix(pat, "*") {
					if strings.Contains(name, trimmed) {
						return true
					}
				}
			}
		}
	}
	return false
}

// ResolveSkillURI resolves skill://<name>/<path> via path.Clean + filepath.EvalSymlinks + strings.HasPrefix.
// It rejects .. after Clean, absolute paths, and symlink escapes after EvalSymlinks.
func ResolveSkillURI(uri string, roots map[string]string) ([]byte, error) {
	const prefix = "skill://"
	if !strings.HasPrefix(uri, prefix) {
		return nil, fmt.Errorf("invalid skill URI %q: must start with %s", uri, prefix)
	}
	rest := uri[len(prefix):]
	if rest == "" {
		return nil, fmt.Errorf("invalid skill URI %q: missing skill name", uri)
	}
	slash := strings.Index(rest, "/")
	var name, rel string
	if slash == -1 {
		return nil, fmt.Errorf("invalid skill URI %q: missing path", uri)
	}
	name = rest[:slash]
	rel = rest[slash+1:]
	if name == "" {
		return nil, fmt.Errorf("invalid skill URI %q: empty skill name", uri)
	}
	if rel == "" {
		return nil, fmt.Errorf("invalid skill URI %q: empty path", uri)
	}
	// Reject absolute paths: rel starting with / or \ or containing // absolute.
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") {
		return nil, fmt.Errorf("invalid skill URI %q: absolute path rejected", uri)
	}
	// Use path.Clean (slash semantics) for containment.
	cleaned := path.Clean(rel)
	if cleaned == "." {
		return nil, fmt.Errorf("invalid skill URI %q: empty cleaned path", uri)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return nil, fmt.Errorf("invalid skill URI %q: traversal with .. rejected", uri)
	}
	// Also reject any remaining .. segment after Clean.
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == ".." {
			return nil, fmt.Errorf("invalid skill URI %q: traversal with .. rejected", uri)
		}
	}
	root, ok := roots[name]
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	if root == "" {
		return nil, fmt.Errorf("skill %q has empty root", name)
	}
	// Resolve root realpath.
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		// If root doesn't exist yet, use Clean path for HasPrefix check; but real skill roots should exist.
		rootReal = root
	}
	rootReal = filepath.Clean(rootReal)

	candidate := filepath.Join(root, filepath.FromSlash(cleaned))
	// EvalSymlinks candidate; handle file vs directory symlinks.
	candidateReal, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// If file doesn't exist, try eval of parent dir + clean check.
		// For traversal protection, we still need to ensure candidate is under root.
		// If EvalSymlinks fails because file missing, check prefix via cleaned join without symlink.
		cleanCandidate := filepath.Clean(candidate)
		if !isSubpath(rootReal, cleanCandidate) {
			return nil, fmt.Errorf("skill URI %q escapes root", uri)
		}
		// File missing: return not found error.
		return nil, fmt.Errorf("skill URI %q not found: %w", uri, err)
	}
	candidateReal = filepath.Clean(candidateReal)
	if !isSubpath(rootReal, candidateReal) {
		return nil, fmt.Errorf("skill URI %q escapes root via symlink", uri)
	}
	data, err := os.ReadFile(candidateReal)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func isSubpath(root, candidate string) bool {
	// Ensure both are cleaned absolute or relative consistently.
	// Use filepath.Clean and check HasPrefix with path separator boundary.
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	if root == candidate {
		return true
	}
	// Ensure root ends without separator for prefix check.
	if !strings.HasSuffix(root, string(filepath.Separator)) {
		root += string(filepath.Separator)
	}
	return strings.HasPrefix(candidate, root) || candidate == strings.TrimSuffix(root, string(filepath.Separator))
}

// PromptData is the unified data inventory for all review prompts.
// Each template may use a subset of these fields; missingkey=error ensures typos fail fast.
type PromptData struct {
	Repo         string
	ChangedLines int
	Paths        []string
	Diff         string
	Truncated    bool
	BaseTree     string
	Hunks        string
	Shared       string
	Payload      string
	Files        []string
}

// LoadPrompt loads a prompt template from assets.FS via text/template with missingkey=error.
func LoadPrompt(name string) (*template.Template, error) {
	if name == "" {
		return nil, fmt.Errorf("prompt name empty")
	}
	// Normalize name: allow with or without .md suffix, with or without prompts/review prefix.
	clean := strings.TrimPrefix(name, "prompts/review/")
	clean = strings.TrimPrefix(clean, "review/")
	if !strings.HasSuffix(clean, ".md") {
		clean += ".md"
	}
	assetPath := path.Join("prompts/review", clean)
	data, err := assets.FS.ReadFile(assetPath)
	if err != nil {
		return nil, fmt.Errorf("load prompt %q: %w", name, err)
	}
	tmpl, err := template.New(clean).Option("missingkey=error").Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse prompt %q: %w", name, err)
	}
	return tmpl, nil
}

// RenderPrompt is a helper to load and execute a prompt in one call.
func RenderPrompt(name string, data any) (string, error) {
	tmpl, err := LoadPrompt(name)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
