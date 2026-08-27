package lens

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// PromptData is the unified inventory for all review prompt templates.
// Each template may use a subset; missingkey=error ensures typos fail fast.
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
}

// LoadPrompt loads a prompt template from assets.FS via text/template with missingkey=error.
func LoadPrompt(name string) (*template.Template, error) {
	if name == "" {
		return nil, fmt.Errorf("prompt name empty")
	}
	clean := name
	if len(clean) > 3 && clean[len(clean)-3:] != ".md" {
		clean += ".md"
	}
	// Normalize prefix handling.
	if len(clean) >= 15 && clean[:15] == "prompts/review/" {
		// already prefixed
	} else if len(clean) >= 7 && clean[:7] == "review/" {
		clean = "prompts/" + clean
	} else {
		clean = "prompts/review/" + clean
	}
	data, err := assets.FS.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("load prompt %q: %w", name, err)
	}
	tmpl, err := template.New(clean).Option("missingkey=error").Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse prompt %q: %w", name, err)
	}
	return tmpl, nil
}

// RenderPrompt loads and executes a prompt template with the given data.
func RenderPrompt(name string, data PromptData) (string, error) {
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

// RenderPromptWithMap is a helper for tests that need missingkey verification with generic maps.
func RenderPromptWithMap(name string, data any) (string, error) {
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
