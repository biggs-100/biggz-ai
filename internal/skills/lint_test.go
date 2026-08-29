package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func genBodyTokens(n int) string {
	return strings.Repeat("word ", n)
}

func TestCountTokens(t *testing.T) {
	if CountTokens("a b c") != 3 {
		t.Fatal("count failed")
	}
	if CountTokens("") != 0 {
		t.Fatal("empty should be 0")
	}
}

func TestLintSkill_Valid300Pass(t *testing.T) {
	body := genBodyTokens(300)
	content := "---\nname: test-skill\ndescription: \"Trigger: test skill, does something. Trigger phrase here.\"\n---\n" + body + "\n"
	p := writeSkill(t, content)
	tokens, diags, err := LintSkill(p)
	if err != nil {
		t.Fatal(err)
	}
	if tokens != 300 {
		t.Fatalf("expected 300 tokens got %d", tokens)
	}
	if HasHardFailure(diags) {
		t.Fatalf("expected pass, got FAIL diags %v", diags)
	}
	if HasWarning(diags) {
		t.Fatalf("300 should not warn, got %v", diags)
	}
}

func TestLintSkill_HardLimitFail(t *testing.T) {
	body := genBodyTokens(1001)
	content := "---\nname: test-skill\ndescription: \"Trigger: test skill overflow.\"\n---\n" + body + "\n"
	p := writeSkill(t, content)
	_, diags, err := LintSkill(p)
	if err != nil {
		t.Fatal(err)
	}
	if !HasHardFailure(diags) {
		t.Fatalf("expected hard fail for 1001 tokens, diags %v", diags)
	}
}

func TestLintSkill_MissingTriggerFail(t *testing.T) {
	body := genBodyTokens(200)
	content := "---\nname: test-skill\ndescription: \"no trigger here, just description.\"\n---\n" + body + "\n"
	p := writeSkill(t, content)
	_, diags, err := LintSkill(p)
	if err != nil {
		t.Fatal(err)
	}
	if !HasHardFailure(diags) {
		t.Fatalf("expected fail for missing trigger, diags %v", diags)
	}
	// also test missing frontmatter
	p2 := writeSkill(t, body)
	_, diags2, _ := LintSkill(p2)
	if !HasHardFailure(diags2) {
		t.Fatalf("expected fail for missing frontmatter, diags %v", diags2)
	}
	// unquoted description
	content3 := "---\nname: test-skill\ndescription: Trigger: unquoted should fail\n---\n" + body + "\n"
	p3 := writeSkill(t, content3)
	_, diags3, _ := LintSkill(p3)
	if !HasHardFailure(diags3) {
		t.Fatalf("expected fail for unquoted, diags %v", diags3)
	}
}

func TestLintSkill_600Warn(t *testing.T) {
	body := genBodyTokens(600)
	content := "---\nname: test-skill\ndescription: \"Trigger: test skill warn case.\"\n---\n" + body + "\n"
	p := writeSkill(t, content)
	_, diags, err := LintSkill(p)
	if err != nil {
		t.Fatal(err)
	}
	if HasHardFailure(diags) {
		t.Fatalf("600 should not hard fail, got %v", diags)
	}
	if !HasWarning(diags) {
		t.Fatalf("600 should warn, got %v", diags)
	}
}
