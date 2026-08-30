package codegraph

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadHint optionally reads the report JSON for the given change and cwd.
// It returns nil, nil if the JSON does not exist (advisory, no block).
func LoadHint(change, cwd string) (*Report, error) {
	if change == "" {
		return nil, nil
	}
	root, err := resolveCwd(cwd)
	if err != nil {
		return nil, nil
	}
	// Default JSON path
	jsonPath := filepath.Join(root, "openspec", "changes", change, "codegraph.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, nil
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, nil
	}
	// Ensure slices non-nil
	if r.Files == nil {
		r.Files = []FileEntry{}
	}
	if r.Graph.Nodes == nil {
		r.Graph.Nodes = []Node{}
	}
	if r.Graph.Edges == nil {
		r.Graph.Edges = []Edge{}
	}
	return &r, nil
}

// FormatHint returns a human-readable advisory string for the report.
func FormatHint(r *Report) string {
	if r == nil || len(r.Files) == 0 {
		return ""
	}
	out := "CodeGraph advisory hint:\n"
	for _, f := range r.Files {
		out += "- " + f.Path + " ("
		for i, rs := range f.Reasons {
			if i > 0 {
				out += ", "
			}
			out += string(rs)
		}
		out += ")\n"
	}
	if len(r.Graph.Nodes) > 0 {
		out += "Graph: "
		out += string(rune('0'+len(r.Graph.Nodes))) + " nodes, " // simplistic placeholder, actual count handled via Render?
	}
	return out
}
