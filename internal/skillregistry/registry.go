// Package skillregistry scans skill directories and generates the
// .atl/skill-registry.md index for sub-agent skill resolution.
// Supports content-fingerprint caching to avoid unnecessary regeneration.
package skillregistry

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// RegistrySchema is the schema version for cache invalidation.
const RegistrySchema = 2

// Entry represents a single skill in the registry.
type Entry struct {
	Name        string
	Path        string // relative to project root
	Description string
}

// Result describes what the registry refresh produced.
type Result struct {
	Regenerated bool
	SkillCount  int
	Registry    string // path to registry file
	Cached      bool   // true if cache was valid and no regeneration was needed
}

// cacheFile is the on-disk cache format.
type cacheFile struct {
	Schema      int    `json:"schema"`
	Fingerprint string `json:"fingerprint"`
	GeneratedAt string `json:"generated_at"`
}

// ─── Fingerprint ─────────────────────────────────────────────────────────────

// Fingerprint computes a content-aware fingerprint for a set of skill files.
// Includes: schema version + filename + modtime + size + first 256 bytes of content.
// This detects content edits even when size and timestamp are unchanged.
func providerDir(provider, projectRoot, home string) string {
	switch provider {
	case "user:opencode":
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".config", "opencode", "skills")
	case "user:biggz":
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".biggz", "skills")
	case "user:claude":
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".claude", "skills")
	case "user:kilo":
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".config", "kilo", "skills")
	case "project:skills":
		return filepath.Join(projectRoot, "skills")
	case "project:opencode":
		return filepath.Join(projectRoot, ".opencode", "skills")
	case "project:github":
		return filepath.Join(projectRoot, ".github", "skills")
	case "project:claude":
		return filepath.Join(projectRoot, ".claude", "skills")
	default:
		return ""
	}
}

// uniqueExistingDirs resolves ProviderPriority directories for projectRoot,
// dedupes by cleaned path, filters to existing directories, and skips missing.
func uniqueExistingDirs(projectRoot string) []string {
	home, _ := os.UserHomeDir()
	seen := make(map[string]struct{})
	var out []string
	for _, p := range ProviderPriority {
		dir := providerDir(p, projectRoot, home)
		if dir == "" {
			continue
		}
		clean := filepath.Clean(dir)
		if _, ok := seen[clean]; ok {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, dir)
	}
	return out
}

func Fingerprint(projectRoot string) string {
	home, _ := os.UserHomeDir()
	var lines []string
	lines = append(lines, fmt.Sprintf("schema:%d", RegistrySchema))

	// Collect all SKILL.md paths from ProviderPriority ordered dirs (oh-my-pi parity)
	seen := map[string]bool{}
	scanDirs := []string{}
	for _, p := range ProviderPriority {
		dir := providerDir(p, projectRoot, home)
		if dir == "" {
			continue
		}
		scanDirs = append(scanDirs, dir)
	}

	for _, dir := range scanDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "_shared" || name == "skill-registry" || strings.HasPrefix(name, "sdd-") {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true

			skillPath := filepath.Join(dir, name, "SKILL.md")
			info, err := os.Stat(skillPath)
			if err != nil {
				continue
			}

			// Include metadata
			lines = append(lines, fmt.Sprintf("%s:%d:%d", skillPath, info.ModTime().UnixNano(), info.Size()))

			// Include first 256 bytes of content to detect content-only changes
			if data, err := os.ReadFile(skillPath); err == nil {
				contentBytes := data
				if len(contentBytes) > 256 {
					contentBytes = contentBytes[:256]
				}
				lines = append(lines, fmt.Sprintf("content:%x", sha1.Sum(contentBytes)))
			}
		}
	}

	// Hash all lines
	sort.Strings(lines)
	h := sha1.Sum([]byte(strings.Join(lines, "\n")))
	return fmt.Sprintf("%x", h)
}

// ─── Cache ───────────────────────────────────────────────────────────────────

func cachePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".atl", ".skill-registry.cache.json")
}

func readCache(projectRoot string) (*cacheFile, error) {
	data, err := os.ReadFile(cachePath(projectRoot))
	if err != nil {
		return nil, err
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	return &cf, nil
}

func writeCache(projectRoot, fingerprint string) error {
	cf := cacheFile{
		Schema:      RegistrySchema,
		Fingerprint: fingerprint,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(projectRoot), data, 0644)
}

// ─── Refresh ─────────────────────────────────────────────────────────────────

// Refresh scans skill directories and regenerates the registry.
// Uses content fingerprinting to skip regeneration when nothing changed.
// Set force=true to bypass cache.
func Refresh(projectRoot string, force bool) (*Result, error) {
	atlDir := filepath.Join(projectRoot, ".atl")
	if err := os.MkdirAll(atlDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir .atl: %w", err)
	}

	// Check cache
	if !force {
		if cf, err := readCache(projectRoot); err == nil {
			if cf.Schema == RegistrySchema {
				currentFP := Fingerprint(projectRoot)
				if cf.Fingerprint == currentFP {
					return &Result{
						Regenerated: false,
						SkillCount:  0,
						Registry:    filepath.Join(atlDir, "skill-registry.md"),
						Cached:      true,
					}, nil
				}
			}
		}
	}

	// Cache miss or force: regenerate
	entries := scanAllSkills(projectRoot)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	content := generateRegistry(entries)
	registryPath := filepath.Join(atlDir, "skill-registry.md")
	if err := os.WriteFile(registryPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write registry: %w", err)
	}

	// Update cache
	fp := Fingerprint(projectRoot)
	if err := writeCache(projectRoot, fp); err != nil {
		// Cache write failure is non-fatal
		_ = err
	}

	return &Result{
		Regenerated: true,
		SkillCount:  len(entries),
		Registry:    registryPath,
	}, nil
}

// ─── Scanning ────────────────────────────────────────────────────────────────

func scanAllSkills(projectRoot string) []Entry {
	return scanAllSkillsWithOpts(projectRoot, ScanOpts{})
}

func scanAllSkillsWithOpts(projectRoot string, opts ScanOpts) []Entry {
	home, _ := os.UserHomeDir()
	seen := map[string]bool{}
	var entries []Entry

	scanDirs := []string{}
	for _, p := range ProviderPriority {
		dir := providerDir(p, projectRoot, home)
		if dir == "" {
			continue
		}
		scanDirs = append(scanDirs, dir)
	}

	for _, dir := range scanDirs {
		entries = append(entries, scanDirWithOpts(dir, seen, projectRoot, opts)...)
	}

	return entries
}

func scanAllSkillsWithPriority(projectRoot string, priority [7]string, opts ScanOpts) []Entry {
	home, _ := os.UserHomeDir()
	seen := map[string]bool{}
	var entries []Entry
	for _, p := range priority {
		dir := providerDir(p, projectRoot, home)
		if dir == "" {
			continue
		}
		entries = append(entries, scanDirWithOpts(dir, seen, projectRoot, opts)...)
	}
	return entries
}

func scanDir(dir string, seen map[string]bool, projectRoot string) []Entry {
	return scanDirWithOpts(dir, seen, projectRoot, ScanOpts{})
}

func scanDirWithOpts(dir string, seen map[string]bool, projectRoot string, opts ScanOpts) []Entry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
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

		if name == "_shared" || name == "skill-registry" {
			continue
		}
		if strings.HasPrefix(name, "sdd-") {
			continue
		}
		if seen[name] {
			continue
		}
		if _, skip := disabled[name]; skip {
			continue
		}
		if matchesGlob(name, opts.Ignored) {
			continue
		}
		if len(opts.Include) > 0 && !matchesGlob(name, opts.Include) {
			continue
		}

		skillDir := filepath.Join(dir, name)
		skillPath := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			continue
		}

		desc := extractDescription(skillPath)

		rel, err := filepath.Rel(projectRoot, skillPath)
		p := skillPath
		if err == nil {
			p = rel
		}
		p = filepath.ToSlash(p)

		seen[name] = true
		result = append(result, Entry{
			Name:        name,
			Path:        p,
			Description: desc,
		})
	}

	return result
}

func matchesGlob(name string, patterns []string) bool {
	for _, pat := range patterns {
		if ok, _ := path.Match(pat, name); ok {
			return true
		}
		if ok, _ := filepath.Match(pat, name); ok {
			return true
		}
	}
	return false
}

func extractDescription(skillPath string) string {
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return ""
	}
	content := string(data)

	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end > 0 {
			frontmatter := content[3 : 3+end]
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "description:") {
					desc := strings.TrimSpace(line[len("description:"):])
					desc = strings.Trim(desc, "\"'")
					return desc
				}
			}
		}
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
	}

	return ""
}

// ─── Registry generation ─────────────────────────────────────────────────────

func generateRegistry(entries []Entry) string {
	var b strings.Builder

	b.WriteString("# Skill Registry — biggz-ai\n\n")
	b.WriteString("<!-- Auto-generated by biggz-ai skill-registry refresh. ")
	b.WriteString("Run `biggz skill-registry refresh` to regenerate. -->\n\n")
	b.WriteString(fmt.Sprintf("Last updated: %s\n\n", time.Now().Format("2006-01-02")))
	b.WriteString("## Contract\n\n")
	b.WriteString("**Delegator use only.** This registry is an index, not a summary. ")
	b.WriteString("Any agent that launches subagents reads it to select relevant skills, ")
	b.WriteString("then passes exact `SKILL.md` paths for the subagent to read before work.\n\n")
	b.WriteString("`SKILL.md` remains the source of truth.\n\n")
	b.WriteString("## Skills\n\n")
	b.WriteString("| Skill | Trigger / description | Scope | Path |\n")
	b.WriteString("| --- | --- | --- | --- |\n")

	for _, e := range entries {
		scope := "project"
		if strings.Contains(e.Path, "/.config/") || strings.Contains(e.Path, "\\.config\\") || strings.Contains(e.Path, string(filepath.Separator)+".config"+string(filepath.Separator)) {
			scope = "user"
		}
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | `%s` |\n", e.Name, e.Description, scope, e.Path))
	}

	b.WriteString("\n## Loading protocol\n\n")
	b.WriteString("1. Match task context and target files against the `Trigger / description` column.\n")
	b.WriteString("2. Pass only the matching `Path` values to the subagent under `## Skills to load before work`.\n")
	b.WriteString("3. Instruct the subagent to read those exact `SKILL.md` files before reading, writing, reviewing, testing, or creating artifacts.\n")
	b.WriteString("4. If no matching skill exists, proceed without project skill injection and report `skill_resolution: none`.\n")

	return b.String()
}
