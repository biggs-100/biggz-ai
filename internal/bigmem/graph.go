package bigmem

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// GraphNode represents a node in the memory graph (an observation with topic_key).
type GraphNode struct {
	ID       string `json:"id"`
	TopicKey string `json:"topic_key"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Project  string `json:"project,omitempty"`
}

// GraphEdge represents a relation between two observations.
type GraphEdge struct {
	SourceID   string  `json:"source"`
	TargetID   string  `json:"target"`
	Relation   string  `json:"relation"`
	Confidence float64 `json:"confidence"`
}

// BuildGraph queries the store for topic_key hierarchy and memory_relations BM25 edges.
// project filters observations by project when non-empty; limit caps both queries (default 50).
func (s *Store) BuildGraph(project string, limit int) ([]GraphNode, []GraphEdge, error) {
	if limit <= 0 {
		limit = 50
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Nodes: observations with topic_key.
	q := `SELECT id, topic_key, title, type, project FROM observations WHERE topic_key IS NOT NULL AND topic_key != '' AND deleted_at IS NULL`
	var args []any
	if project != "" {
		q += " AND project = ?"
		args = append(args, project)
	}
	q += " ORDER BY topic_key LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("graph nodes: %w", err)
	}
	defer rows.Close()

	var nodes []GraphNode
	for rows.Next() {
		var n GraphNode
		if err := rows.Scan(&n.ID, &n.TopicKey, &n.Title, &n.Type, &n.Project); err != nil {
			continue
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nodes, nil, err
	}

	// Edges: memory_relations with allowed relations.
	edgeQ := `SELECT source_id, target_id, relation, COALESCE(confidence,0) FROM memory_relations WHERE relation IN ('related','compatible','scoped','conflicts_with','supersedes') LIMIT ?`
	erows, err := s.db.Query(edgeQ, limit)
	if err != nil {
		// table may not exist in fresh DB — return nodes only.
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].TopicKey < nodes[j].TopicKey })
		return nodes, nil, nil
	}
	defer erows.Close()

	var edges []GraphEdge
	for erows.Next() {
		var e GraphEdge
		if err := erows.Scan(&e.SourceID, &e.TargetID, &e.Relation, &e.Confidence); err != nil {
			continue
		}
		edges = append(edges, e)
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].TopicKey < nodes[j].TopicKey })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].SourceID == edges[j].SourceID {
			return edges[i].TargetID < edges[j].TargetID
		}
		return edges[i].SourceID < edges[j].SourceID
	})

	return nodes, edges, nil
}

// graphTreeNode is an internal trie for ASCII rendering.
type graphTreeNode struct {
	name     string
	children map[string]*graphTreeNode
	isLeaf   bool
}

func sortedKeys(m map[string]*graphTreeNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// RenderASCII renders the topic_key hierarchy as an indented tree with relations as edges.
// Returns "No graph data" when both nodes and edges are empty.
func RenderASCII(nodes []GraphNode, edges []GraphEdge) string {
	if len(nodes) == 0 && len(edges) == 0 {
		return "No graph data"
	}

	// Build trie.
	root := &graphTreeNode{children: make(map[string]*graphTreeNode)}
	for _, n := range nodes {
		parts := strings.Split(n.TopicKey, "/")
		cur := root
		for i, part := range parts {
			if part == "" {
				continue
			}
			if _, ok := cur.children[part]; !ok {
				cur.children[part] = &graphTreeNode{name: part, children: make(map[string]*graphTreeNode)}
			}
			cur = cur.children[part]
			if i == len(parts)-1 {
				cur.isLeaf = true
			}
		}
	}

	var b strings.Builder

	// Render trie recursively.
	keys := sortedKeys(root.children)
	for i, k := range keys {
		child := root.children[k]
		isLast := i == len(keys)-1
		renderTree(child, "", isLast, &b)
	}

	// Render relations.
	if len(edges) > 0 {
		// Map observation ID to topic_key for friendly display.
		idToKey := make(map[string]string, len(nodes))
		for _, n := range nodes {
			idToKey[n.ID] = n.TopicKey
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Relations:\n")
		for _, e := range edges {
			from := idToKey[e.SourceID]
			if from == "" {
				from = e.SourceID
				if len(from) > 16 {
					from = from[:16]
				}
			}
			to := idToKey[e.TargetID]
			if to == "" {
				to = e.TargetID
				if len(to) > 16 {
					to = to[:16]
				}
			}
			confStr := strconv.FormatFloat(e.Confidence, 'g', -1, 64)
			// Ensure at least one decimal place for integer confidences? Keep raw.
			b.WriteString(fmt.Sprintf("  %s --%s--> %s (%s)\n", from, e.Relation, to, confStr))
		}
	}

	result := strings.TrimRight(b.String(), "\n")
	if result == "" {
		return "No graph data"
	}
	return result
}

func renderTree(n *graphTreeNode, prefix string, isLast bool, b *strings.Builder) {
	connector := "├─ "
	if isLast {
		connector = "└─ "
	}
	// Show branch as dir with "/" suffix if it has children, else leaf.
	suffix := ""
	if len(n.children) > 0 {
		suffix = "/"
	}
	b.WriteString(prefix + connector + n.name + suffix + "\n")

	childPrefix := prefix
	if isLast {
		childPrefix += "   "
	} else {
		childPrefix += "│  "
	}
	keys := sortedKeys(n.children)
	for i, k := range keys {
		child := n.children[k]
		childIsLast := i == len(keys)-1
		renderTree(child, childPrefix, childIsLast, b)
	}
}

// RenderDOT renders the graph as DOT digraph text.
func RenderDOT(nodes []GraphNode, edges []GraphEdge) string {
	if len(nodes) == 0 && len(edges) == 0 {
		return "No graph data"
	}
	idToKey := make(map[string]string, len(nodes))
	for _, n := range nodes {
		idToKey[n.ID] = n.TopicKey
	}

	var b strings.Builder
	b.WriteString("digraph bigmem {\n")
	b.WriteString("  rankdir=LR;\n")
	// Deduplicate topic_keys for nodes (multiple observations may share same topic_key via revisions, but BuildGraph dedups by latest)
	seen := make(map[string]bool)
	for _, n := range nodes {
		if seen[n.TopicKey] {
			continue
		}
		seen[n.TopicKey] = true
		safe := strings.ReplaceAll(n.TopicKey, "\"", "\\\"")
		b.WriteString(fmt.Sprintf("  \"%s\";\n", safe))
	}
	for _, e := range edges {
		from := idToKey[e.SourceID]
		if from == "" {
			from = e.SourceID
		}
		to := idToKey[e.TargetID]
		if to == "" {
			to = e.TargetID
		}
		fromSafe := strings.ReplaceAll(from, "\"", "\\\"")
		toSafe := strings.ReplaceAll(to, "\"", "\\\"")
		confStr := strconv.FormatFloat(e.Confidence, 'g', -1, 64)
		label := e.Relation + " " + confStr
		labelSafe := strings.ReplaceAll(label, "\"", "\\\"")
		b.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n", fromSafe, toSafe, labelSafe))
	}
	b.WriteString("}\n")
	return b.String()
}

// RenderJSON renders the graph as {"nodes":[...],"edges":[...] } JSON.
func RenderJSON(nodes []GraphNode, edges []GraphEdge) (string, error) {
	if nodes == nil {
		nodes = []GraphNode{}
	}
	if edges == nil {
		edges = []GraphEdge{}
	}
	payload := map[string]any{
		"nodes": nodes,
		"edges": edges,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
