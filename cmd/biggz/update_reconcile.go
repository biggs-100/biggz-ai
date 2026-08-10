package main

import (
	"context"
	"fmt"

	"github.com/biggs-100/biggz-ai/internal/update"
	"github.com/biggs-100/biggz-ai/plugin"
)

// postUpdateReconcile re-deploys managed assets for the detected agent after
// a successful binary replacement. It never fails the update: problems are
// reported as a warning with a manual fallback. Returns the report text to
// print.
func postUpdateReconcile(ctx context.Context, adapters map[string]plugin.AgentAdapter, homeDir string, noReconcile bool) string {
	if noReconcile {
		return "Reconcile skipped (--no-reconcile)"
	}

	var adapter plugin.AgentAdapter
	for _, name := range priorityAgents() {
		a := adapters[name]
		if a == nil {
			continue
		}
		if ok, _, _, _, _ := a.Detect(ctx, homeDir); ok {
			adapter = a
			break
		}
	}
	if adapter == nil {
		// No detected agent: pick any registered adapter so path-based
		// reconciliation can still run (mirrors sync's fallback).
		for name := range adapters {
			adapter = adapters[name]
			if adapter != nil {
				_ = name
				break
			}
		}
	}
	if adapter == nil {
		return "warning: reconcile failed: no agent adapters registered"
	}

	rr, err := update.Reconcile(ctx, adapter, homeDir)
	if err != nil {
		return fmt.Sprintf("warning: reconcile failed: %v — run 'biggz sync --all' to redeploy managed assets", err)
	}
	cfg := "config merged"
	if !rr.ConfigMerged {
		cfg = "config unchanged"
	}
	mcp := "MCP deployed"
	if !rr.MCPDeployed {
		mcp = "MCP not deployed"
	}
	return fmt.Sprintf("Reconciled: skills %d, commands %d, plugins %d, prompts %d, %s, %s",
		rr.Skills, rr.Commands, rr.Plugins, rr.Prompts, cfg, mcp)
}
