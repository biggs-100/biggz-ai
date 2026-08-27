// Command biggz-mcp provides an MCP server exposing all Engram protocol tools.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
)

var store *bigmem.Store
var toolPrefix string

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
		if strings.HasPrefix(arg, "--prefix=") {
			toolPrefix = strings.TrimPrefix(arg, "--prefix=")
		}
		if strings.HasPrefix(arg, "--tools=") {
			tools = strings.TrimPrefix(arg, "--tools=")
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
	if toolPrefix != "" && strings.HasPrefix(name, toolPrefix+"_") {
		name = strings.TrimPrefix(name, toolPrefix+"_")
	}
	switch name {
	case "mem_save":
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
		// BlobStore externalization: len>100000 OR data:image/ → PutBlob → addr
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
		textResult(id, fmt.Sprintf("Saved: %s (id: %s)", obs.Title, obs.ID))

	case "mem_search":
		results, err := store.Search(getStr(args, "query"), bigmem.SearchOptions{
			Project:   getStr(args, "project"),
			Type:      getStr(args, "type"),
			Scope:     getStr(args, "scope"),
			Limit:     getInt(args, "limit", 10),
			MatchMode: getStr(args, "match_mode"),
		})
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
			entries = append(entries, entry)
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
		// Transparent blob resolve fallback (Store.Get already resolves)
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
		textResult(id, strings.Join(parts, "\n"))

	case "mem_session_summary":
		sessionID := getStr(args, "session_id")
		summary := getStr(args, "content")
		if sessionID == "" || summary == "" {
			writeError(id, "session_id and content are required")
			return
		}
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
		content := getStr(args, "content")
		sessionID := getStr(args, "session_id")
		if content == "" {
			writeError(id, "content is required")
			return
		}
		p, err := store.SavePrompt(content, sessionID)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		textResult(id, fmt.Sprintf("Prompt saved: %s", p.ID))

	case "mem_current_project":
		cwd, err := os.Getwd()
		if err != nil {
			writeError(id, err.Error())
			return
		}
		jsonResult(id, map[string]any{"project": filepath.Base(cwd), "path": cwd})

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
		// judgmentID format: rel-obsA-obsB
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
		project := getStr(args, "project")
		if content == "" {
			writeError(id, "content required")
			return
		}
		obs, err := bigmem.CapturePassive(content, project)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		saved := 0
		for _, o := range obs {
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

func buildToolList(scope string) []map[string]any {
	tools := []map[string]any{
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
			"query":      map[string]any{"type": "string"},
			"project":    map[string]any{"type": "string"},
			"type":       map[string]any{"type": "string"},
			"scope":      map[string]any{"type": "string"},
			"limit":      map[string]any{"type": "number"},
			"match_mode": map[string]any{"type": "string", "description": "\"all\" (AND, default) or \"any\" (OR)"},
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
	return tools
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
