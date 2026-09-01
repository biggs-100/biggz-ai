// Command biggz-mcp provides an MCP server exposing all Engram protocol tools.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
	"github.com/biggs-100/biggz-ai/internal/project"
)

var store *bigmem.Store
var toolPrefix string

// writeQueue serializes write operations (buffered-1 chan parity with Engram's writeQueue).
var writeQueue = make(chan struct{}, 1)

// sessionActivity tracks tool calls and nudges (Engram parity SessionActivity).
var sessionActivity = NewSessionActivity(20)

// serverInstructions mirrors Engram's serverInstructions CORE vs DEFERRED (WithDeferLoading parity).
const serverInstructions = `BigMem provides persistent memory that survives across sessions and compactions.

CORE TOOLS (always available — use without extra loading):
  mem_save — save decisions, bugs, discoveries, conventions PROACTIVELY (do not wait to be asked)
  mem_search — find past work, decisions, or context from previous sessions
  mem_context — get recent session history (call at session start or after compaction)
  mem_session_summary — save end-of-session summary (MANDATORY before saying "done")
  mem_get_observation — get full untruncated content of a search result by ID
  mem_save_prompt — save user prompt for context
  mem_current_project — detect current project from cwd (recommended first call)

DEFERRED TOOLS (use --tools=admin or --tools=all when needed):
  mem_update, mem_delete, mem_pin, mem_unpin, mem_suggest_topic_key, mem_session_start, mem_session_end,
  mem_stats, mem_timeline, mem_compare, mem_judge, mem_capture_passive, mem_merge_projects, mem_review,
  bigmem_branch_create, bigmem_branch_list, bigmem_branch_get

PROACTIVE SAVE RULE: Call mem_save immediately after ANY decision, bug fix, discovery, or convention — not just when asked.`

// MCPConfig holds optional overrides (Engram parity: BM25Floor, Limit).
type MCPConfig struct {
	BM25Floor *float64
	Limit     *int
}

// SessionActivity tracks tool calls for nudge logic (parity with engram/internal/mcp SessionActivity).
type SessionActivity struct {
	mu        sync.Mutex
	counts    map[string]int
	threshold int
	startedAt time.Time
}

func NewSessionActivity(threshold int) *SessionActivity {
	if threshold <= 0 {
		threshold = 20
	}
	return &SessionActivity{
		counts:    make(map[string]int),
		threshold: threshold,
		startedAt: time.Now(),
	}
}

func (a *SessionActivity) RecordToolCall(sessionID string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if sessionID == "" {
		sessionID = "global"
	}
	a.counts[sessionID]++
}

func (a *SessionActivity) NudgeIfNeeded(sessionID string) string {
	if a == nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if sessionID == "" {
		sessionID = "global"
	}
	c := a.counts[sessionID]
	if c > 0 && c%a.threshold == 0 {
		return fmt.Sprintf("\n\n💡 Reminder: %d tool calls without mem_session_summary — consider persisting session work.", c)
	}
	return ""
}

func (a *SessionActivity) ActivityScore(sessionID string) string {
	if a == nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if sessionID == "" {
		sessionID = "global"
	}
	c := a.counts[sessionID]
	return fmt.Sprintf("Activity: %d tool calls (threshold %d)", c, a.threshold)
}

func currentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func defaultSessionID(proj string) string {
	if strings.TrimSpace(proj) == "" {
		return "manual-save"
	}
	return "manual-save-" + strings.TrimSpace(proj)
}

func resolveFallbackSessionID(s *bigmem.Store, proj string) string {
	if s != nil {
		if id, ok, err := s.MostRecentActiveSession(proj); err == nil && ok {
			return id
		}
	}
	return defaultSessionID(proj)
}

func ensureImplicitSessionWithCWD(s *bigmem.Store, sessionID, proj string) error {
	if s == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	return s.EnsureImplicitSession(sessionID, proj)
}

func resolveProject(provided string) string {
	trimmed := strings.TrimSpace(provided)
	if trimmed != "" {
		return project.NormalizeProjectName(trimmed)
	}
	cwd := currentWorkingDirectory()
	if cwd != "" {
		info, err := project.DetectProject(cwd)
		if err == nil && strings.TrimSpace(info.Project) != "" && info.Project != "unknown" {
			return project.NormalizeProjectName(info.Project)
		}
		// Fallback to basename even on ambiguous/error
		if strings.TrimSpace(info.Project) != "" && info.Project != "unknown" {
			return project.NormalizeProjectName(info.Project)
		}
		// If DetectProject failed with ambiguous, fallback to basename via NormalizeProjectName of cwd base
		if cwd != "" {
			base := filepath.Base(cwd)
			if base != "" && base != "." {
				n := project.NormalizeProjectName(base)
				if n != "" && n != "unknown" {
					return n
				}
			}
		}
	}
	return "biggz-ai"
}

// queuedWriteHandler serializes write operations via writeQueue (buffered-1).
func queuedWriteHandler(fn func()) {
	writeQueue <- struct{}{}
	defer func() { <-writeQueue }()
	fn()
}

// ─── Tool Profiles (Engram parity) ───────────────────────────────────────────
// agent: 15 tools agents actually use (per skill files)
// admin: 4 tools for TUI/CLI curation
// all: 22+3 = 25 (nil allowlist)

var ProfileAgent = map[string]bool{
	"mem_save":              true,
	"mem_search":            true,
	"mem_get_observation":   true,
	"mem_context":           true,
	"mem_session_summary":   true,
	"mem_session_start":     true,
	"mem_session_end":       true,
	"mem_save_prompt":       true,
	"mem_current_project":   true,
	"mem_suggest_topic_key": true,
	"mem_timeline":          true,
	"mem_stats":             true,
	"mem_pin":               true,
	"mem_unpin":             true,
	"mem_compare":           true,
}

var ProfileAdmin = map[string]bool{
	"mem_doctor":         true,
	"mem_update":         true,
	"mem_delete":         true,
	"mem_merge_projects": true,
}

var Profiles = map[string]map[string]bool{
	"agent": ProfileAgent,
	"admin": ProfileAdmin,
}

// ResolveTools takes a comma-separated string of profile names and/or individual
// tool names and returns the allowlist. Nil means register everything (all).
func ResolveTools(input string) map[string]bool {
	input = strings.TrimSpace(input)
	if input == "" || input == "all" {
		return nil
	}
	result := make(map[string]bool)
	for _, token := range strings.Split(input, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if token == "all" {
			return nil
		}
		if profile, ok := Profiles[token]; ok {
			for tool := range profile {
				result[tool] = true
			}
		} else {
			result[token] = true
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func shouldRegister(name string, allowlist map[string]bool) bool {
	if allowlist == nil {
		return true
	}
	return allowlist[name]
}

func init() {
	// Auto-detect: default to "biggz" prefix when engram is installed
	if _, err := os.Stat(os.ExpandEnv("$HOME/AppData/Local/engram/bin/engram.exe")); err == nil {
		toolPrefix = "biggz"
	}
	if _, err := os.Stat(os.ExpandEnv("$HOME/.local/bin/engram")); err == nil {
		toolPrefix = "biggz"
	}
}

func main() {
	var err error
	store, err = bigmem.Open("")
	if err != nil {
		log.Fatalf("open bigmem: %v", err)
	}

	tools := "agent"
	for _, arg := range os.Args[1:] {
		if after, ok := strings.CutPrefix(arg, "--prefix="); ok {
			toolPrefix = after
		}
		if after, ok := strings.CutPrefix(arg, "--tools="); ok {
			tools = after
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		var req map[string]any
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		method, _ := req["method"].(string)
		id := req["id"]

		switch method {
		case "initialize":
			writeJSON(map[string]any{
				"jsonrpc": "2.0", "id": id,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]string{"name": "biggz-ai", "version": "1.0.0"},
					"instructions":    serverInstructions,
				},
			})
		case "ping":
			writeJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{}})
		case "tools/list":
			toolList := buildToolList(tools)
			if toolPrefix != "" {
				for i, t := range toolList {
					if name, ok := t["name"].(string); ok {
						toolList[i]["name"] = toolPrefix + "_" + name
					}
				}
			}
			writeJSON(map[string]any{
				"jsonrpc": "2.0", "id": id,
				"result": map[string]any{"tools": toolList},
			})
		case "tools/call":
			params, _ := req["params"].(map[string]any)
			name, _ := params["name"].(string)
			args, _ := params["arguments"].(map[string]any)
			handleToolCall(id, name, args)
		case "notifications/initialized":
			// ignore
		default:
			writeJSON(map[string]any{
				"jsonrpc": "2.0", "id": id,
				"error": map[string]any{"code": -32601, "message": fmt.Sprintf("unknown: %s", method)},
			})
		}
	}
}

func handleToolCall(id any, name string, args map[string]any) {
	// Strip prefix if present (e.g. "biggz_mem_save" → "mem_save")
	if toolPrefix != "" {
		if after, ok := strings.CutPrefix(name, toolPrefix+"_"); ok {
			name = after
		}
	}
	// Activity tracking (record every tool call)
	sessForActivity := getStr(args, "session_id")
	if sessForActivity == "" {
		sessForActivity = getStr(args, "sessionId")
	}
	// For search/context without session_id, use a stable global bucket
	if sessForActivity == "" {
		sessForActivity = "global"
	}
	sessionActivity.RecordToolCall(sessForActivity)

	switch name {
	case "mem_save":
		// Serialize writes via queuedWriteHandler (buffered-1)
		queuedWriteHandler(func() {
			obs := &bigmem.Observation{
				Title:     getStr(args, "title"),
				Content:   getStr(args, "content"),
				Type:      getStr(args, "type"),
				TopicKey:  getStr(args, "topic_key"),
				Project:   getStr(args, "project"),
				Scope:     getStr(args, "scope"),
				SessionID: getStr(args, "session_id"),
				ToolName:  getStr(args, "tool_name"),
			}
			if obs.Title == "" {
				writeError(id, "title is required")
				return
			}
			resolvedProject := resolveProject(obs.Project)
			if strings.TrimSpace(obs.SessionID) == "" {
				obs.SessionID = resolveFallbackSessionID(store, resolvedProject)
			}
			_ = ensureImplicitSessionWithCWD(store, obs.SessionID, resolvedProject)
			obs.Project = resolvedProject
			origContent := obs.Content
			origWasExternalized := bigmem.ShouldExternalize(origContent)
			if bigmem.ShouldExternalize(obs.Content) {
				if addr, err := bigmem.PutBlob([]byte(obs.Content)); err == nil {
					obs.Content = addr
				} else {
					fmt.Fprintf(os.Stderr, "[bigmem] PutBlob failed: %v\n", err)
				}
			}
			if err := store.Save(obs); err != nil {
				writeError(id, err.Error())
				return
			}
			msg := fmt.Sprintf("Saved: %s (id: %s)", obs.Title, obs.ID)
			if !origWasExternalized && len(origContent) > 50000 {
				msg += fmt.Sprintf(" ⚠️ content truncated from %d to %d bytes", len(origContent), 50000)
			}
			if nudge := sessionActivity.NudgeIfNeeded(sessForActivity); nudge != "" {
				msg += nudge
			}
			textResult(id, msg)
		})
		return

	case "mem_search":
		query := getStr(args, "query")
		allProjects := getBool(args, "all_projects", false)
		// all_projects true means ignore project filter; scope personal also cross-project
		project := getStr(args, "project")
		if allProjects {
			project = ""
		}
		matchMode := getStr(args, "match_mode")
		limit := getInt(args, "limit", 10)
		// BM25Floor/Limit override via env (Engram parity MCPConfig)
		var bm25Floor *float64
		if envVal := strings.TrimSpace(os.Getenv("BIGMEM_BM25_FLOOR")); envVal != "" {
			if f, err := strconv.ParseFloat(envVal, 64); err == nil {
				bm25Floor = &f
			}
		}
		// Also allow BIGGZ_BM25_FLOOR alias
		if bm25Floor == nil {
			if envVal := strings.TrimSpace(os.Getenv("BIGGZ_BM25_FLOOR")); envVal != "" {
				if f, err := strconv.ParseFloat(envVal, 64); err == nil {
					bm25Floor = &f
				}
			}
		}
		opts := bigmem.SearchOptions{
			Project:     project,
			Type:        getStr(args, "type"),
			Scope:       getStr(args, "scope"),
			Limit:       limit,
			MatchMode:   matchMode,
			AllProjects: allProjects,
			BM25Floor:   bm25Floor,
		}
		results, err := store.Search(query, opts)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		entries := make([]map[string]any, 0, len(results))
		for _, r := range results {
			entry := map[string]any{
				"id": r.ID, "title": r.Title, "type": r.Type,
				"content":    truncate(r.Content, 300),
				"session_id": r.SessionID, "tool_name": r.ToolName,
				"topic_key": r.TopicKey, "project": r.Project,
				"scope": r.Scope, "revision_count": r.RevisionCount,
				"duplicate_count": r.DuplicateCount,
				"state":           r.State(),
				"created":         r.CreatedAt,
			}
			if r.ReviewAfter != nil {
				entry["review_after"] = *r.ReviewAfter
			}
			if strings.Contains(r.Content, "[truncated]") {
				entry["truncation_warning"] = "⚠️ content truncated from >50000 to 50000 bytes"
			}
			entries = append(entries, entry)
		}
		// Append activity nudge if needed
		if nudge := sessionActivity.NudgeIfNeeded(sessForActivity); nudge != "" && len(entries) > 0 {
			// Append nudge as extra entry hint; for now just log to stderr and keep JSON pure
			fmt.Fprintln(os.Stderr, nudge)
		}
		jsonResult(id, entries)

	case "mem_get_observation", "mem_get":
		obsID := getStr(args, "id")
		if obsID == "" {
			writeError(id, "id is required")
			return
		}
		obs, err := store.Get(obsID)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		if bigmem.IsBlobAddr(obs.Content) {
			if data, err := bigmem.GetBlob(obs.Content); err == nil {
				obs.Content = string(data)
			}
		}
		jsonResult(id, map[string]any{
			"id": obs.ID, "title": obs.Title, "type": obs.Type,
			"content": obs.Content, "session_id": obs.SessionID,
			"tool_name": obs.ToolName, "topic_key": obs.TopicKey,
			"project": obs.Project, "scope": obs.Scope,
			"revision_count":  obs.RevisionCount,
			"duplicate_count": obs.DuplicateCount,
			"last_seen_at":    obs.LastSeenAt,
			"review_after":    obs.ReviewAfter,
			"pinned":          obs.Pinned,
			"state":           obs.State(),
			"created":         obs.CreatedAt, "updated": obs.UpdatedAt,
		})

	case "mem_update":
		obsID := getStr(args, "id")
		if obsID == "" {
			writeError(id, "id is required")
			return
		}
		updates := map[string]any{}
		if v, ok := args["title"]; ok {
			updates["title"] = v
		}
		if v, ok := args["content"]; ok {
			updates["content"] = v
		}
		if v, ok := args["type"]; ok {
			updates["type"] = v
		}
		if v, ok := args["topic_key"]; ok {
			updates["topic_key"] = v
		}
		if v, ok := args["scope"]; ok {
			updates["scope"] = v
		}
		obs, err := store.Update(obsID, updates)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		textResult(id, fmt.Sprintf("Updated: %s", obs.ID))

	case "mem_delete":
		obsID := getStr(args, "id")
		if obsID == "" {
			writeError(id, "id is required")
			return
		}
		if err := store.Delete(obsID); err != nil {
			writeError(id, err.Error())
			return
		}
		textResult(id, "Deleted")

	case "mem_context":
		sessions, err := store.SessionContext(getInt(args, "limit", 5))
		if err != nil {
			writeError(id, err.Error())
			return
		}
		if len(sessions) == 0 {
			textResult(id, "No session history.")
			return
		}
		var parts []string
		for _, s := range sessions {
			line := fmt.Sprintf("Session %s: %s", s.ID, s.StartTime.Format("2006-01-02 15:04"))
			if !s.EndTime.IsZero() {
				line += fmt.Sprintf(" → %s", s.EndTime.Format("15:04"))
			}
			if s.Summary != "" {
				line += fmt.Sprintf(" — %s", s.Summary[:min(len(s.Summary), 150)])
			}
			parts = append(parts, line)
		}
		msg := strings.Join(parts, "\n")
		if nudge := sessionActivity.NudgeIfNeeded(sessForActivity); nudge != "" {
			msg += nudge
		}
		textResult(id, msg)

	case "mem_session_summary":
		sessionID := getStr(args, "session_id")
		summary := getStr(args, "content")
		if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(summary) == "" {
			writeError(id, "session_id and content are required")
			return
		}
		resolvedProj := resolveProject(getStr(args, "project"))
		_ = ensureImplicitSessionWithCWD(store, sessionID, resolvedProj)
		s, err := store.SessionEnd(sessionID, summary)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		textResult(id, fmt.Sprintf("Session %s ended", s.ID))

	case "mem_session_start":
		idStr := getStr(args, "id")
		project := getStr(args, "project")
		if idStr == "" {
			writeError(id, "id is required")
			return
		}
		s, err := store.SessionStart(idStr, project)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, map[string]any{"session_id": s.ID, "started": s.StartTime})

	case "mem_session_end":
		sessionID := getStr(args, "id")
		summary := getStr(args, "summary")
		if sessionID == "" {
			writeError(id, "id is required")
			return
		}
		s, err := store.SessionEnd(sessionID, summary)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		textResult(id, fmt.Sprintf("Session %s ended", s.ID))

	case "mem_save_prompt":
		queuedWriteHandler(func() {
			content := getStr(args, "content")
			sessionID := getStr(args, "session_id")
			if content == "" {
				writeError(id, "content is required")
				return
			}
			resolvedProj := resolveProject(getStr(args, "project"))
			if strings.TrimSpace(sessionID) == "" {
				sessionID = resolveFallbackSessionID(store, resolvedProj)
			}
			_ = ensureImplicitSessionWithCWD(store, sessionID, resolvedProj)
			origPrompt := content
			p, err := store.SavePrompt(content, sessionID)
			if err != nil {
				writeError(id, err.Error())
				return
			}
			msg := fmt.Sprintf("Prompt saved: %s", p.ID)
			if len(origPrompt) > 50000 {
				msg += fmt.Sprintf(" ⚠️ content truncated from %d to %d bytes", len(origPrompt), 50000)
			}
			if nudge := sessionActivity.NudgeIfNeeded(sessForActivity); nudge != "" {
				msg += nudge
			}
			textResult(id, msg)
		})
		return

	case "mem_current_project":
		cwd, err := os.Getwd()
		if err != nil {
			writeError(id, err.Error())
			return
		}
		info, detErr := project.DetectProject(cwd)
		if detErr != nil && errors.Is(detErr, project.ErrInvalidConfig) {
			writeError(id, detErr.Error())
			return
		}
		result := map[string]any{
			"project":            info.Project,
			"project_source":     info.Source,
			"project_path":       info.Path,
			"path":               info.Path,
			"cwd":                cwd,
			"available_projects": info.AvailableProjects,
		}
		if info.Warning != "" {
			result["warning"] = info.Warning
		}
		if detErr != nil && errors.Is(detErr, project.ErrAmbiguousProject) {
			result["error_hint"] = detErr.Error()
		}
		if result["project"] == "" && (detErr == nil || !errors.Is(detErr, project.ErrAmbiguousProject)) {
			cwdCopy := cwd
			result["project"] = project.NormalizeProjectName(filepath.Base(cwdCopy))
		}
		jsonResult(id, result)

	case "mem_suggest_topic_key":
		title := getStr(args, "title")
		content := getStr(args, "content")
		obsType := getStr(args, "type")
		key := bigmem.SuggestTopicKey(title, content, obsType)
		textResult(id, key)

	case "mem_timeline":
		entries, err := store.Timeline(bigmem.TimelineOptions{
			Limit: getInt(args, "limit", 20),
		})
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, entries)

	case "mem_stats":
		stats, err := store.Stats()
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, stats)

	case "mem_pin":
		obsID := getStr(args, "id")
		if obsID == "" {
			writeError(id, "id required")
			return
		}
		store.Pin(obsID)
		textResult(id, "Pinned")

	case "mem_unpin":
		obsID := getStr(args, "id")
		if obsID == "" {
			writeError(id, "id required")
			return
		}
		store.Unpin(obsID)
		textResult(id, "Unpinned")

	case "mem_doctor":
		result, err := store.Doctor()
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, result)

	case "mem_compare":
		a := getStr(args, "memory_id_a")
		b := getStr(args, "memory_id_b")
		if a == "" || b == "" {
			writeError(id, "memory_id_a and memory_id_b required")
			return
		}
		result, err := store.Compare(a, b)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, result)

	case "mem_judge":
		judgmentID := getStr(args, "judgment_id")
		relation := getStr(args, "relation")
		reason := getStr(args, "reason")
		confidence := getFloat(args, "confidence", 1.0)
		if judgmentID == "" || relation == "" {
			writeError(id, "judgment_id and relation required")
			return
		}
		parts := strings.SplitN(judgmentID, "-", 3)
		if len(parts) < 3 {
			writeError(id, "invalid judgment_id format")
			return
		}
		jr, err := store.SaveRelation(parts[1], parts[2], relation, reason, confidence)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, jr)

	case "mem_capture_passive":
		content := getStr(args, "content")
		projectArg := getStr(args, "project")
		sessionID := getStr(args, "session_id")
		if content == "" {
			writeError(id, "content required")
			return
		}
		resolvedProj := resolveProject(projectArg)
		if strings.TrimSpace(sessionID) == "" {
			sessionID = resolveFallbackSessionID(store, resolvedProj)
		}
		_ = ensureImplicitSessionWithCWD(store, sessionID, resolvedProj)
		obs, err := bigmem.CapturePassive(content, resolvedProj)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		saved := 0
		for _, o := range obs {
			o.SessionID = sessionID
			o.Project = resolvedProj
			if e := store.Save(o); e == nil {
				saved++
			}
		}
		textResult(id, fmt.Sprintf("Captured %d learnings", saved))

	case "mem_merge_projects":
		source := getStr(args, "source_project")
		target := getStr(args, "target_project")
		if source == "" || target == "" {
			writeError(id, "source_project and target_project required")
			return
		}
		count, err := store.MergeProjects(source, target)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		textResult(id, fmt.Sprintf("Merged %d observations from %s to %s", count, source, target))

	case "mem_review":
		action := getStr(args, "action")
		obsID := getStr(args, "observation_id")

		if action == "list" {
			ids, err := store.ListNeedsReview()
			if err != nil {
				writeError(id, err.Error())
				return
			}
			jsonResult(id, map[string]any{"need_review": ids})
		} else if action == "mark_reviewed" {
			if obsID == "" {
				writeError(id, "observation_id required")
				return
			}
			if err := store.Review("mark_reviewed", obsID); err != nil {
				writeError(id, err.Error())
				return
			}
			textResult(id, "Marked reviewed")
		} else {
			writeError(id, "unknown action, use 'list' or 'mark_reviewed'")
		}

	case "bigmem_branch_create":
		parentID := getStr(args, "parent_id")
		summary := getStr(args, "branch_summary")
		sess, err := store.CreateBranch(parentID, summary)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, sess)

	case "bigmem_branch_list":
		sessions, err := store.ListBranches()
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, sessions)

	case "bigmem_branch_get":
		sid := getStr(args, "id")
		if sid == "" {
			writeError(id, "id required")
			return
		}
		sess, err := store.GetBranch(sid)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, sess)

	default:
		writeError(id, fmt.Sprintf("unknown tool: %s", name))
	}
}

func buildToolList(profile string) []map[string]any {
	allowlist := ResolveTools(profile)
	all := []map[string]any{
		toolDef("mem_save", "Save an observation to persistent memory.", map[string]any{
			"title":      map[string]any{"type": "string", "description": "Short searchable title"},
			"content":    map[string]any{"type": "string"},
			"type":       map[string]any{"type": "string", "description": "decision|architecture|bugfix|discovery|config|preference"},
			"topic_key":  map[string]any{"type": "string"},
			"project":    map[string]any{"type": "string"},
			"scope":      map[string]any{"type": "string"},
			"session_id": map[string]any{"type": "string"},
			"tool_name":  map[string]any{"type": "string"},
			"pinned":     map[string]any{"type": "boolean"},
		}, []string{"title"}),
		toolDef("mem_search", "Search memory by keywords.", map[string]any{
			"query":       map[string]any{"type": "string"},
			"project":     map[string]any{"type": "string", "description": "Filter by project. Ignored when all_projects=true."},
			"type":        map[string]any{"type": "string"},
			"scope":       map[string]any{"type": "string"},
			"limit":       map[string]any{"type": "number", "description": "Max results (default: 10)"},
			"match_mode":  map[string]any{"type": "string", "description": "\"all\" (AND, default) or \"any\" (OR)"},
			"all_projects": map[string]any{"type": "boolean", "description": "When true, search across all projects (ignore project filter). Scope=personal also ignores project."},
		}, []string{}),
		toolDef("mem_get_observation", "Get full observation by ID.", map[string]any{
			"id": map[string]any{"type": "string"},
		}, []string{"id"}),
		toolDef("mem_update", "Update an existing observation.", map[string]any{
			"id": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"}, "type": map[string]any{"type": "string"},
			"topic_key": map[string]any{"type": "string"}, "scope": map[string]any{"type": "string"},
		}, []string{"id"}),
		toolDef("mem_delete", "Delete an observation.", map[string]any{
			"id": map[string]any{"type": "string"},
		}, []string{"id"}),
		toolDef("mem_context", "Get recent session history.", map[string]any{
			"limit": map[string]any{"type": "number"},
		}, nil),
		toolDef("mem_session_summary", "End a session with summary.", map[string]any{
			"session_id": map[string]any{"type": "string"},
			"content":    map[string]any{"type": "string"},
		}, []string{"session_id", "content"}),
		toolDef("mem_session_start", "Start a new session.", map[string]any{
			"id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"},
		}, []string{"id"}),
		toolDef("mem_session_end", "End a session.", map[string]any{
			"id": map[string]any{"type": "string"}, "summary": map[string]any{"type": "string"},
		}, []string{"id"}),
		toolDef("mem_save_prompt", "Save user prompt for context.", map[string]any{
			"content": map[string]any{"type": "string"}, "session_id": map[string]any{"type": "string"},
		}, []string{"content"}),
		toolDef("mem_current_project", "Detect current project from working directory.", nil, nil),
		toolDef("mem_suggest_topic_key", "Suggest a topic key from title/content.", map[string]any{
			"title": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"},
			"type": map[string]any{"type": "string"},
		}, nil),
		toolDef("mem_timeline", "Chronological listing of observations.", map[string]any{
			"limit": map[string]any{"type": "number"},
		}, nil),
		toolDef("mem_stats", "Usage statistics.", nil, nil),
		toolDef("mem_pin", "Pin an observation.", map[string]any{
			"id": map[string]any{"type": "string"},
		}, []string{"id"}),
		toolDef("mem_unpin", "Unpin an observation.", map[string]any{
			"id": map[string]any{"type": "string"},
		}, []string{"id"}),
		toolDef("mem_doctor", "Run store diagnostics.", nil, nil),
		toolDef("mem_compare", "Compare two observations.", map[string]any{
			"memory_id_a": map[string]any{"type": "string"},
			"memory_id_b": map[string]any{"type": "string"},
		}, []string{"memory_id_a", "memory_id_b"}),
		toolDef("mem_judge", "Record a judgment between memories.", map[string]any{
			"judgment_id": map[string]any{"type": "string"},
			"relation":    map[string]any{"type": "string", "description": "related|compatible|scoped|conflicts_with|supersedes|not_conflict"},
			"reason":      map[string]any{"type": "string"},
			"confidence":  map[string]any{"type": "number"},
		}, []string{"judgment_id", "relation"}),
		toolDef("mem_capture_passive", "Extract learnings from text.", map[string]any{
			"content": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"},
		}, []string{"content"}),
		toolDef("mem_merge_projects", "Move observations between projects.", map[string]any{
			"source_project": map[string]any{"type": "string"},
			"target_project": map[string]any{"type": "string"},
		}, []string{"source_project", "target_project"}),
		toolDef("mem_review", "Manage observation lifecycle review state.", map[string]any{
			"action":         map[string]any{"type": "string", "description": "list|mark_reviewed"},
			"observation_id": map[string]any{"type": "string", "description": "Observation ID (required for mark_reviewed)"},
		}, []string{"action"}),
		toolDef("bigmem_branch_create", "Create a branching session (internal-only).", map[string]any{
			"parent_id":      map[string]any{"type": "string", "description": "Parent session ID, empty for root"},
			"branch_summary": map[string]any{"type": "string", "description": "Optional branch summary"},
		}, nil),
		toolDef("bigmem_branch_list", "List branching sessions (internal-only).", nil, nil),
		toolDef("bigmem_branch_get", "Get branching session by ID (internal-only).", map[string]any{
			"id": map[string]any{"type": "string"},
		}, []string{"id"}),
	}
	if allowlist == nil {
		return all
	}
	var filtered []map[string]any
	for _, t := range all {
		if name, ok := t["name"].(string); ok && shouldRegister(name, allowlist) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func toolDef(name, desc string, props map[string]any, required []string) map[string]any {
	t := map[string]any{"name": name, "description": desc}
	req := required
	if req == nil {
		req = []string{}
	}
	if props != nil {
		t["inputSchema"] = map[string]any{"type": "object", "properties": props, "required": req}
	} else {
		t["inputSchema"] = map[string]any{"type": "object", "properties": map[string]any{}, "required": req}
	}
	return t
}

func getStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]any, key string, def int) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return def
}

func getFloat(m map[string]any, key string, def float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return def
}

func getBool(m map[string]any, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	if v, ok := m[key].(string); ok {
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "true" || s == "1" {
			return true
		}
		if s == "false" || s == "0" {
			return false
		}
	}
	if v, ok := m[key].(float64); ok {
		return v != 0
	}
	return def
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func writeJSON(v any) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
}

func writeError(id any, msg string) {
	writeJSON(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32603, "message": msg}})
}

func textResult(id any, text string) {
	writeJSON(map[string]any{
		"jsonrpc": "2.0", "id": id,
		"result": map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
		},
	})
}

func jsonResult(id any, v any) {
	writeJSON(map[string]any{
		"jsonrpc": "2.0", "id": id,
		"result": map[string]any{
			"content": []map[string]any{{"type": "json", "json": v}},
		},
	})
}
