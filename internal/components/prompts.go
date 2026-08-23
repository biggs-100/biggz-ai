package components

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/plugin"
)

type promptsComponent struct {
	homeDir string
	fsys    fs.FS
}

// NewPromptsComponent creates a Component that deploys embedded SDD prompt
// files to the agent's prompts directory. The fsys parameter should be the
// embedded assets filesystem (use assets.FS in production).
func NewPromptsComponent(homeDir string, fsys fs.FS) Component {
	return &promptsComponent{homeDir: homeDir, fsys: fsys}
}

func (c *promptsComponent) ID() string { return "prompts" }

func (c *promptsComponent) Deploy(ctx context.Context, adapter plugin.AgentAdapter) (*DeploymentResult, error) {
	promptsDir := filepath.Join(adapter.GlobalConfigDir(c.homeDir), "prompts", "sdd")

	// Collect file list from the embedded FS before deploying
	var files []string
	fs.WalkDir(c.fsys, "prompts/sdd", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	err := install.DeployPrompts(promptsDir, c.fsys, false)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return &DeploymentResult{Changed: false}, nil
	}

	return &DeploymentResult{
		Changed: true,
		Files:   files,
	}, nil
}

var _ Component = (*promptsComponent)(nil)
var _ = assets.FS

// SharedPromptDir returns the shared SDD prompt directory beside OpenCode's
// settings file, including when XDG_CONFIG_HOME overrides the default root.
func SharedPromptDir(homeDir string) string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "opencode", "prompts", "sdd")
	}
	return filepath.Join(homeDir, ".config", "opencode", "prompts", "sdd")
}

// SharedPromptFileRef returns a prompt file reference relative to the settings
// file that will contain it.
func SharedPromptFileRef(settingsPath, homeDir, phase string) (string, error) {
	return sharedPromptFileRef(settingsPath, homeDir, phase, filepath.Rel)
}

func sharedPromptFileRef(settingsPath, homeDir, phase string, rel func(string, string) (string, error)) (string, error) {
	promptPath := filepath.Join(SharedPromptDir(homeDir), phase+".md")
	relativePath, err := rel(filepath.Dir(settingsPath), promptPath)
	if err != nil {
		return "{file:" + filepath.ToSlash(promptPath) + "}", nil
	}
	relativePath = filepath.ToSlash(relativePath)
	if !strings.HasPrefix(relativePath, ".") {
		relativePath = "./" + relativePath
	}
	return "{file:" + relativePath + "}", nil
}

// profilePhaseOrder is the canonical ordered list of SDD phases that have
// shared prompt files. Used by WriteSharedPromptFiles and backup enumeration.
var profilePhaseOrder = []string{
	"sdd-init",
	"sdd-explore",
	"sdd-propose",
	"sdd-spec",
	"sdd-design",
	"sdd-tasks",
	"sdd-apply",
	"sdd-verify",
	"sdd-archive",
	"sdd-onboard",
}

var subAgentPhaseOrder = profilePhaseOrder

// ProfilePhaseOrder returns the ordered list of phase names.
func ProfilePhaseOrder() []string {
	return profilePhaseOrder
}

// SharedPromptPhases returns the ordered list of phase names that have shared
// prompt files in SharedPromptDir(). Used by backup target enumeration and any
// caller that needs to enumerate all prompt files without importing internal vars.
func SharedPromptPhases() []string {
	return ProfilePhaseOrder()
}

// readSkillContent reads the embedded skill content for the given phase.
func readSkillContent(phase string) (string, error) {
	data, err := fs.ReadFile(assets.FS, "skills/"+phase+"/SKILL.md")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// extractModelSection extracts the section matching the given capability
// ("capable" or "small") from content containing
// <!-- section:model-capable --> and <!-- section:model-small --> markers.
// If no matching section is found, the full content is returned.
func extractModelSection(content, capability string) string {
	return filemerge.ExtractHTMLCommentSection(content, "model-"+capability)
}

// agentLanguageContract returns the canonical executor language contract
// (issue #1702 defect 4). Single source of truth: injected into every
// rendered sub-agent prompt so executors spawned inside a non-English
// conversation never mimic its dialect when writing artifacts.
func agentLanguageContract() string {
	if data, err := fs.ReadFile(assets.FS, "generic/agent-language-contract.md"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return strings.TrimSpace(`## Artifact Language Contract

Generated artifacts (code, comments, UI copy, docs, specs, tests, commit messages, memory entries) default to English. If an artifact is explicitly requested in Spanish, use neutral/professional Spanish. Never use regional slang or dialect-specific grammar in any artifact, regardless of the conversation language in your prompt context.

Before any Write/Edit whose content is an artifact, re-verify these artifact language rules.`)
}

// injectLanguageContractIntoPrompt appends the canonical language contract
// as a managed markdown section. Marker-bound, so re-rendering an already
// injected prompt is a no-op (same mechanism as the CodeGraph guidance).
func injectLanguageContractIntoPrompt(prompt string) string {
	return filemerge.InjectMarkdownSection(prompt, "agent-language-contract", strings.TrimSpace(agentLanguageContract()))
}

// WriteSharedPromptFiles writes the 10 SDD sub-agent prompt files to
// {homeDir}/.config/opencode/prompts/sdd/. The content for each phase is extracted
// from the embedded skill file, filtered to the section matching the phase's
// model capability ("capable" or "small").
//
// The phaseCapabilities map controls which section is extracted per phase:
//   - "capable" sections are used for high-capability models
//   - "small" sections are used for small/fast models (e.g., flash, mini)
//   - If a phase is missing from the map, "capable" is used as default
//
// Returns (true, nil) if any file was created or changed, (false, nil) if all
// files already match (idempotent). Uses WriteFileAtomic so the operation is
// safe to repeat.
func WriteSharedPromptFiles(homeDir string, phaseCapabilities map[string]string) (bool, error) {
	promptDir := SharedPromptDir(homeDir)
	anyChanged := false

	for _, phase := range subAgentPhaseOrder {
		skillContent, err := readSkillContent(phase)
		if err != nil {
			return false, err
		}

		capability := "capable"
		if phaseCapabilities != nil {
			if cap, ok := phaseCapabilities[phase]; ok && cap != "" {
				capability = cap
			}
		}

		content := extractModelSection(skillContent, capability)
		content = injectLanguageContractIntoPrompt(content)

		path := filepath.Join(promptDir, phase+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return false, err
		}
		result, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return false, err
		}

		if result.Changed || result.Created {
			anyChanged = true
		}
	}

	return anyChanged, nil
}
