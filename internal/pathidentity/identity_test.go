package pathidentity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func aliasedRoots(t *testing.T) (real string, aliased string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real = filepath.Join(base, "real", "repo")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "real"), filepath.Join(base, "alias")); err != nil {
		t.Skipf("symlink not available (requires privilege): %v", err)
	}
	return real, filepath.Join(base, "alias", "repo")
}

func lexicallyWithin(root, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func TestSameDirectoryAcceptsTwoSpellingsOfOneDirectory(t *testing.T) {
	real, aliased := aliasedRoots(t)
	if lexicallyWithin(real, aliased) {
		t.Fatal("precondition failed: the two spellings compare equal lexically, so this test proves nothing")
	}
	if !SameDirectory(real, aliased) {
		t.Fatalf("SameDirectory(%q, %q) = false, want true", real, aliased)
	}
	if !SameDirectory(aliased, real) {
		t.Fatal("SameDirectory is not symmetric")
	}
}

func TestContainsAcceptsAPathAddressedThroughAnAliasedAncestor(t *testing.T) {
	real, aliased := aliasedRoots(t)
	nested := filepath.Join(aliased, "openspec", "changes", "thin")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if lexicallyWithin(real, nested) {
		t.Fatal("precondition failed: the aliased descendant is already lexically within the real root")
	}
	if !Contains(real, nested) {
		t.Fatalf("Contains(%q, %q) = false, want true", real, nested)
	}
	if !Contains(aliased, filepath.Join(real, "openspec")) {
		t.Fatal("Contains did not accept the real descendant under the aliased root")
	}
}

func TestContainsAcceptsTheRootItself(t *testing.T) {
	real, aliased := aliasedRoots(t)
	if !Contains(real, real) || !Contains(real, aliased) {
		t.Fatal("Contains rejected the root addressed as itself")
	}
}

func TestContainsRejectsSiblingsAndAncestors(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "root")
	sibling := filepath.Join(base, "sibling")
	for _, dir := range []string{root, sibling} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if Contains(root, sibling) {
		t.Fatal("Contains accepted a sibling directory")
	}
	if Contains(root, base) {
		t.Fatal("Contains accepted the parent directory")
	}
}

func TestContainsAcceptsADescendantThatDoesNotExistYet(t *testing.T) {
	real, aliased := aliasedRoots(t)
	if !Contains(real, filepath.Join(aliased, "not", "created", "yet")) {
		t.Fatal("Contains rejected a not-yet-created descendant of the aliased root")
	}
}

func TestSameDirectoryRejectsPathsThatDoNotExist(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(base, "missing")
	if SameDirectory(missing, missing) {
		t.Fatal("SameDirectory accepted a directory that does not exist")
	}
}
