// Package components provides modular component injection for agent configs.
package components

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/biggz-ai/biggz/internal/assets"
	"github.com/biggz-ai/biggz/internal/filemerge"
	"github.com/biggz-ai/biggz/internal/install"
	"github.com/biggz-ai/biggz/plugin"
)

// PersonaStyle defines available persona styles.
type PersonaStyle string

const (
	PersonaGentleman PersonaStyle = "gentleman"
	PersonaNeutral   PersonaStyle = "neutral"
	PersonaCustom    PersonaStyle = "custom"
)

// InjectPersona injects a persona style into the agent's AGENTS.md.
func InjectPersona(adapter plugin.AgentAdapter, homeDir string, style PersonaStyle, dryRun bool) error {
	if !adapter.SupportsSystemPrompt() {
		return nil
	}
	promptFile := adapter.SystemPromptFile(homeDir)
	if promptFile == "" {
		return nil
	}

	// Read persona content from embedded assets
	personaFile := fmt.Sprintf("biggz/biggz-persona.md")
	personaData, err := fs.ReadFile(assets.FS, personaFile)
	if err != nil {
		return fmt.Errorf("read persona: %w", err)
	}

	var existing []byte
	if _, err := os.Stat(promptFile); err == nil {
		existing, err = os.ReadFile(promptFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", promptFile, err)
		}
	}

	updated := install.InjectByMarker(string(existing), string(personaData), "biggz:persona")

	if dryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(promptFile), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(promptFile), err)
	}
	if _, err := filemerge.WriteFileAtomic(promptFile, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write %s: %w", promptFile, err)
	}
	return nil
}
