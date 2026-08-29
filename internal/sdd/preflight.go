package sdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type PreflightPrefs struct {
	ExecutionMode      string `json:"executionMode"`
	ArtifactStore      string `json:"artifactStore"`
	ChainedPrStrategy  string `json:"chainedPrStrategy"`
	ReviewBudgetLines  int    `json:"reviewBudgetLines"`
}

var preflightCache = map[string]PreflightPrefs{}

func SddPreflightDiskPath(home ...string) string {
	base := ""
	if len(home) > 0 && home[0] != "" {
		base = home[0]
	} else {
		if v := os.Getenv("GENTLE_PI_CONFIG_HOME"); v != "" {
			base = v
		} else {
			hd, _ := os.UserHomeDir()
			base = filepath.Join(hd, ".pi", "gentle-ai")
		}
	}
	return filepath.Join(base, "sdd-preflight.json")
}

func NormalizePreflightArtifactStore(s string) string {
	low := strings.ToLower(strings.TrimSpace(s))
	switch low {
	case "both", "hybrid", "engram", "bigmem":
		return "hybrid"
	case "openspec":
		return "openspec"
	case "none":
		return ""
	default:
		return low
	}
}

func canonicalizePrefs(p PreflightPrefs) PreflightPrefs {
	p.ArtifactStore = NormalizePreflightArtifactStore(p.ArtifactStore)
	if p.ExecutionMode == "" {
		p.ExecutionMode = "interactive"
	}
	if p.ChainedPrStrategy == "" {
		p.ChainedPrStrategy = "stacked-to-main"
	}
	if p.ReviewBudgetLines == 0 {
		p.ReviewBudgetLines = 400
	}
	return p
}

func WriteSddPreflightToDisk(p PreflightPrefs, home ...string) error {
	p = canonicalizePrefs(p)
	path := SddPreflightDiskPath(home...)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func ReadSddPreflightToDisk(home ...string) (PreflightPrefs, bool) {
	path := SddPreflightDiskPath(home...)
	data, err := os.ReadFile(path)
	if err != nil {
		return PreflightPrefs{}, false
	}
	var p PreflightPrefs
	if err := json.Unmarshal(data, &p); err != nil {
		return PreflightPrefs{}, false
	}
	p = canonicalizePrefs(p)
	return p, true
}

func SetPreflightPrefs(cwd string, p PreflightPrefs) {
	preflightCache[cwd] = canonicalizePrefs(p)
}

func GetPreflightPrefs(cwd string) (PreflightPrefs, bool) {
	p, ok := preflightCache[cwd]
	return p, ok
}

func ClearPreflightPrefs(cwd string) {
	delete(preflightCache, cwd)
}

func ResolvePreflightPrefs(cwd string, home ...string) PreflightPrefs {
	if p, ok := GetPreflightPrefs(cwd); ok {
		return p
	}
	if p, ok := ReadSddPreflightToDisk(home...); ok {
		return p
	}
	return PreflightPrefs{ExecutionMode: "interactive", ArtifactStore: "openspec", ChainedPrStrategy: "stacked-to-main", ReviewBudgetLines: 400}
}

type PreflightQuestionEnvelope struct {
	Pace     string `json:"pace"`
	Artifacts string `json:"artifacts"`
	PRs      string `json:"prs"`
	Review   string `json:"review"`
}

func ValidatePreflightQuestionEnvelope(env PreflightQuestionEnvelope) bool {
	if env.Pace != "interactive" && env.Pace != "auto" {
		return false
	}
	if env.Artifacts != "openspec" && env.Artifacts != "BigMem" && env.Artifacts != "both" && env.Artifacts != "hybrid" {
		return false
	}
	if env.PRs != "ask-on-risk" && env.PRs != "single-pr" && env.PRs != "auto-chain" {
		return false
	}
	if env.Review != "400" && env.Review != "800" && env.Review != "Other" {
		return false
	}
	return true
}

func SessionRecallMarkdown(project string, contextObservations int, sddObservations int, sessionObservations int) string {
	return "## Session Recall\n**Context Loaded:** " + preflightItoa(contextObservations) + " observations, " + preflightItoa(sessionObservations) + " sessions\n**Project:** " + project + "\n**Recent Summaries:** none\n**Fallback Used:** no\n"
}

func preflightItoa(n int) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(jsonNumber(n), "\"", ""), " ", ""), "\n", ""))
}

func jsonNumber(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

type PreflightSequence struct {
	RecallMarkdown string
	Envelope       PreflightQuestionEnvelope
	Prefs          PreflightPrefs
}
