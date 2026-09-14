package gitexec

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const cleanGo = "package p\n"

// spawnGo is a target whose single site sits on line 5.
const spawnGo = `package p

import "os/exec"

func f() { exec.Command("git", "status") }
`

// writeFiles materialises a source tree under a fresh temporary root.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, src := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scan(t *testing.T, files map[string]string) []Finding {
	t.Helper()
	res, err := (Scanner{Root: writeFiles(t, files)}).Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return res.Findings
}

// TestFixtureSetIsThePositiveControl pins the checked-in fixture census: the scanner
// must report exactly these sites, so a scanner that resolves nothing cannot pass.
func TestFixtureSetIsThePositiveControl(t *testing.T) {
	res, err := SelfCheck("testdata/fixtures")
	if err != nil {
		t.Fatalf("SelfCheck(testdata/fixtures): %v", err)
	}
	want := map[string]int{
		"spawns.go":                       5,
		"internal/doctor/checks.go":       2,
		"internal/assets/pi/host-tool.ts": 2,
		"clean.go":                        0,
		"internal/assets/pi/notes.md":     0,
	}
	got := map[string]int{}
	var debt, boundary int
	for _, f := range res.Findings {
		got[f.Path]++
		if f.Rule == RuleGoGitSpawn {
			debt++
		} else {
			boundary++
		}
		if f.Line < 1 {
			t.Fatalf("finding must report file:line, got %q", f)
		}
	}
	for path, n := range want {
		if got[path] != n {
			t.Errorf("%s: %d sites, want %d", path, got[path], n)
		}
	}
	sites := 0
	for _, n := range want {
		if n > 0 {
			sites++
		}
	}
	if len(got) != sites {
		t.Errorf("scanned %d files with sites, want %d: %v", len(got), sites, got)
	}
	if debt != 6 || boundary != 3 {
		t.Errorf("debt = %d, boundary = %d; want 6 and 3", debt, boundary)
	}
}

// TestScanScopeExcludesHarnessAndProse is TM-1: classification follows syntax and
// path scope, so documentation and CI text that merely mentions a spawn is never
// counted, and host spawns are only read under the host prefix.
func TestScanScopeExcludesHarnessAndProse(t *testing.T) {
	tests := []struct {
		name string
		file string
		src  string
	}{
		{"ci harness", ".github/workflows/ci.yml", "run: exec.Command(\"git\", \"status\")\n"},
		{"host spawn outside the host prefix", "internal/assets/other/x.ts", `const r = execFileSync("git", ["status"]);`},
		{"spawn text in markdown", "docs/notes.md", "exec.Command(\"git\", \"status\") is forbidden\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// A scan resolving no .go target fails by design, so the probe keeps one.
			if findings := scan(t, map[string]string{tc.file: tc.src, "anchor.go": cleanGo}); len(findings) != 0 {
				t.Fatalf("got %d findings, want none: %v", len(findings), findings)
			}
		})
	}
}

// TestSelfCheckRejectsADeadScanner is the "dead scanner" case: zero findings or zero
// resolved targets must fail the self-test, never read as a clean tree.
func TestSelfCheckRejectsADeadScanner(t *testing.T) {
	tests := map[string]map[string]string{
		"scanner reports nothing":  {"clean.go": cleanGo},
		"scanner resolves nothing": {"notes.md": "exec.Command(\"git\")\n"},
	}
	for name, files := range tests {
		if _, err := SelfCheck(writeFiles(t, files)); err == nil {
			t.Fatalf("%s: SelfCheck passed, want failure", name)
		}
	}
}

// TestReconcileCoversTheBaselineScenarios exercises debt and boundary entries:
// matching, new sites, stale entries and line rot.
func TestReconcileCoversTheBaselineScenarios(t *testing.T) {
	debt := Finding{Rule: RuleGoGitSpawn, Path: "x.go", Line: 7}
	boundary := Finding{Rule: RuleDeclaredBoundary, Path: "internal/doctor/git.go", Line: 76}
	other := Finding{Rule: RuleGoGitSpawn, Path: "y.go", Line: 3}
	tests := []struct {
		name     string
		entries  []Finding
		findings []Finding
		stale    int
		added    int
	}{
		{"fully matched baseline passes", []Finding{debt}, []Finding{debt}, 0, 0},
		{"new site without an entry blocks", nil, []Finding{debt}, 0, 1},
		{"stale entry matches nothing", []Finding{debt}, nil, 1, 0},
		{"stale boundary entry is a failure", []Finding{boundary}, nil, 1, 0},
		{"migrated debt entry is stale", []Finding{debt, other}, []Finding{other}, 1, 0},
		{"boundary entry with a live site keeps passing", []Finding{boundary}, []Finding{boundary}, 0, 0},
		{"debt zero with boundary remaining passes", []Finding{boundary}, []Finding{boundary}, 0, 0},
		{"rotten line is refreshed, not stale", []Finding{{Rule: RuleGoGitSpawn, Path: "x.go", Line: 999}}, []Finding{debt}, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stale, added, refreshed := Reconcile(tc.entries, tc.findings)
			if len(stale) != tc.stale || len(added) != tc.added {
				t.Fatalf("stale = %d, added = %d; want %d and %d", len(stale), len(added), tc.stale, tc.added)
			}
			if len(refreshed) != len(tc.entries) {
				t.Fatalf("refreshed = %d entries, want %d: reconcile never adds or deletes", len(refreshed), len(tc.entries))
			}
			if len(refreshed) == 1 && len(tc.findings) == 1 && refreshed[0].Line != tc.findings[0].Line {
				t.Errorf("refreshed line = %d, want %d", refreshed[0].Line, tc.findings[0].Line)
			}
		})
	}
}

// TestUnreadableTargetFailsTheScan: an input the scan cannot read is a failure,
// never a skipped file.
func TestUnreadableTargetFailsTheScan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 does not deny reads on Windows")
	}
	root := writeFiles(t, map[string]string{"x.go": cleanGo})
	if err := os.Chmod(filepath.Join(root, "x.go"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := (Scanner{Root: root}).Scan(); err == nil {
		t.Fatal("unreadable target passed the scan, want failure")
	}
}

func buildChecker(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gitexec")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, "./cmd/gitexec").CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/gitexec: %v\n%s", err, out)
	}
	return bin
}

// runChecker runs the built checker the way CI does and returns its exit status.
func runChecker(t *testing.T, checker, root, fixtures string, update bool) (int, string) {
	t.Helper()
	args := []string{"-root", root, "-fixtures", fixtures}
	if update {
		args = append(args, "-update")
	}
	cmd := exec.Command(checker, args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err == nil {
		return 0, out.String()
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("run %v: %v", args, err)
	}
	return exit.ExitCode(), out.String()
}

// TestCommandExitIsTheVerdict is the CLI contract: exit 0 only when the self-check
// passed, targets resolved and every baseline entry matched.
func TestCommandExitIsTheVerdict(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the checker binary")
	}
	checker := buildChecker(t)
	fixtures := filepath.Join("testdata", "fixtures")
	tests := []struct {
		name     string
		files    map[string]string
		baseline string
		update   bool
		want     int
		debt     string
	}{
		{"fully matched baseline passes", map[string]string{"x.go": spawnGo}, "go-git-spawn\tx.go\t5\n", false, 0, "1"},
		{"new site without an entry blocks", map[string]string{"x.go": spawnGo}, "", false, 1, ""},
		{"stale entry is a failure", map[string]string{"x.go": cleanGo}, "go-git-spawn\tgone.go\t1\n", false, 1, ""},
		{"debt zero with boundary remaining passes", map[string]string{"anchor.go": cleanGo, "internal/assets/pi/a.ts": `execFileSync("git", ["status"]);`}, "declared-boundary\tinternal/assets/pi/a.ts\t1\n", false, 0, "0"},
		{"zero targets cannot pass", map[string]string{"notes.md": "git status\n"}, "", false, 1, ""},
		{"invalid syntax cannot pass", map[string]string{"x.go": "package p\nfunc {\n"}, "", false, 1, ""},
		{"update refreshes a rotten line", map[string]string{"x.go": spawnGo}, "go-git-spawn\tx.go\t999\n", true, 0, "1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFiles(t, tc.files)
			path := filepath.Join(root, ".github", "guard-baseline.txt")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.baseline), 0o644); err != nil {
				t.Fatal(err)
			}
			code, out := runChecker(t, checker, root, fixtures, tc.update)
			if code != tc.want {
				t.Fatalf("exit = %d, want %d\n%s", code, tc.want, out)
			}
			if tc.debt == "" {
				if strings.Contains(out, "gitexec: OK") {
					t.Fatalf("failing run printed a pass verdict:\n%s", out)
				}
				return
			}
			if !strings.Contains(out, tc.debt+" debt") {
				t.Fatalf("verdict must report %s debt:\n%s", tc.debt, out)
			}
			if tc.update {
				refreshed, err := os.ReadFile(filepath.Join(root, ".github", "guard-baseline.txt"))
				if err != nil {
					t.Fatal(err)
				}
				if string(refreshed) != "go-git-spawn\tx.go\t5\n" {
					t.Fatalf("baseline after -update = %q, want the live line", refreshed)
				}
			}
		})
	}
	t.Run("frozen tree matches its baseline", func(t *testing.T) {
		code, out := runChecker(t, checker, filepath.Join("..", ".."), fixtures, false)
		if code != 0 {
			t.Fatalf("exit = %d on the frozen tree, want 0\n%s", code, out)
		}
	})
}

// TestAbsentCheckerCannotPass is the absence case: a checker nobody provisioned
// cannot report a pass, and the shell reports 127 for it.
func TestAbsentCheckerCannotPass(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gitexec-absent")
	out, err := exec.Command(missing, "-root", ".").CombinedOutput()
	if err == nil {
		t.Fatalf("absent checker exited 0: %s", out)
	}
	if strings.Contains(string(out), "gitexec: OK") {
		t.Fatalf("absent checker printed a verdict: %s", out)
	}
	if runtime.GOOS == "windows" {
		return
	}
	sh, lookErr := exec.LookPath("sh")
	if lookErr != nil {
		t.Skip("no sh to check the 127 contract")
	}
	var exit *exec.ExitError
	if err := exec.Command(sh, "-c", "exec "+strconv.Quote(missing)+" -root .").Run(); !errors.As(err, &exit) || exit.ExitCode() != 127 {
		t.Fatalf("absent checker through sh: %v, want exit 127", err)
	}
}

// TestUnbuildableCheckerCannotPass: a build failure must not leave a runnable
// checker behind, so the CI step cannot print a verdict from a failed build.
func TestUnbuildableCheckerCannotPass(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gitexec")
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/unbuildable").CombinedOutput(); err == nil {
		t.Fatalf("unbuildable package built: %s", out)
	}
	if _, err := os.Stat(bin); err == nil {
		t.Fatal("a failed build left a runnable checker behind")
	}
}
