// Package hooks provides a local command hook system for biggz-ai lifecycle events.
// Hooks are scripts or commands that run when specific events occur, configured
// via .biggz/hooks.yaml. No network calls, no webhooks — only local execution.
package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Event names
const (
	EventReviewStart     = "on_review_start"
	EventReviewComplete  = "on_review_complete"
	EventApplyDone       = "on_apply_done"
	EventPRCreated       = "on_pr_created"
	EventInstallDone     = "on_install_done"
)

// HookConfig is the top-level hooks configuration.
type HookConfig struct {
	Hooks map[string][]HookDef `yaml:"hooks"`
}

// HookDef defines a single hook action.
type HookDef struct {
	// Command to run (executed via shell on Unix, cmd on Windows)
	Command string `yaml:"command,omitempty"`
	// Description for logging
	Description string `yaml:"description,omitempty"`
	// Timeout in seconds (default: 30)
	Timeout int `yaml:"timeout,omitempty"`
	// ContinueOnError: if true, hook failure doesn't block the event
	ContinueOnError bool `yaml:"continue_on_error,omitempty"`
}

// Manager loads and executes hooks.
type Manager struct {
	config *HookConfig
	root   string
}

// NewManager creates a hook manager from a project root.
func NewManager(projectRoot string) *Manager {
	return &Manager{root: projectRoot}
}

// Load reads .biggz/hooks.yaml from the project root.
func (m *Manager) Load() error {
	paths := []string{
		filepath.Join(m.root, ".biggz", "hooks.yaml"),
		filepath.Join(m.root, ".biggz", "hooks.yml"),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			var cfg HookConfig
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return fmt.Errorf("parse %s: %w", p, err)
			}
			if cfg.Hooks == nil {
				cfg.Hooks = make(map[string][]HookDef)
			}
			m.config = &cfg
			return nil
		}
	}
	// No hooks file — not an error
	m.config = &HookConfig{Hooks: make(map[string][]HookDef)}
	return nil
}

// Dispatch runs all hooks registered for the given event.
func (m *Manager) Dispatch(event string, args ...string) *HookResults {
	results := &HookResults{Event: event}
	if m.config == nil {
		return results
	}
	hooks, ok := m.config.Hooks[event]
	if !ok {
		return results
	}
	for _, hook := range hooks {
		r := m.runHook(hook, args...)
		results.Results = append(results.Results, *r)
		if !r.Success && !hook.ContinueOnError {
			results.Blocked = true
			return results
		}
	}
	return results
}

func (m *Manager) runHook(hook HookDef, args ...string) *HookResult {
	r := &HookResult{Command: hook.Command, Description: hook.Description}
	if hook.Command == "" {
		r.Error = "empty command"
		return r
	}

	cmd := exec.Command("sh", "-c", hook.Command)
	if len(args) > 0 {
		cmd.Args = append(cmd.Args, args...)
	}
	// On Windows, use cmd
	if isWindows() {
		cmd = exec.Command("cmd", "/c", hook.Command)
	}

	output, err := cmd.CombinedOutput()
	r.Output = string(output)
	if err != nil {
		r.Error = err.Error()
		r.Success = false
	} else {
		r.Success = true
	}
	return r
}

// HookResult records the outcome of a single hook execution.
type HookResult struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	Success     bool   `json:"success"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
}

// HookResults aggregates results from all hooks for one event.
type HookResults struct {
	Event   string       `json:"event"`
	Results []HookResult `json:"results"`
	Blocked bool         `json:"blocked"`
}

// Success returns true if all hooks ran without error.
func (hr *HookResults) Success() bool {
	return !hr.Blocked && len(hr.Results) > 0
}

func isWindows() bool {
	return len(os.Getenv("windir")) > 0 || len(os.Getenv("COMSPEC")) > 0
}

// DefaultHooksYAML returns the default hooks configuration content.
func DefaultHooksYAML() string {
	return `# biggz-ai hooks
# Commands run on lifecycle events. All commands are local — no network calls.
hooks:
  on_review_complete:
    - command: "echo 'Review completed'"
      description: "Log review completion"
      continue_on_error: true

  on_apply_done:
    - command: "go test ./..."
      description: "Run tests after apply"
      continue_on_error: true

  on_pr_created:
    - command: "echo 'PR created'"
      description: "Notify PR creation"
      continue_on_error: true
`
}
