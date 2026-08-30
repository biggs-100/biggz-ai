package sdd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Background canonical resolution — 4-source strict 2-key.
// Port of extensions/gentle-ai.ts with .biggz paths and BIGGZ_* precedence.

// BackgroundSubagentsSchema is the schema identifier.
const BackgroundSubagentsSchema = "gentle-pi.background-subagents/v1"

// BackgroundSubagentsFile is the backing file name.
const BackgroundSubagentsFile = "background-subagents.json"

// BackgroundSubagentsPolicy values.
const (
	BackgroundPolicyOn  = "on"
	BackgroundPolicyOff = "off"
)

// Capability values.
const (
	BackgroundCapabilityReady  = "ready"
	BackgroundCapabilityAbsent = "absent"
)

// BackgroundSubagentsPolicy type.
type BackgroundSubagentsPolicy string

func (p BackgroundSubagentsPolicy) String() string { return string(p) }

// BackgroundSubagentsSource identifies who decided.
type BackgroundSubagentsSource string

const (
	BackgroundSourceProject     BackgroundSubagentsSource = "project_file"
	BackgroundSourceGlobal      BackgroundSubagentsSource = "global_file"
	BackgroundSourceEnvironment BackgroundSubagentsSource = "environment"
	BackgroundSourceDefault     BackgroundSubagentsSource = "default"
)

// BackgroundSubagentsResolution is the result of resolving policy.
type BackgroundSubagentsResolution struct {
	Policy            BackgroundSubagentsPolicy `json:"policy"`
	Source            BackgroundSubagentsSource `json:"source"`
	Malformed         bool                      `json:"malformed"`
	ProjectFile       string                    `json:"projectFile"`
	GlobalFile        string                    `json:"globalFile"`
	ProjectFileExists bool                      `json:"projectFileExists"`
	GlobalFileExists  bool                      `json:"globalFileExists"`
	EnvValue          *string                   `json:"envValue"`
}

// LoadBackgroundSubagentsOptions controls resolution.
type LoadBackgroundSubagentsOptions struct {
	GentleAiConfigHome string
	Env                map[string]string
}

// BackgroundSubagentsReport is the human-readable rendering.
type BackgroundSubagentsReport struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// parseBackgroundSubagentsPolicyFile strict decodes — exactly 2 keys.
func parseBackgroundSubagentsPolicyFile(raw string) (BackgroundSubagentsPolicy, bool) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", false
	}
	if len(parsed) != 2 {
		return "", false
	}
	schema, ok := parsed["schema"].(string)
	if !ok || schema != BackgroundSubagentsSchema {
		return "", false
	}
	policy, ok := parsed["policy"].(string)
	if !ok || (policy != BackgroundPolicyOn && policy != BackgroundPolicyOff) {
		return "", false
	}
	return BackgroundSubagentsPolicy(policy), true
}

// ParseBackgroundSubagentsPolicyFile exported for tests.
func ParseBackgroundSubagentsPolicyFile(raw string) (BackgroundSubagentsPolicy, bool) {
	return parseBackgroundSubagentsPolicyFile(raw)
}

// biggzConfigHome resolves the biggz config home: BIGGZ_CONFIG_HOME > GENTLE_PI_CONFIG_HOME > ~/.biggz
func biggzConfigHome() string {
	if v := strings.TrimSpace(os.Getenv("BIGGZ_CONFIG_HOME")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("GENTLE_PI_CONFIG_HOME")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("GENTLE_AI_CONFIG_HOME")); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".biggz")
}

// BiggzConfigHome exported.
func BiggzConfigHome() string { return biggzConfigHome() }

func lookupBackgroundEnv(env map[string]string) (string, bool) {
	if env != nil {
		if v, ok := env["BIGGZ_BACKGROUND_SUBAGENTS"]; ok {
			return v, true
		}
		if v, ok := env["GENTLE_PI_BACKGROUND_SUBAGENTS"]; ok {
			return v, true
		}
		return "", false
	}
	if v, ok := os.LookupEnv("BIGGZ_BACKGROUND_SUBAGENTS"); ok {
		return v, true
	}
	if v, ok := os.LookupEnv("GENTLE_PI_BACKGROUND_SUBAGENTS"); ok {
		return v, true
	}
	return "", false
}

func describeBackgroundSubagentsSource(r BackgroundSubagentsResolution) string {
	switch r.Source {
	case BackgroundSourceProject:
		return fmt.Sprintf("project file %s", r.ProjectFile)
	case BackgroundSourceGlobal:
		return fmt.Sprintf("global file %s", r.GlobalFile)
	case BackgroundSourceEnvironment:
		if r.EnvValue != nil {
			// BIGGZ takes precedence label when present; fallback to legacy name for reporting
			return "BIGGZ_BACKGROUND_SUBAGENTS"
		}
		return "BIGGZ_BACKGROUND_SUBAGENTS"
	default:
		return "built-in default"
	}
}

// ResolveBackgroundSubagentsPolicy implements 4-source first-hit-wins, max 2 reads, malformed fails closed.
func ResolveBackgroundSubagentsPolicy(cwd string, opts LoadBackgroundSubagentsOptions) BackgroundSubagentsResolution {
	configHome := opts.GentleAiConfigHome
	if configHome == "" {
		configHome = biggzConfigHome()
	}
	projectFile := filepath.Join(cwd, ".biggz", BackgroundSubagentsFile)
	legacyProjectFile := filepath.Join(cwd, ".pi", "gentle-ai", BackgroundSubagentsFile)
	globalFile := filepath.Join(configHome, BackgroundSubagentsFile)
	if _, err := os.Stat(projectFile); os.IsNotExist(err) {
		if _, err2 := os.Stat(legacyProjectFile); err2 == nil {
			projectFile = legacyProjectFile
		}
	}
	projectExists := false
	globalExists := false
	if _, err := os.Stat(projectFile); err == nil {
		projectExists = true
	}
	if _, err := os.Stat(globalFile); err == nil {
		globalExists = true
	}
	envVal, envSet := lookupBackgroundEnv(opts.Env)
	var envPtr *string
	if envSet {
		envPtr = &envVal
	}
	locations := BackgroundSubagentsResolution{
		ProjectFile:       projectFile,
		GlobalFile:        globalFile,
		ProjectFileExists: projectExists,
		GlobalFileExists:  globalExists,
		EnvValue:          envPtr,
	}
	for _, entry := range []struct {
		source BackgroundSubagentsSource
		path   string
		exists bool
	}{
		{BackgroundSourceProject, projectFile, projectExists},
		{BackgroundSourceGlobal, globalFile, globalExists},
	} {
		if !entry.exists {
			continue
		}
		raw, err := os.ReadFile(entry.path)
		if err != nil {
			locations.Policy = BackgroundPolicyOff
			locations.Source = entry.source
			locations.Malformed = true
			return locations
		}
		if decoded, ok := parseBackgroundSubagentsPolicyFile(string(raw)); ok {
			locations.Policy = decoded
			locations.Source = entry.source
			locations.Malformed = false
			return locations
		}
		locations.Policy = BackgroundPolicyOff
		locations.Source = entry.source
		locations.Malformed = true
		return locations
	}
	if envSet && (envVal == BackgroundPolicyOn || envVal == BackgroundPolicyOff) {
		locations.Policy = BackgroundSubagentsPolicy(envVal)
		locations.Source = BackgroundSourceEnvironment
		locations.Malformed = false
		return locations
	}
	locations.Policy = BackgroundPolicyOff
	locations.Source = BackgroundSourceDefault
	locations.Malformed = false
	return locations
}

// LoadBackgroundSubagentsPolicy delegate.
func LoadBackgroundSubagentsPolicy(cwd string) string {
	return ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{}).Policy.String()
}

// RenderBackgroundSubagentsReport renders the human status.
func RenderBackgroundSubagentsReport(r BackgroundSubagentsResolution, capability string, wrote *BackgroundSubagentsPolicy) BackgroundSubagentsReport {
	lines := []string{fmt.Sprintf("background subagents: %s (decided by %s; capability: %s)", r.Policy, describeBackgroundSubagentsSource(r), capability)}
	if wrote != nil {
		lines = append(lines, fmt.Sprintf("Wrote %s to the global file %s.", *wrote, r.GlobalFile))
	}
	if r.Malformed {
		path := r.GlobalFile
		if r.Source == BackgroundSourceProject {
			path = r.ProjectFile
		}
		lines = append(lines, fmt.Sprintf("%s is present but malformed, so the policy fails closed to off and no lower-priority source is consulted.", path))
	}
	outranks := wrote != nil && r.Source == BackgroundSourceProject
	if outranks {
		lines = append(lines, fmt.Sprintf("That global write does not take effect here: the project file %s outranks it. Edit or remove that project file to let the global setting decide.", r.ProjectFile))
	} else if wrote == nil && r.Source == BackgroundSourceProject && r.GlobalFileExists {
		lines = append(lines, fmt.Sprintf("The global file %s exists but is outranked by that project file.", r.GlobalFile))
	}
	if r.EnvValue != nil && r.Source != BackgroundSourceEnvironment {
		ev := *r.EnvValue
		if ev == BackgroundPolicyOn || ev == BackgroundPolicyOff {
			lines = append(lines, fmt.Sprintf("BIGGZ_BACKGROUND_SUBAGENTS=%s is set, but both files outrank it and it outranks the built-in default; it decides only when neither file exists.", ev))
		} else {
			lines = append(lines, fmt.Sprintf("BIGGZ_BACKGROUND_SUBAGENTS=\"%s\" is not a recognized value (\"on\" or \"off\"), so it is ignored.", ev))
		}
	}
	lines = append(lines, "Resolution order (first hit wins): project file, global file, BIGGZ_BACKGROUND_SUBAGENTS, built-in default off.")
	tp := "info"
	if r.Malformed || outranks {
		tp = "warning"
	}
	return BackgroundSubagentsReport{Message: strings.Join(lines, "\n"), Type: tp}
}

// ResolveBackgroundSubagentsCapability probes for subagent_run.
func ResolveBackgroundSubagentsCapability(homeDir string) string {
	// probe via pi-subagents package presence as fallback; prefer subagent_run tool if available
	candidates := []string{
		filepath.Join(homeDir, ".pi", "agent", "npm", "node_modules", "pi-subagents"),
		filepath.Join(homeDir, ".pi", "agent", "node_modules", "pi-subagents"),
		filepath.Join(homeDir, ".biggz", "subagents"),
	}
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		candidates = append(candidates,
			filepath.Join(v, "npm", "node_modules", "pi-subagents"),
			filepath.Join(v, "node_modules", "pi-subagents"),
		)
	}
	// Also check BIGGZ home subagents
	if v := strings.TrimSpace(os.Getenv("BIGGZ_CONFIG_HOME")); v != "" {
		candidates = append(candidates, filepath.Join(v, "subagents"))
	}
	for _, root := range candidates {
		if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
			return BackgroundCapabilityReady
		}
	}
	return BackgroundCapabilityAbsent
}

// RenderBackgroundSubagentsStatusLine convenience.
func RenderBackgroundSubagentsStatusLine(homeDir string) string {
	res := ResolveBackgroundSubagentsPolicy(homeDir, LoadBackgroundSubagentsOptions{})
	capability := ResolveBackgroundSubagentsCapability(homeDir)
	report := RenderBackgroundSubagentsReport(res, capability, nil)
	return report.Message
}
