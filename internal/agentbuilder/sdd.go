package agentbuilder

import (
	"fmt"
	"os"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/codegraph"
	"github.com/biggs-100/biggz-ai/internal/install"
)

// InjectSDDReference appends (or replaces) a custom-agent reference block in the
// system prompt file at systemPromptPath.
//
// For SDDPhaseSupport mode the block declares that the skill supports an existing
// phase. For SDDNewPhase mode the block integrates it as a first-class new phase
// (mode deferred in biggz, see types.go note).
//
// The function is a no-op when agent.SDDConfig is nil or the mode is SDDStandalone.
//
// It reuses install.InjectByMarker — the same HTML-comment marker mechanism the
// install pipeline uses for the persona and BigMem protocol blocks — with
// `biggz:custom-agent:<name>` markers, so repeated runs replace the block
// idempotently instead of duplicating it.
func InjectSDDReference(agent *GeneratedAgent, systemPromptPath string) error {
	if agent == nil || agent.SDDConfig == nil || agent.SDDConfig.Mode == SDDStandalone {
		return nil
	}

	data, err := os.ReadFile(systemPromptPath)
	if err != nil {
		return fmt.Errorf("sdd inject: read %s: %w", systemPromptPath, err)
	}

	block := buildSDDBlock(agent)
	updated := install.InjectByMarker(string(data), block, "biggz:custom-agent:"+agent.Name)

	if updated == string(data) {
		return nil
	}

	if err := os.WriteFile(systemPromptPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("sdd inject: write %s: %w", systemPromptPath, err)
	}

	return nil
}

// AdvisoryHint loads the CodeGraph report for the change (if present) and returns an advisory hint string.
// It is advisory only: nil report returns empty string without error or blocking.
// The hint surfaces files with reasons and MUST NOT auto-mutate tasks or block SDD when absent.
func AdvisoryHint(change, cwd string) string {
	report, err := codegraph.LoadHint(change, cwd)
	if err != nil || report == nil || len(report.Files) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("CodeGraph advisory hint for change ")
	b.WriteString(change)
	b.WriteString(":\n")
	for _, f := range report.Files {
		b.WriteString("- ")
		b.WriteString(f.Path)
		b.WriteString(" (")
		for i, r := range f.Reasons {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(string(r))
		}
		b.WriteString(")\n")
	}
	if len(report.Graph.Nodes) > 0 || len(report.Graph.Edges) > 0 {
		b.WriteString(fmt.Sprintf("Graph: %d nodes, %d edges\n", len(report.Graph.Nodes), len(report.Graph.Edges)))
	}
	return b.String()
}

// FormatAdvisoryHint formats a report into an advisory string (nil-safe).
func FormatAdvisoryHint(r *codegraph.Report) string {
	if r == nil || len(r.Files) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range r.Files {
		b.WriteString(f.Path)
		b.WriteString(":")
		for i, rs := range f.Reasons {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(string(rs))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// buildSDDBlock returns the marker-body for the agent (the markers themselves
// are owned by install.InjectByMarker).
func buildSDDBlock(agent *GeneratedAgent) string {
	cfg := agent.SDDConfig

	switch cfg.Mode {
	case SDDPhaseSupport:
		return fmt.Sprintf(
			"## Custom Agent: %s (Phase Support)\n\n"+
				"This skill provides additional support for the `sdd-%s` phase.\n"+
				"When working on tasks related to `%s`, load the `%s` skill for enhanced guidance.\n\n"+
				"Trigger phrases: %s\n",
			agent.Title,
			cfg.TargetPhase,
			cfg.TargetPhase,
			agent.Name,
			agent.Trigger,
		)
	case SDDNewPhase:
		// Deferred mode in biggz: kept for type parity with gentle-ai.
		phaseName := cfg.PhaseName
		if phaseName == "" {
			phaseName = agent.Name
		}
		return fmt.Sprintf(
			"## Custom Agent: %s (New SDD Phase)\n\n"+
				"This skill adds a new phase `%s` to the SDD dependency graph.\n"+
				"Load the `%s` skill when the orchestrator launches you for the `%s` phase.\n\n"+
				"Trigger phrases: %s\n",
			agent.Title,
			phaseName,
			agent.Name,
			phaseName,
			agent.Trigger,
		)
	default:
		return fmt.Sprintf("## Custom Agent: %s\n\nTrigger: %s\n", agent.Title, agent.Trigger)
	}
}
