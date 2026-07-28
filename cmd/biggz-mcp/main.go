// Command biggz-mcp provides an MCP (Model Context Protocol) server for biggz-ai.
//
// It exposes the engram store as MCP tools so AI agents can call mem_save,
// mem_search, and mem_get directly — just like gentle-ai's engram MCP.
//
// Usage: biggz-mcp --tools=agent|all
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/biggz-ai/biggz/internal/engram"
)

var store *engram.Store

func main() {
	var err error
	store, err = engram.Open("")
	if err != nil {
		log.Fatalf("open engram: %v", err)
	}

	// Read tools flag
	tools := "agent"
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--tools=") {
			tools = strings.TrimPrefix(arg, "--tools=")
		}
	}

	// Send initialize response
	initResp := map[string]any{
		"jsonrpc": "2.0",
		"id":      "init",
		"result": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]string{
				"name":    "biggz-ai",
				"version": "1.0.0",
			},
		},
	}
	writeJSON(initResp)

	// Handle incoming requests
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
		case "ping":
			writeJSON(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result":  map[string]any{},
			})

		case "tools/list":
			toolsList := buildToolList(tools)
			writeJSON(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"tools": toolsList,
				},
			})

		case "tools/call":
			params, _ := req["params"].(map[string]any)
			name, _ := params["name"].(string)
			args, _ := params["arguments"].(map[string]any)
			handleToolCall(id, name, args)

		case "notifications/initialized":
			// Ignore

		default:
			writeJSON(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"error": map[string]any{
					"code":    -32601,
					"message": fmt.Sprintf("unknown method: %s", method),
				},
			})
		}
	}
}

func handleToolCall(id any, name string, args map[string]any) {
	switch name {
	case "mem_save":
		title, _ := args["title"].(string)
		content, _ := args["content"].(string)
		obsType, _ := args["type"].(string)
		topicKey, _ := args["topic_key"].(string)

		if title == "" {
			writeError(id, "title is required")
			return
		}

		obs := &engram.Observation{
			Title:    title,
			Type:     obsType,
			Content:  content,
			TopicKey: topicKey,
		}
		if err := store.Save(obs); err != nil {
			writeError(id, err.Error())
			return
		}

		writeJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": fmt.Sprintf("Saved: %s (id: %s)", title, obs.ID),
					},
				},
			},
		})

	case "mem_search":
		query, _ := args["query"].(string)
		project, _ := args["project"].(string)
		obsType, _ := args["type"].(string)
		limit := 10
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}

		results, err := store.Search(query, engram.SearchOptions{
			Project: project,
			Type:    obsType,
			Limit:   limit,
		})
		if err != nil {
			writeError(id, err.Error())
			return
		}

		entries := make([]map[string]any, 0, len(results))
		for _, r := range results {
			entries = append(entries, map[string]any{
				"id":        r.ID,
				"title":     r.Title,
				"type":      r.Type,
				"content":   r.Content[:min(len(r.Content), 300)],
				"topic_key": r.TopicKey,
				"created":   r.CreatedAt,
			})
		}

		writeJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": fmt.Sprintf("Found %d results", len(entries)),
					},
					{
						"type":     "json",
						"json":     entries,
						"_entries": entries,
					},
				},
			},
		})

	case "mem_get":
		obsID, _ := args["id"].(string)
		if obsID == "" {
			writeError(id, "id is required")
			return
		}
		obs, err := store.Get(obsID)
		if err != nil {
			writeError(id, err.Error())
			return
		}
		writeJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"content": []map[string]any{
					{
						"type": "json",
						"json": map[string]any{
							"id":        obs.ID,
							"title":     obs.Title,
							"type":      obs.Type,
							"content":   obs.Content,
							"topic_key": obs.TopicKey,
							"project":   obs.Project,
							"created":   obs.CreatedAt,
							"updated":   obs.UpdatedAt,
						},
					},
				},
			},
		})

	case "mem_delete":
		obsID, _ := args["id"].(string)
		if obsID == "" {
			writeError(id, "id is required")
			return
		}
		if err := store.Delete(obsID); err != nil {
			writeError(id, err.Error())
			return
		}
		writeJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "Deleted"},
				},
			},
		})

	default:
		writeError(id, fmt.Sprintf("unknown tool: %s", name))
	}
}

func buildToolList(scope string) []map[string]any {
	tools := []map[string]any{
		{
			"name":        "mem_save",
			"description": "Save an observation to persistent memory. Call this after completing significant work like decisions, bug fixes, or discoveries. Include title, content (with What/Why/Where/Learned), and optional type/topic_key.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":     map[string]any{"type": "string", "description": "Short searchable title"},
					"content":   map[string]any{"type": "string", "description": "Structured content"},
					"type":      map[string]any{"type": "string", "description": "decision|architecture|bugfix|discovery|config|preference"},
					"topic_key": map[string]any{"type": "string", "description": "Optional stable key for upsert"},
					"scope":     map[string]any{"type": "string", "description": "project|personal"},
				},
				"required": []string{"title"},
			},
		},
		{
			"name":        "mem_search",
			"description": "Search persistent memory by keywords. Returns matching observations with their IDs, titles, types, and content previews.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":   map[string]any{"type": "string", "description": "Search keywords"},
					"project": map[string]any{"type": "string", "description": "Filter by project"},
					"type":    map[string]any{"type": "string", "description": "Filter by type"},
					"limit":   map[string]any{"type": "number", "description": "Max results"},
				},
				"required": []string{},
			},
		},
		{
			"name":        "mem_get",
			"description": "Get a full observation by its ID. Returns complete content.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string", "description": "Observation ID"},
				},
				"required": []string{"id"},
			},
		},
	}

	if scope == "all" {
		tools = append(tools, map[string]any{
			"name": "mem_delete",
			"description": "Delete an observation by ID.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string", "description": "Observation ID"},
				},
				"required": []string{"id"},
			},
		})
	}

	return tools
}

func writeJSON(v any) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
}

func writeError(id any, message string) {
	writeJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    -32603,
			"message": message,
		},
	})
}
