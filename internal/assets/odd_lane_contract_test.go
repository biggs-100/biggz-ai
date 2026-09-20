// Package assets_test verifies the embedded ODD lane contract: the skills tree
// ships skills/odd/SKILL.md, its frontmatter parses, the lazy delegation doc
// carries the ordered 7-step protocol, and the retired odd/plans path is never
// introduced.
package assets_test

import (
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"gopkg.in/yaml.v3"
)

type oddSkillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	License     string `yaml:"license"`
	Metadata    struct {
		Author  string `yaml:"author"`
		Version string `yaml:"version"`
	} `yaml:"metadata"`
}

func readEmbeddedAsset(t *testing.T, path string) string {
	t.Helper()
	data, err := assets.FS.ReadFile(path)
	if err != nil {
		t.Fatalf("assets.FS.ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}

func oddProtocolSection(t *testing.T) string {
	t.Helper()
	delegation := readEmbeddedAsset(t, "biggz/biggz-orchestrator-delegation.md")
	idx := strings.Index(delegation, "## ODD Lane")
	if idx == -1 {
		t.Fatalf("biggz-orchestrator-delegation.md missing the ODD protocol section heading")
	}
	return delegation[idx:]
}

func TestOddLaneContract(t *testing.T) {
	t.Run("embedded FS ships skills/odd/SKILL.md", func(t *testing.T) {
		content := readEmbeddedAsset(t, "skills/odd/SKILL.md")
		if !strings.HasPrefix(content, "---\n") {
			t.Fatalf("skills/odd/SKILL.md must start with YAML frontmatter")
		}
	})

	t.Run("frontmatter parses", func(t *testing.T) {
		content := readEmbeddedAsset(t, "skills/odd/SKILL.md")
		rest, ok := strings.CutPrefix(content, "---\n")
		if !ok {
			t.Fatalf("skills/odd/SKILL.md must start with YAML frontmatter")
		}
		fm, _, ok := strings.Cut(rest, "\n---")
		if !ok {
			t.Fatalf("skills/odd/SKILL.md missing frontmatter closing delimiter")
		}
		var parsed oddSkillFrontmatter
		if err := yaml.Unmarshal([]byte(fm), &parsed); err != nil {
			t.Fatalf("skills/odd/SKILL.md frontmatter parse error = %v", err)
		}
		if parsed.Name != "odd" {
			t.Errorf("frontmatter name = %q, want %q", parsed.Name, "odd")
		}
		if !strings.Contains(parsed.Description, "Trigger:") {
			t.Errorf("frontmatter description = %q, want a Trigger: keyword", parsed.Description)
		}
		if parsed.License == "" {
			t.Errorf("frontmatter license must not be empty")
		}
		if parsed.Metadata.Author == "" || parsed.Metadata.Version == "" {
			t.Errorf("frontmatter metadata author/version must not be empty")
		}
	})

	t.Run("delegation doc lists 7 ordered steps", func(t *testing.T) {
		section := oddProtocolSection(t)
		steps := []string{"Authorize", "Explore", "Resolve uncertainty", "Classify", "Track before", "Implement", "Close"}
		prev := -1
		for _, step := range steps {
			idx := strings.Index(section, step)
			if idx == -1 {
				t.Fatalf("ODD protocol section missing ordered step %q", step)
			}
			if idx <= prev {
				t.Fatalf("ODD protocol step %q is out of order (index %d <= %d)", step, idx, prev)
			}
			prev = idx
		}
		if !strings.Contains(section, "odd/tasks/<slug>.md") {
			t.Errorf("ODD protocol section missing the odd/tasks/<slug>.md document contract")
		}
	})

	t.Run("no odd/plans path or text", func(t *testing.T) {
		for _, path := range []string{"skills/odd/SKILL.md", "biggz/biggz-orchestrator-delegation.md"} {
			if strings.Contains(readEmbeddedAsset(t, path), "odd/plans") {
				t.Errorf("%s must not introduce the retired odd/plans path", path)
			}
		}
	})
}
