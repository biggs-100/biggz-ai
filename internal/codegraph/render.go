package codegraph

import (
	"fmt"
	"sort"
	"strings"
)

// RenderMarkdown renders the report as Markdown with a files table and graph summary.
func RenderMarkdown(r *Report) string {
	if r == nil {
		return "# CodeGraph Report\n\nNo data.\n"
	}
	var b strings.Builder
	b.WriteString("# CodeGraph Report\n\n")
	b.WriteString(fmt.Sprintf("Generated: %d files, %d nodes, %d edges\n\n", len(r.Files), len(r.Graph.Nodes), len(r.Graph.Edges)))

	b.WriteString("## Files\n\n")
	if len(r.Files) == 0 {
		b.WriteString("No files matched.\n\n")
	} else {
		b.WriteString("| Path | Reasons |\n")
		b.WriteString("|------|---------|\n")
		// Sort for determinism
		files := append([]FileEntry(nil), r.Files...)
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		for _, f := range files {
			reasons := make([]string, len(f.Reasons))
			for i, r := range f.Reasons {
				reasons[i] = string(r)
			}
			b.WriteString(fmt.Sprintf("| %s | %s |\n", f.Path, strings.Join(reasons, ", ")))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Graph Summary\n\n")
	b.WriteString(fmt.Sprintf("- Nodes: %d\n", len(r.Graph.Nodes)))
	b.WriteString(fmt.Sprintf("- Edges: %d\n", len(r.Graph.Edges)))
	b.WriteString("\n")

	if len(r.Graph.Nodes) > 0 {
		b.WriteString("### Nodes\n\n")
		b.WriteString("| ID | Path | Reasons |\n")
		b.WriteString("|----|------|---------|\n")
		nodes := append([]Node(nil), r.Graph.Nodes...)
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
		for _, n := range nodes {
			reasons := make([]string, len(n.Reasons))
			for i, r := range n.Reasons {
				reasons[i] = string(r)
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", n.ID, n.Path, strings.Join(reasons, ", ")))
		}
		b.WriteString("\n")
	}

	if len(r.Graph.Edges) > 0 {
		b.WriteString("### Edges\n\n")
		b.WriteString("| From | To | Reason |\n")
		b.WriteString("|------|----|--------|\n")
		edges := append([]Edge(nil), r.Graph.Edges...)
		sort.Slice(edges, func(i, j int) bool {
			if edges[i].From == edges[j].From {
				if edges[i].To == edges[j].To {
					return edges[i].Reason < edges[j].Reason
				}
				return edges[i].To < edges[j].To
			}
			return edges[i].From < edges[j].From
		})
		for _, e := range edges {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", e.From, e.To, string(e.Reason)))
		}
		b.WriteString("\n")
	}

	return b.String()
}
