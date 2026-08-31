package codegraph

// Reason describes why a file appears in the change-intent report.
type Reason string

const (
	ReasonSDD    Reason = "sdd"
	ReasonImport Reason = "import"
	ReasonCall   Reason = "call"
)

// Weight constants: symbols rank higher than keywords (design decision).
const (
	WeightSymbol  = 2
	WeightKeyword = 1
)

// FileEntry is the flat file list with aggregated reasons.
type FileEntry struct {
	Path    string   `json:"path"`
	Reasons []Reason `json:"reasons"`
}

// Node is a graph vertex.
type Node struct {
	ID      string   `json:"id"`
	Path    string   `json:"path"`
	Reasons []Reason `json:"reasons"`
}

// Edge is a directed dependency or call edge.
type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason Reason `json:"reason"`
}

// Graph contains nodes and edges with transitive closure expanded.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Report is the dual-output payload (JSON + Markdown).
type Report struct {
	Files []FileEntry `json:"files"`
	Graph Graph       `json:"graph"`
}

// Valid reports whether the reason is one of the three allowed values.
func (r Reason) Valid() bool {
	return r == ReasonSDD || r == ReasonImport || r == ReasonCall
}
