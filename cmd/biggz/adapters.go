package main

import (
	"github.com/biggs-100/biggz-ai/internal/agents/antigravity"
	"github.com/biggs-100/biggz-ai/internal/agents/claude"
	"github.com/biggs-100/biggz-ai/internal/agents/codex"
	"github.com/biggs-100/biggz-ai/internal/agents/cursor"
	"github.com/biggs-100/biggz-ai/internal/agents/gemini"
	"github.com/biggs-100/biggz-ai/internal/agents/hermes"
	"github.com/biggs-100/biggz-ai/internal/agents/kilocode"
	"github.com/biggs-100/biggz-ai/internal/agents/kimi"
	"github.com/biggs-100/biggz-ai/internal/agents/kiro"
	"github.com/biggs-100/biggz-ai/internal/agents/openclaw"
	"github.com/biggs-100/biggz-ai/internal/agents/opencode"
	"github.com/biggs-100/biggz-ai/internal/agents/pi"
	"github.com/biggs-100/biggz-ai/internal/agents/qwen"
	"github.com/biggs-100/biggz-ai/internal/agents/trae"
	"github.com/biggs-100/biggz-ai/internal/agents/vscode"
	"github.com/biggs-100/biggz-ai/internal/agents/windsurf"
	"github.com/biggs-100/biggz-ai/plugin"
)

// agentAdapters returns the built-in agent adapter map shared by install,
// uninstall, sync and update.
func agentAdapters() map[string]plugin.AgentAdapter {
	return map[string]plugin.AgentAdapter{
		"opencode":    opencode.NewAdapter(),
		"qwen":        qwen.NewAdapter(),
		"claude":      claude.NewAdapter(),
		"cursor":      cursor.NewAdapter(),
		"windsurf":    windsurf.NewAdapter(),
		"gemini":      gemini.NewAdapter(),
		"codex":       codex.NewAdapter(),
		"pi":          pi.NewAdapter(),
		"vscode":      vscode.NewAdapter(),
		"kiro":        kiro.NewAdapter(),
		"antigravity": antigravity.NewAdapter(),
		"hermes":      hermes.NewAdapter(),
		"kimi":        kimi.NewAdapter(),
		"kilocode":    kilocode.NewAdapter(),
		"trae":        trae.NewAdapter(),
		"openclaw":    openclaw.NewAdapter(),
	}
}

// priorityAgents returns the adapter IDs tried in order when resolving "the
// detected agent" for install and post-update reconcile.
func priorityAgents() []string {
	return []string{"opencode", "claude", "qwen", "cursor", "windsurf", "gemini", "codex", "pi", "vscode", "kiro"}
}
