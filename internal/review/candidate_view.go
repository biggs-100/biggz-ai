package review

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"
)

type ChangedPathEntry struct {
	Path        string `json:"path"`
	Status      string `json:"status"`
	OldMode     string `json:"old_mode"`
	NewMode     string `json:"new_mode"`
	Deleted     bool   `json:"deleted"`
	TypeChanged bool   `json:"type_changed"`
	ModeOnly    bool   `json:"mode_only"`
}

var rawRe = regexp.MustCompile(`^:([0-7]{6}) ([0-7]{6}) ([0-9a-f]{7,64}) ([0-9a-f]{7,64}) ([AMDT]|R[0-9]{3})$`)

func DeriveChangedPathManifest(cwd, baseTree, candidateTree string) ([]ChangedPathEntry, error) {
	if strings.TrimSpace(cwd) == "" || strings.TrimSpace(baseTree) == "" || strings.TrimSpace(candidateTree) == "" {
		return nil, fmt.Errorf("candidate_view: trees required")
	}
	raw, err := runGitRaw(cwd, []string{"diff", "--raw", "-z", "--abbrev=40", "--no-ext-diff", "--find-renames=100%", baseTree, candidateTree})
	if err != nil {
		return nil, err
	}
	toks, err := splitNul(raw)
	if err != nil {
		return nil, err
	}
	var out []ChangedPathEntry
	for i := 0; i < len(toks); {
		m := rawRe.FindStringSubmatch(string(toks[i]))
		i++
		if m == nil {
			return nil, fmt.Errorf("candidate_view: bad header %q", toks[i-1])
		}
		if i >= len(toks) {
			return nil, fmt.Errorf("candidate_view: incomplete")
		}
		first, err := decodePath(toks[i])
		i++
		if err != nil {
			return nil, err
		}
		sr, pathStr := m[5], first
		if strings.HasPrefix(sr, "R") {
			if i >= len(toks) {
				return nil, fmt.Errorf("candidate_view: incomplete rename")
			}
			second, err := decodePath(toks[i])
			i++
			if err != nil {
				return nil, err
			}
			pathStr, sr = second, "A"
		}
		out = append(out, ChangedPathEntry{Path: pathStr, Status: sr, OldMode: m[1], NewMode: m[2], Deleted: m[5] == "D", TypeChanged: m[5] == "T", ModeOnly: m[3] == m[4] && m[1] != m[2]})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Path < out[b].Path })
	return out, nil
}
func DigestChangedPathManifest(m []ChangedPathEntry) string {
	cp := append([]ChangedPathEntry(nil), m...)
	sort.Slice(cp, func(a, b int) bool { return cp[a].Path < cp[b].Path })
	type c struct {
		Path        string `json:"path"`
		Status      string `json:"status"`
		OldMode     string `json:"old_mode"`
		NewMode     string `json:"new_mode"`
		Deleted     bool   `json:"deleted"`
		TypeChanged bool   `json:"type_changed"`
		ModeOnly    bool   `json:"mode_only"`
	}
	canon := make([]c, len(cp))
	for i, e := range cp {
		canon[i] = c{e.Path, e.Status, e.OldMode, e.NewMode, e.Deleted, e.TypeChanged, e.ModeOnly}
	}
	b, _ := json.Marshal(canon)
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}
func IsWithin(parent, path string) bool { return isWithin(parent, path) }
func isWithin(parent, path string) bool {
	parent, path = filepath.Clean(parent), filepath.Clean(path)
	rel, err := filepath.Rel(parent, path)
	if err != nil || rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}
func IsSafeCandidatePath(p string) bool {
	if p == "" || filepath.IsAbs(p) || strings.Contains(p, "\\") || strings.Contains(p, "\x00") {
		return false
	}
	for _, r := range p {
		if r <= 0x1f || r == 0x7f {
			return false
		}
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	return true
}
func decodePath(b []byte) (string, error) {
	if !utf8.Valid(b) || !bytes.Equal([]byte(string(b)), b) {
		return "", fmt.Errorf("candidate_view: invalid utf8")
	}
	s := string(b)
	if !IsSafeCandidatePath(s) {
		return "", fmt.Errorf("candidate_view: unsafe %q", s)
	}
	return s, nil
}
func symlinkHasUnsafePrefix(target string) bool {
	return target == "" || filepath.IsAbs(target) || strings.Contains(target, "\\")
}

func symlinkHasControlChars(target string) bool {
	for _, r := range target {
		if r <= 0x1f || r == 0x7f {
			return true
		}
	}
	return false
}

func symlinkHasInvalidSegment(target string) bool {
	for _, seg := range strings.Split(target, "/") {
		if seg == "" || seg == "." {
			return true
		}
	}
	return false
}

func symlinkIsWindowsDrive(target string) bool {
	if len(target) < 3 {
		return false
	}
	if target[1] != ':' || target[2] != '/' {
		return false
	}
	c := target[0]
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func symlinkResolvedPath(root, entryPath, target string) string {
	return filepath.Clean(filepath.Join(filepath.Dir(filepath.Join(root, filepath.FromSlash(entryPath))), filepath.FromSlash(target)))
}

func symlinkEscapesRoot(root, resolved string) bool {
	meta := filepath.Join(root, ".git")
	return !isWithin(root, resolved) || resolved == meta || isWithin(meta, resolved)
}

func ValidateSymlinkTarget(root, entryPath, target string) error {
	if symlinkHasUnsafePrefix(target) {
		return fmt.Errorf("candidate_view: symlink unsafe")
	}
	if symlinkHasControlChars(target) {
		return fmt.Errorf("candidate_view: symlink unsafe")
	}
	if symlinkHasInvalidSegment(target) {
		return fmt.Errorf("candidate_view: symlink unsafe")
	}
	if symlinkIsWindowsDrive(target) {
		return fmt.Errorf("candidate_view: symlink unsafe")
	}
	resolved := symlinkResolvedPath(root, entryPath, target)
	if symlinkEscapesRoot(root, resolved) {
		return fmt.Errorf("candidate_view: symlink escapes root")
	}
	return nil
}
func MakeReadOnly(root string, m []ChangedPathEntry) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	for _, e := range m {
		if e.Deleted || e.NewMode == "120000" {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(e.Path))
		if !isWithin(root, p) && filepath.Clean(p) != filepath.Clean(root) {
			return fmt.Errorf("candidate_view: escape %q", e.Path)
		}
		mode := os.FileMode(0444)
		if e.NewMode == "100755" {
			mode = 0555
		}
		if err := os.Chmod(p, mode); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if fi, err := os.Lstat(filepath.Join(root, ".git")); err == nil && !fi.IsDir() && fi.Mode()&os.ModeSymlink == 0 {
		_ = os.Chmod(filepath.Join(root, ".git"), 0444)
	}
	for _, d := range candidateDirs(root, m) {
		_ = os.Chmod(d, 0555)
	}
	return nil
}
func candidateDirs(root string, m []ChangedPathEntry) []string {
	set := map[string]struct{}{filepath.Clean(root): {}}
	for _, e := range m {
		if e.Deleted {
			continue
		}
		dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(e.Path)))
		for {
			c := filepath.Clean(dir)
			set[c] = struct{}{}
			if c == filepath.Clean(root) {
				break
			}
			if !isWithin(root, c) && c != filepath.Clean(root) {
				break
			}
			n := filepath.Dir(c)
			if n == c {
				break
			}
			dir = n
		}
	}
	var out []string
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(a, b int) bool { return len(out[a]) > len(out[b]) })
	return out
}
func MakeWritableForCleanup(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if fi.IsDir() {
		ents, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, e := range ents {
			if err := MakeWritableForCleanup(filepath.Join(path, e.Name())); err != nil {
				return err
			}
		}
		return os.Chmod(path, 0755)
	}
	return os.Chmod(path, 0644)
}
func runGitRaw(cwd string, args []string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
func splitNul(raw []byte) ([][]byte, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if raw[len(raw)-1] != 0 {
		return nil, fmt.Errorf("candidate_view: not NUL-terminated")
	}
	var toks [][]byte
	start := 0
	for i, b := range raw {
		if b == 0 {
			toks = append(toks, raw[start:i])
			start = i + 1
		}
	}
	return toks, nil
}
