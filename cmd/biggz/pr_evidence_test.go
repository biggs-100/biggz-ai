package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeChangeFixture(t *testing.T, root, name string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, "openspec", "changes", name, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

func TestPRBody_WithoutEvidence(t *testing.T) {
	body := buildPRBody("my-change", []string{"a.go", "b.go"})
	if strings.Contains(body, "## Evidence") {
		t.Errorf("body without evidence should not contain ## Evidence, got:\n%s", body)
	}
	if !strings.Contains(body, "## Summary") {
		t.Errorf("body should contain Summary, got:\n%s", body)
	}
}

func TestPREvidence_Minimal(t *testing.T) {
	tmp := t.TempDir()
	writeChangeFixture(t, tmp, "test-change", map[string]string{
		"proposal.md": "# Proposal\n",
		"tasks.md":    "- [x] T1\n- [ ] T2\n",
	})

	body := buildPRBodyWithEvidence("my-change", []string{"a.go"}, prEvidenceOptions{
		WithEvidence: true,
		Cwd:          tmp,
	})
	if !strings.Contains(body, "## Evidence") {
		t.Fatalf("expected Evidence block, got:\n%s", body)
	}
	if !strings.Contains(body, "**SDD Changes**: 1 active") {
		t.Errorf("expected 1 active change, got:\n%s", body)
	}
	if !strings.Contains(body, "`test-change`") {
		t.Errorf("expected test-change in evidence, got:\n%s", body)
	}
	if !strings.Contains(body, "(1/2 tasks)") {
		t.Errorf("expected task progress 1/2, got:\n%s", body)
	}
	if !strings.Contains(body, "**Version**") {
		t.Errorf("expected Version line, got:\n%s", body)
	}
	if !strings.Contains(body, "docs/comparison-with-gentle.md") {
		t.Errorf("expected Comparison line, got:\n%s", body)
	}
	// Ensure base body still present
	if !strings.Contains(body, "## Summary") {
		t.Errorf("expected base Summary preserved, got:\n%s", body)
	}
}

func TestPREvidence_NoOpenspec(t *testing.T) {
	tmp := t.TempDir()
	body := buildPRBodyWithEvidence("my-change", nil, prEvidenceOptions{
		WithEvidence: true,
		Cwd:          tmp,
	})
	if !strings.Contains(body, "**SDD Changes**: 0 active") {
		t.Errorf("expected 0 active when no openspec, got:\n%s", body)
	}
}

func TestPREvidence_ChangeFilter(t *testing.T) {
	tmp := t.TempDir()
	writeChangeFixture(t, tmp, "alpha", map[string]string{
		"proposal.md": "# Proposal\n",
		"tasks.md":    "- [x] T1\n",
	})
	writeChangeFixture(t, tmp, "beta", map[string]string{
		"proposal.md": "# Proposal\n",
		"tasks.md":    "- [ ] T1\n",
	})

	tests := []struct {
		name   string
		filter string
		wantN  string
		wantA  bool
		wantB  bool
	}{
		{name: "all active", filter: "", wantN: "**SDD Changes**: 2 active", wantA: true, wantB: true},
		{name: "filter alpha", filter: "alpha", wantN: "**SDD Changes**: 1 active", wantA: true, wantB: false},
		{name: "filter beta", filter: "beta", wantN: "**SDD Changes**: 1 active", wantA: false, wantB: true},
		{name: "filter missing", filter: "gamma", wantN: "**SDD Changes**: 0 active", wantA: false, wantB: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := buildPRBodyWithEvidence("my-change", nil, prEvidenceOptions{
				WithEvidence: true,
				Cwd:          tmp,
				ChangeFilter: tt.filter,
			})
			if !strings.Contains(body, tt.wantN) {
				t.Errorf("filter %q: expected %q in body, got:\n%s", tt.filter, tt.wantN, body)
			}
			hasA := strings.Contains(body, "`alpha`")
			hasB := strings.Contains(body, "`beta`")
			if hasA != tt.wantA {
				t.Errorf("filter %q: alpha presence = %v, want %v", tt.filter, hasA, tt.wantA)
			}
			if hasB != tt.wantB {
				t.Errorf("filter %q: beta presence = %v, want %v", tt.filter, hasB, tt.wantB)
			}
		})
	}
}

func TestPREvidence_TemplatePlaceholder(t *testing.T) {
	tmp := t.TempDir()
	writeChangeFixture(t, tmp, "demo", map[string]string{
		"proposal.md": "# Proposal\n",
		"tasks.md":    "- [x] T1\n",
	})
	tmpl := filepath.Join(tmp, "pr_template.md")
	content := "# Title\n\n{{evidence}}\n\nFooter"
	if err := os.WriteFile(tmpl, []byte(content), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	body := buildPRBodyWithEvidence("my-change", nil, prEvidenceOptions{
		WithEvidence: true,
		Cwd:          tmp,
		TemplatePath: tmpl,
	})
	if strings.Contains(body, "{{evidence}}") {
		t.Errorf("placeholder should be replaced, got:\n%s", body)
	}
	if !strings.Contains(body, "## Evidence") {
		t.Errorf("expected Evidence after template replace, got:\n%s", body)
	}
	if !strings.Contains(body, "# Title") {
		t.Errorf("expected template title preserved, got:\n%s", body)
	}
	if !strings.Contains(body, "Footer") {
		t.Errorf("expected template footer preserved, got:\n%s", body)
	}
	// Ensure not double-appended base
	if strings.Contains(body, "## Summary") {
		t.Errorf("template mode should not include base Summary when template provided, got:\n%s", body)
	}
}

func TestPREvidence_TemplateWithoutPlaceholder(t *testing.T) {
	tmp := t.TempDir()
	writeChangeFixture(t, tmp, "demo2", map[string]string{
		"proposal.md": "# Proposal\n",
		"tasks.md":    "- [x] T1\n",
	})
	tmpl := filepath.Join(tmp, "tmpl2.md")
	content := "# Custom Template\n\nBody here."
	if err := os.WriteFile(tmpl, []byte(content), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	body := buildPRBodyWithEvidence("my-change", nil, prEvidenceOptions{
		WithEvidence: true,
		Cwd:          tmp,
		TemplatePath: tmpl,
	})
	if !strings.Contains(body, "# Custom Template") {
		t.Errorf("expected template content, got:\n%s", body)
	}
	if !strings.Contains(body, "## Evidence") {
		t.Errorf("expected Evidence appended, got:\n%s", body)
	}
	// template should be prefix
	idxTmpl := strings.Index(body, "# Custom Template")
	idxEv := strings.Index(body, "## Evidence")
	if idxTmpl < 0 || idxEv < 0 || idxEv < idxTmpl {
		t.Errorf("evidence should be appended after template, got:\n%s", body)
	}
}

func TestPREvidence_BasePlaceholder(t *testing.T) {
	tmp := t.TempDir()
	writeChangeFixture(t, tmp, "x", map[string]string{
		"proposal.md": "# Proposal\n",
		"tasks.md":    "- [ ] T1\n",
	})
	// Simulate placeholder handling via direct evidence block check
	evidence := buildEvidenceBlock(tmp, "")
	// Use helper to verify replacement path in buildPRBodyWithEvidence when base contains placeholder?
	// Since buildPRBody never contains placeholder, we test inject logic via direct call:
	// Instead test that evidence block itself is deterministic and that a base with placeholder
	// would be replaced if we used the generic path. We exercise buildEvidenceBlock directly.
	if !strings.Contains(evidence, "## Evidence") {
		t.Fatalf("evidence block missing, got:\n%s", evidence)
	}
	if !strings.Contains(evidence, "`x`") {
		t.Fatalf("expected change x in evidence, got:\n%s", evidence)
	}
	// Ensure remediation absent by default
	if strings.Contains(evidence, "Remediation:") {
		t.Errorf("should not contain remediation when not required, got:\n%s", evidence)
	}
}

func TestPRBodyWithEvidence_Disabled(t *testing.T) {
	tmp := t.TempDir()
	writeChangeFixture(t, tmp, "any", map[string]string{
		"proposal.md": "# Proposal\n",
		"tasks.md":    "- [x] T1\n",
	})
	body := buildPRBodyWithEvidence("my-change", []string{"f.go"}, prEvidenceOptions{
		WithEvidence: false,
		Cwd:          tmp,
	})
	if strings.Contains(body, "## Evidence") {
		t.Errorf("with evidence disabled should not inject, got:\n%s", body)
	}
}

func TestPREvidence_MissingTemplateFallsBack(t *testing.T) {
	tmp := t.TempDir()
	writeChangeFixture(t, tmp, "c1", map[string]string{
		"proposal.md": "# Proposal\n",
		"tasks.md":    "- [x] T1\n",
	})
	missing := filepath.Join(tmp, "does-not-exist.md")
	body := buildPRBodyWithEvidence("my-change", nil, prEvidenceOptions{
		WithEvidence: true,
		Cwd:          tmp,
		TemplatePath: missing,
	})
	if !strings.Contains(body, "## Evidence") {
		t.Fatalf("expected fallback evidence when template missing, got:\n%s", body)
	}
	if !strings.Contains(body, "## Summary") {
		t.Errorf("expected fallback to base body, got:\n%s", body)
	}
}
