package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var deniedBashPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+-rf\s+(?:/(?:\s|$)|~(?:/|\s|$)|[$]HOME(?:/|\s|$)|\.\.?(?:\s|$))`),
	regexp.MustCompile(`\bgit\s+reset\s+--hard\b`),
	regexp.MustCompile(`\bgit\s+clean\b`),
	regexp.MustCompile(`\bgit(?:\s+--?\S+(?:\s+[^-\s]\S*)?)*\s+push\b`),
	regexp.MustCompile(`\bchmod\s+-R\s+777\b`),
	regexp.MustCompile(`\bchown\s+-R\b`),
}

const (
	GuardGitPush           = "gitPush"
	GuardGitRebase         = "gitRebase"
	GuardBranchDeleteForce = "gitBranchDeleteForce"
	GuardNpmPublish        = "npmPublish"
	GuardPiRemove          = "piRemove"
)

var guardedKeyPatterns = map[string]*regexp.Regexp{
	GuardGitPush:           regexp.MustCompile(`\bgit(?:\s+--?\S+(?:\s+[^-\s]\S*)?)*\s+push\b`),
	GuardGitRebase:         regexp.MustCompile(`\bgit\s+rebase\b`),
	GuardBranchDeleteForce: regexp.MustCompile(`\bgit\s+branch\s+(?:-[a-zA-Z]*D[a-zA-Z]*|-[a-zA-Z]*d[a-zA-Z]*f[a-zA-Z]*|-[a-zA-Z]*f[a-zA-Z]*d[a-zA-Z]*|--delete\b[^\n]*--force\b|--force\b[^\n]*--delete\b)`),
	GuardNpmPublish:        regexp.MustCompile(`\bnpm\s+publish\b`),
	GuardPiRemove:          regexp.MustCompile(`\bpi\s+remove\b`),
}

var autonomousDefaultActions = map[string]string{
	GuardGitPush:           "allow",
	GuardGitRebase:         "confirm",
	GuardBranchDeleteForce: "confirm",
	GuardNpmPublish:        "block",
	GuardPiRemove:          "confirm",
}

type RuntimeGuardrailsConfig struct {
	AutonomousMode  bool              `json:"autonomousMode"`
	GuardedCommands map[string]string `json:"guardedCommands"`
}

var safeGuardrailsConfig = RuntimeGuardrailsConfig{
	AutonomousMode:  false,
	GuardedCommands: map[string]string{},
}

func IsDenied(command string) bool {
	for i, p := range deniedBashPatterns {
		if !p.MatchString(command) {
			continue
		}
		// Special handling for patterns that were lookahead-based in TS
		if i == 2 { // git clean
			if !(regexp.MustCompile(`\s-[^\s]*f`).MatchString(command) || strings.Contains(command, "--force")) {
				continue
			}
			if !(regexp.MustCompile(`\s-[^\s]*d`).MatchString(command) || strings.Contains(command, "--directories")) {
				continue
			}
		}
		if i == 3 { // git push
			if !(strings.Contains(command, "--force") || regexp.MustCompile(`\s-[^\s-]*f`).MatchString(command)) {
				continue
			}
		}
		return true
	}
	return false
}

func ClassifyGuardedCommand(command string, cfg RuntimeGuardrailsConfig) string {
	if IsDenied(command) {
		return "block"
	}
	for key, pat := range guardedKeyPatterns {
		if !pat.MatchString(command) {
			continue
		}
		if !cfg.AutonomousMode {
			return "confirm"
		}
		if act, ok := cfg.GuardedCommands[key]; ok {
			return act
		}
		return autonomousDefaultActions[key]
	}
	return "not-guarded"
}

func ParseGuardrailsConfigFile(raw string) (*RuntimeGuardrailsConfig, bool) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	autonomousMode := parsed["autonomousMode"] == true
	rawCommands, _ := parsed["guardedCommands"].(map[string]any)
	guarded := map[string]string{}
	validActions := map[string]bool{"allow": true, "confirm": true, "block": true}
	validKeys := map[string]bool{GuardGitPush: true, GuardGitRebase: true, GuardBranchDeleteForce: true, GuardNpmPublish: true, GuardPiRemove: true}
	for k, v := range rawCommands {
		s, ok := v.(string)
		if !ok || !validActions[s] || !validKeys[k] {
			continue
		}
		guarded[k] = s
	}
	return &RuntimeGuardrailsConfig{AutonomousMode: autonomousMode, GuardedCommands: guarded}, true
}

func LoadRuntimeGuardrailsConfig(cwd string, configHome ...string) RuntimeGuardrailsConfig {
	if os.Getenv("GENTLE_PI_AUTONOMOUS_MODE") == "1" {
		return RuntimeGuardrailsConfig{AutonomousMode: true, GuardedCommands: map[string]string{}}
	}
	home := ""
	if len(configHome) > 0 && configHome[0] != "" {
		home = configHome[0]
	} else {
		home = gentlePiConfigHome()
	}
	globalPath := filepath.Join(home, "runtime-guardrails.json")
	projectPath := filepath.Join(cwd, ".pi", "gentle-ai", "runtime-guardrails.json")

	merged := RuntimeGuardrailsConfig{AutonomousMode: false, GuardedCommands: map[string]string{}}
	hasGlobal := false
	if data, err := os.ReadFile(globalPath); err == nil {
		if cfg, ok := ParseGuardrailsConfigFile(string(data)); ok {
			merged = *cfg
			hasGlobal = true
		} else {
			return safeGuardrailsConfig
		}
	}
	if data, err := os.ReadFile(projectPath); err == nil {
		if cfg, ok := ParseGuardrailsConfigFile(string(data)); ok {
			if hasGlobal {
				// copy-on-merge: shallow-copy global map so global not mutated
				newMap := make(map[string]string, len(merged.GuardedCommands)+len(cfg.GuardedCommands))
				for k, v := range merged.GuardedCommands {
					newMap[k] = v
				}
				for k, v := range cfg.GuardedCommands {
					newMap[k] = v
				}
				merged.GuardedCommands = newMap
				merged.AutonomousMode = cfg.AutonomousMode
			} else {
				merged = *cfg
			}
		} else {
			return safeGuardrailsConfig
		}
	}
	if merged.GuardedCommands == nil {
		merged.GuardedCommands = map[string]string{}
	}
	return merged
}

func gentlePiConfigHome() string {
	if v := os.Getenv("GENTLE_PI_CONFIG_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pi", "gentle-ai")
}

var pathGuardedTools = map[string]bool{"read": true, "write": true, "edit": true}
var pathInputKeys = map[string]bool{"path": true, "paths": true, "file": true, "files": true, "filePath": true, "filePaths": true}

var sensitivePathPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(^|/)\.ssh(?:/|$)`),
	regexp.MustCompile(`(^|/)\.credentials(?:/|$)`),
	regexp.MustCompile(`(^|/)library/keychains(?:/|$)`),
	regexp.MustCompile(`(^|/)\.aws/credentials$`),
	regexp.MustCompile(`(^|/)\.config/gh/hosts\.ya?ml$`),
	regexp.MustCompile(`(^|/)secrets(?:/|$)`),
	regexp.MustCompile(`(^|/)\.env(?:$|[./_-])`),
	regexp.MustCompile(`\.(?:pem|key|p12|pfx)$`),
}

func isSensitivePath(value string) bool {
	norm := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	norm = strings.ToLower(norm)
	norm = strings.ReplaceAll(norm, "~", os.Getenv("HOME"))
	for _, p := range sensitivePathPatterns {
		if p.MatchString(norm) {
			return true
		}
	}
	return false
}

func collectPathInputs(value any, key string) []string {
	switch v := value.(type) {
	case string:
		if pathInputKeys[key] {
			return []string{v}
		}
		return nil
	case []any:
		var out []string
		for _, item := range v {
			out = append(out, collectPathInputs(item, key)...)
		}
		return out
	case map[string]any:
		var out []string
		for k, val := range v {
			out = append(out, collectPathInputs(val, k)...)
		}
		return out
	default:
		return nil
	}
}

func EvaluateSensitivePathTool(toolName string, input any) *ToolCallDecision {
	if !pathGuardedTools[toolName] {
		return nil
	}
	paths := collectPathInputs(input, "")
	// Also try direct map extraction
	if m, ok := input.(map[string]any); ok {
		for k, v := range m {
			if pathInputKeys[k] {
				if s, ok := v.(string); ok {
					paths = append(paths, s)
				}
				if arr, ok := v.([]any); ok {
					for _, item := range arr {
						if s, ok := item.(string); ok {
							paths = append(paths, s)
						}
					}
				}
				if arr, ok := v.([]string); ok {
					paths = append(paths, arr...)
				}
			}
		}
	}
	for _, p := range paths {
		if isSensitivePath(p) {
			return &ToolCallDecision{Kind: DecisionBlock, Reason: "Gentle AI safety policy blocked access to sensitive path: " + p}
		}
	}
	return nil
}
