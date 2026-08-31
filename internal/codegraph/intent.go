package codegraph

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	symbolRe  = regexp.MustCompile(`\b[A-Z][a-zA-Z0-9_]*\b`)
	keywordRe = regexp.MustCompile(`\b[a-z][a-z0-9_]{2,}\b`)
)

// ExtractIntent parses SDD artifacts for the given change and returns a weighted token map.
// Proposal is REQUIRED; spec/design/tasks are optional.
// Symbols (^[A-Z]...) weight 2, keywords (lowercase >=3 chars) weight 1.
func ExtractIntent(change, cwd string) (map[string]int, error) {
	if strings.TrimSpace(change) == "" {
		return nil, fmt.Errorf("proposal required")
	}
	root, err := resolveCwd(cwd)
	if err != nil {
		return nil, err
	}
	proposalPath := filepath.Join(root, "openspec", "changes", change, "proposal.md")
	data, err := os.ReadFile(proposalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("proposal required")
		}
		return nil, fmt.Errorf("proposal required: %w", err)
	}
	// Optional artifacts
	optional := []string{"spec.md", "design.md", "tasks.md"}
	// Also consider specs subfolder specs/*/...? For simplicity, check legacy paths and specs dir.
	var combined strings.Builder
	combined.Write(data)
	combined.WriteString("\n")
	for _, name := range optional {
		p := filepath.Join(root, "openspec", "changes", change, name)
		if b, err := os.ReadFile(p); err == nil {
			combined.Write(b)
			combined.WriteString("\n")
		}
		// Also check specs subdirectories (new structure: specs/<domain>/spec.md)
		specDir := filepath.Join(root, "openspec", "changes", change, "specs")
		if entries, err := os.ReadDir(specDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					candidate := filepath.Join(specDir, e.Name(), name)
					if b, err := os.ReadFile(candidate); err == nil {
						combined.Write(b)
						combined.WriteString("\n")
					}
					// also spec.md inside domain
					if name == "spec.md" {
						candidate2 := filepath.Join(specDir, e.Name(), "spec.md")
						if b, err := os.ReadFile(candidate2); err == nil {
							_ = b
						}
					}
				}
			}
		}
	}
	// Also directly read all files under specs for completeness
	specRoot := filepath.Join(root, "openspec", "changes", change, "specs")
	_ = filepath.WalkDir(specRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") {
			if b, err := os.ReadFile(path); err == nil {
				combined.Write(b)
				combined.WriteString("\n")
			}
		}
		return nil
	})

	content := combined.String()
	tokens := make(map[string]int)

	// Symbols first (weight 2)
	for _, m := range symbolRe.FindAllString(content, -1) {
		// Filter trivial single letter symbols? Keep >=2 chars
		if len(m) < 2 {
			continue
		}
		tokens[m] = WeightSymbol
	}
	// Keywords (weight 1) — only if not already a symbol token
	for _, m := range keywordRe.FindAllString(content, -1) {
		if _, exists := tokens[m]; exists {
			continue
		}
		// Also case-insensitive check: if upper variant exists, keep symbol weight
		foundSymbol := false
		for k := range tokens {
			if strings.EqualFold(k, m) && tokens[k] == WeightSymbol {
				foundSymbol = true
				break
			}
		}
		if foundSymbol {
			continue
		}
		// Normalize keyword to lower for consistent matching? Keep as lower.
		lower := strings.ToLower(m)
		if _, ok := tokens[lower]; !ok {
			tokens[lower] = WeightKeyword
			// also keep original case if different?
			if lower != m {
				tokens[m] = WeightKeyword
			}
		}
	}
	return tokens, nil
}

func resolveCwd(cwd string) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		cwd = "."
	}
	canonical, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		canonical = cwd
	}
	abs, err := filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("resolve cwd %q: %w", cwd, err)
	}
	return abs, nil
}
