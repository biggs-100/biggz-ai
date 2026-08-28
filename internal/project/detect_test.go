package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initGit initialises a new git repository in dir.
func initGit(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
}

// --- extractRepoName ---

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "SSH with .git", url: "git@github.com:user/repo.git", want: "repo"},
		{name: "SSH without .git", url: "git@github.com:user/repo", want: "repo"},
		{name: "HTTPS with .git", url: "https://github.com/user/repo.git", want: "repo"},
		{name: "HTTPS without .git", url: "https://github.com/user/repo", want: "repo"},
		{name: "org with dots", url: "git@github.com:Gentleman-Programming/engram.git", want: "engram"},
		{name: "subgroup", url: "git@gitlab.com:group/subgroup/my-project", want: "my-project"},
		{name: "empty", url: "", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRepoName(tc.url)
			if got != tc.want {
				t.Errorf("extractRepoName(%q) = %q; want %q", tc.url, got, tc.want)
			}
		})
	}
}

// --- helpers ---

func TestIsNoiseDir(t *testing.T) {
	if !IsNoiseDir("node_modules") {
		t.Error("node_modules should be noise")
	}
	if !IsNoiseDir("vendor") {
		t.Error("vendor should be noise")
	}
	if IsNoiseDir("my-project") {
		t.Error("my-project should not be noise")
	}
}

func TestCanonicalizePath(t *testing.T) {
	dir := t.TempDir()
	got := CanonicalizePath(dir)
	if got == "" {
		t.Error("CanonicalizePath should not be empty")
	}
	if CanonicalizePath("") != "" {
		t.Error("empty path should return empty")
	}
}

func TestNormalizeProjectName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{" MyProject ", "myproject"},
		{"FOO_BAR", "foo-bar"},
		{"hello  world", "hello-world"},
		{"", "unknown"},
		{"___", "unknown"},
	}
	for _, tc := range tests {
		got := NormalizeProjectName(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeProjectName(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// --- 5-case detection ---

func TestDetectProjectFull_CaseConfig_Env(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIGMEM_PROJECT", "EnvApp")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	res, err := DetectProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Source != SourceConfig {
		t.Errorf("Source = %q; want %q", res.Source, SourceConfig)
	}
	if res.Project != "envapp" {
		t.Errorf("Project = %q; want %q", res.Project, "envapp")
	}
	// cleanup env for other tests
	t.Setenv("BIGMEM_PROJECT", "")
}

func TestDetectProjectFull_CaseConfig_EnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIGMEM_PROJECT", "First")
	t.Setenv("BIGGZ_PROJECT", "Second")
	t.Setenv("ENGRAM_PROJECT", "Third")
	res, err := DetectProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Project != "first" {
		t.Errorf("Project = %q; want %q", res.Project, "first")
	}
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
}

func TestDetectProjectFull_Case1_GitRemote(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	dir := t.TempDir()
	initGit(t, dir)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:testuser/my-cool-repo.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	res := DetectProjectFull(dir)
	if res.Source != SourceGitRemote {
		t.Errorf("Source = %q; want %q", res.Source, SourceGitRemote)
	}
	if res.Project != "my-cool-repo" {
		t.Errorf("Project = %q; want %q", res.Project, "my-cool-repo")
	}
	if res.Error != nil {
		t.Errorf("unexpected error: %v", res.Error)
	}
}

func TestDetectProjectFull_Case2_GitRoot(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	dir := t.TempDir()
	initGit(t, dir)
	res := DetectProjectFull(dir)
	if res.Source != SourceGitRoot {
		t.Errorf("Source = %q; want %q", res.Source, SourceGitRoot)
	}
	if res.Project == "" {
		t.Error("Project must not be empty")
	}
}

func TestDetectProjectFull_Case2_Subdir(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	root := t.TempDir()
	initGit(t, root)
	subdir := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	res := DetectProjectFull(subdir)
	if res.Source != SourceGitRoot {
		t.Errorf("Source = %q; want %q", res.Source, SourceGitRoot)
	}
	wantPath, _ := filepath.EvalSymlinks(root)
	gotPath, _ := filepath.EvalSymlinks(res.Path)
	if gotPath != wantPath {
		t.Errorf("Path = %q; want %q", res.Path, root)
	}
}

func TestDetectProjectFull_Case3_SingleChild(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	parent := t.TempDir()
	child := filepath.Join(parent, "my-child-repo")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	initGit(t, child)
	res := DetectProjectFull(parent)
	if res.Source != SourceGitChild {
		t.Errorf("Source = %q; want %q", res.Source, SourceGitChild)
	}
	if res.Warning == "" {
		t.Error("Warning must be non-empty for git_child promotion")
	}
	if res.Error != nil {
		t.Errorf("unexpected error: %v", res.Error)
	}
	if res.Project != "my-child-repo" {
		t.Errorf("Project = %q; want %q", res.Project, "my-child-repo")
	}
}

func TestDetectProjectFull_Case4_Ambiguous(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	parent := t.TempDir()
	for _, name := range []string{"repo-alpha", "repo-beta"} {
		child := filepath.Join(parent, name)
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		initGit(t, child)
	}
	res := DetectProjectFull(parent)
	if !errors.Is(res.Error, ErrAmbiguousProject) {
		t.Errorf("Error = %v; want ErrAmbiguousProject", res.Error)
	}
	if len(res.AvailableProjects) != 2 {
		t.Errorf("AvailableProjects len = %d; want 2", len(res.AvailableProjects))
	}
	if res.Project != "" {
		t.Errorf("Project = %q; want empty on ambiguous", res.Project)
	}
	if res.Source != SourceAmbiguous {
		t.Errorf("Source = %q; want %q", res.Source, SourceAmbiguous)
	}
	// DetectProject wrapper must return error
	if _, err := DetectProject(parent); !errors.Is(err, ErrAmbiguousProject) {
		t.Errorf("DetectProject error = %v; want ErrAmbiguousProject", err)
	}
}

func TestDetectProjectFull_Case5_DirBasename(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	parent := t.TempDir()
	plain := filepath.Join(parent, "plain-dir")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	res := DetectProjectFull(plain)
	if res.Source != SourceDirBasename {
		t.Errorf("Source = %q; want %q", res.Source, SourceDirBasename)
	}
	if res.Project != "plain-dir" {
		t.Errorf("Project = %q; want %q", res.Project, "plain-dir")
	}
	if res.Error != nil {
		t.Errorf("unexpected error: %v", res.Error)
	}
}

func TestScanChildren_SkipNoise(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	parent := t.TempDir()
	nm := filepath.Join(parent, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	initGit(t, nm)
	legit := filepath.Join(parent, "my-project")
	if err := os.MkdirAll(legit, 0o755); err != nil {
		t.Fatal(err)
	}
	initGit(t, legit)
	res := DetectProjectFull(parent)
	if res.Source != SourceGitChild {
		t.Errorf("Source = %q; want %q (node_modules should be skipped)", res.Source, SourceGitChild)
	}
	if res.Project != "my-project" {
		t.Errorf("Project = %q; want %q", res.Project, "my-project")
	}
}

func TestScanChildren_SkipHidden(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	parent := t.TempDir()
	hidden := filepath.Join(parent, ".hidden-repo")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	initGit(t, hidden)
	visible := filepath.Join(parent, "visible-repo")
	if err := os.MkdirAll(visible, 0o755); err != nil {
		t.Fatal(err)
	}
	initGit(t, visible)
	res := DetectProjectFull(parent)
	if res.Source != SourceGitChild {
		t.Errorf("Source = %q; want %q (hidden should be skipped)", res.Source, SourceGitChild)
	}
	if res.Project != "visible-repo" {
		t.Errorf("Project = %q; want %q", res.Project, "visible-repo")
	}
}

func TestDetectProject_ConfigFile(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	root := t.TempDir()
	initGit(t, root)
	configDir := filepath.Join(root, ".biggz")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{"project_name":"FileApp"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res := DetectProjectFull(root)
	if res.Source != SourceConfig {
		t.Errorf("Source = %q; want %q", res.Source, SourceConfig)
	}
	if res.Project != "fileapp" {
		t.Errorf("Project = %q; want %q", res.Project, "fileapp")
	}
}

func TestDetectProject_EnvOverridesFile(t *testing.T) {
	root := t.TempDir()
	initGit(t, root)
	configDir := filepath.Join(root, ".biggz")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{"project_name":"file-app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BIGMEM_PROJECT", "env-app")
	defer func() { t.Setenv("BIGMEM_PROJECT", ""); t.Setenv("BIGGZ_PROJECT", ""); t.Setenv("ENGRAM_PROJECT", "") }()
	res := DetectProjectFull(root)
	if res.Source != SourceConfig || res.Project != "env-app" {
		t.Fatalf("expected env config override, got %+v", res)
	}
}

func TestDetectProject_InvalidConfig(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "bad/name")
	defer t.Setenv("BIGMEM_PROJECT", "")
	dir := t.TempDir()
	_, err := DetectProject(dir)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestDetectProject_EmptyDir(t *testing.T) {
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
	got := DetectProjectLegacy("")
	if got == "" {
		t.Error("DetectProjectLegacy empty should return non-empty")
	}
}
