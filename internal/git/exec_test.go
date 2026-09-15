package git

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run go test ./internal/git -update)", err)
	}
	if got != string(want) {
		t.Errorf("golden %s mismatch\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

// initRepo creates a fresh git repository and returns its root.
func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return root
}

// canonical returns the symlink-resolved form of path.
func canonical(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", path, err)
	}
	return resolved
}

// TestRunPreservesRawStderr pins the cas_store contract: a non-zero exit
// surfaces as *exec.ExitError carrying git's unmodified stderr, stdout verbatim.
func TestRunPreservesRawStderr(t *testing.T) {
	root := initRepo(t)
	if out, err := Run(t.Context(), root, "rev-parse", "--is-inside-work-tree"); err != nil || string(out) != "true\n" {
		t.Fatalf("Run in repo = %q, %v, want %q", out, err, "true\n")
	}
	t.Setenv("LANG", "C")
	t.Setenv("LC_ALL", "C")
	dir := t.TempDir() // deliberately not a repository
	stdout, err := Run(t.Context(), dir, "rev-parse", "--is-inside-work-tree")
	var exitErr *exec.ExitError
	if err == nil || !errors.As(err, &exitErr) {
		t.Fatalf("error %v (%T) does not carry *exec.ExitError, stdout %q", err, err, stdout)
	}
	direct := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	direct.Dir = dir
	var directStderr bytes.Buffer
	direct.Stderr = &directStderr
	_ = direct.Run()
	if got, want := string(exitErr.Stderr), directStderr.String(); got != want {
		t.Fatalf("stderr not verbatim\ngot:  %q\nwant: %q", got, want)
	}
	if !strings.Contains(strings.ToLower(string(exitErr.Stderr)), "not a git repository") {
		t.Fatalf("stderr %q misses git's wording used by cas_store", exitErr.Stderr)
	}
}

// TestRepositoryAnchorMatrix is TM-2's RED test: every selector style resolves
// the same repository, and a foreign cwd never wins.
func TestRepositoryAnchorMatrix(t *testing.T) {
	root := initRepo(t)
	sub := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := t.TempDir()
	wantTop, wantGit := canonical(t, root), canonical(t, filepath.Join(root, ".git"))

	cases := []struct{ name, cwd, repo string }{
		{"relative cwd, dot selector", root, "."},
		{"relative cwd, relative selector", root, filepath.Join("nested", "deeper")},
		{"foreign cwd, absolute selector", foreign, root},
		{"subdirectory cwd, dot selector", sub, "."},
		{"foreign cwd, subdirectory selector", foreign, sub},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(tc.cwd)
			got, err := TopLevel(t.Context(), tc.repo)
			if err != nil || got != wantTop {
				t.Fatalf("TopLevel = %q, %v, want %q", got, err, wantTop)
			}
			gitDir, commonDir, err := ResolveGitDirs(t.Context(), tc.repo)
			if err != nil || gitDir != wantGit || commonDir != wantGit {
				t.Fatalf("ResolveGitDirs = %q, %q, %v, want %q", gitDir, commonDir, err, wantGit)
			}
			if !filepath.IsAbs(gitDir) || canonical(t, gitDir) != gitDir {
				t.Fatalf("gitDir %q is not absolute and symlink-resolved", gitDir)
			}
			if inside, err := RevParse(t.Context(), tc.repo, "--is-inside-work-tree"); err != nil || inside != "true" {
				t.Fatalf("RevParse = %q, %v, want %q", inside, err, "true")
			}
		})
	}
}

// TestResolveGitDirsSymlinkResolved proves a symlinked selector is canonicalized
// to the real repository's directories.
func TestResolveGitDirsSymlinkResolved(t *testing.T) {
	root := initRepo(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitDir, commonDir, err := ResolveGitDirs(t.Context(), link)
	if err != nil {
		t.Fatalf("ResolveGitDirs: %v", err)
	}
	if want := canonical(t, filepath.Join(root, ".git")); gitDir != want || commonDir != want {
		t.Fatalf("ResolveGitDirs via symlink = %q, %q, want %q", gitDir, commonDir, want)
	}
}

// TestNewCommandGolden pins the constructor's env/argv contract: --no-pager,
// -C from the explicit root, inherited GIT_*/locale overrides stripped,
// LANG=C and caller extras appended.
func TestNewCommandGolden(t *testing.T) {
	t.Setenv("GIT_DIR", "/leaked/repo")
	t.Setenv("GIT_INDEX_FILE", "/leaked/index")
	t.Setenv("LANG", "fr_FR.UTF-8")
	t.Setenv("LC_ALL", "fr_FR.UTF-8")

	cmd := NewCommand(ExecOptions{Repo: "/repo/root", NoPager: true, ExtraEnv: []string{"GIT_DIR=/view"}},
		"rev-parse", "HEAD^{tree}")
	golden(t, "new_command.argv.golden", strings.Join(cmd.Args, "\n")+"\n")
	tail := cmd.Env[len(cmd.Env)-3:]
	golden(t, "new_command.env.golden", strings.Join(tail, "\n")+"\n")
	for _, entry := range cmd.Env[:len(cmd.Env)-3] {
		name, _, _ := strings.Cut(entry, "=")
		switch upper := strings.ToUpper(name); {
		case strings.HasPrefix(upper, "GIT_"), upper == "LANG", upper == "LC_ALL":
			t.Errorf("inherited env leaked %q", entry)
		}
	}
}
