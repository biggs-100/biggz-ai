package filemerge

import (
	"bytes"
	"strings"
	"testing"
)

func TestRemoveKeysJSONC_TopLevel(t *testing.T) {
	existing := []byte(`{
  // user comment stays
  "default_agent": "biggz-orchestrator",
  "theme": "dark",
  "share": "disabled"
}`)
	got, err := RemoveKeysJSONC(existing, "default_agent")
	if err != nil {
		t.Fatalf("RemoveKeysJSONC error = %v", err)
	}
	want := []byte(`{
  // user comment stays
  "theme": "dark",
  "share": "disabled"
}`)
	if !bytes.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveKeysJSONC_Nested(t *testing.T) {
	existing := []byte(`{
  "agent": {
    "biggz-orchestrator": { "mode": "primary" },
    "user-agent": { "mode": "subagent" }
  },
  "mcp": {
    "biggz": { "command": ["/home/u/.biggz/biggz-mcp.exe"] },
    "github": { "command": ["gh"] }
  }
}`)
	got, err := RemoveKeysJSONC(existing, "agent.biggz-orchestrator", "mcp.biggz")
	if err != nil {
		t.Fatalf("RemoveKeysJSONC error = %v", err)
	}
	want := []byte(`{
  "agent": {
    "user-agent": { "mode": "subagent" }
  },
  "mcp": {
    "github": { "command": ["gh"] }
  }
}`)
	if !bytes.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveKeysJSONC_LastPairInContainer(t *testing.T) {
	existing := []byte(`{"a": 1, "b": 2, "mcp": {"biggz": 1}}`)
	got, err := RemoveKeysJSONC(existing, "mcp.biggz", "b")
	if err != nil {
		t.Fatalf("RemoveKeysJSONC error = %v", err)
	}
	want := []byte(`{"a": 1, "mcp": {}}`)
	if !bytes.Equal(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestRemoveKeysJSONC_ScalarInArrayNotTouched(t *testing.T) {
	existing := []byte(`{"list": ["biggz-orchestrator", "x"], "agent": {"biggz-orchestrator": 1}}`)
	got, err := RemoveKeysJSONC(existing, "agent.biggz-orchestrator")
	if err != nil {
		t.Fatalf("RemoveKeysJSONC error = %v", err)
	}
	want := []byte(`{"list": ["biggz-orchestrator", "x"], "agent": {}}`)
	if !bytes.Equal(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestRemoveKeysJSONC_KeyInsideStringNotTouched(t *testing.T) {
	existing := []byte(`{"note": "see agent.biggz-orchestrator in docs", "agent": {"biggz-orchestrator": true}}`)
	got, err := RemoveKeysJSONC(existing, "agent.biggz-orchestrator")
	if err != nil {
		t.Fatalf("RemoveKeysJSONC error = %v", err)
	}
	want := []byte(`{"note": "see agent.biggz-orchestrator in docs", "agent": {}}`)
	if !bytes.Equal(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestRemoveKeysJSONC_KeyInsideCommentNotTouched(t *testing.T) {
	existing := []byte(`{
  // agent.biggz-orchestrator is documented elsewhere
  "agent": {
    "biggz-orchestrator": { "mode": "primary" }
  }
}`)
	got, err := RemoveKeysJSONC(existing, "agent.biggz-orchestrator")
	if err != nil {
		t.Fatalf("RemoveKeysJSONC error = %v", err)
	}
	want := []byte(`{
  // agent.biggz-orchestrator is documented elsewhere
  "agent": {
    
  }
}`)
	if !bytes.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveKeysJSONC_MissingPathNoop(t *testing.T) {
	existing := []byte(`{"a": 1}`)
	got, err := RemoveKeysJSONC(existing, "agent.biggz-orchestrator", "mcp.biggz")
	if err != nil {
		t.Fatalf("RemoveKeysJSONC error = %v", err)
	}
	if !bytes.Equal(got, existing) {
		t.Errorf("missing path should leave document untouched, got %s", got)
	}
}

func TestRemoveKeysJSONC_InvalidDocument(t *testing.T) {
	if _, err := RemoveKeysJSONC([]byte(`{not json}`), "a.b"); err == nil {
		t.Error("RemoveKeysJSONC on invalid JSONC should fail")
	}
}

func TestRemoveKeysJSONC_ResultStaysValidJSONC(t *testing.T) {
	existing := []byte(`{
  "agent": {
    "biggz-orchestrator": { "mode": "primary", "prompt": "{file:./prompts/sdd/sdd-init.md}" },
  },
  "mcp": { "biggz": { "command": ["a", "b"], }, },
}`)
	got, err := RemoveKeysJSONC(existing, "agent.biggz-orchestrator", "mcp.biggz")
	if err != nil {
		t.Fatalf("RemoveKeysJSONC error = %v", err)
	}
	// The re-validation inside RemoveKeysJSONC guarantees parseability; also
	// assert trailing-comma JSONC is tolerated after deletion.
	if !strings.Contains(string(got), `"agent"`) {
		t.Errorf("expected agent key to remain, got: %s", got)
	}
}
