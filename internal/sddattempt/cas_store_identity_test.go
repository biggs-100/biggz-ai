package sddattempt

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestEncodedNamespaceSeparation pins the `_encoded` namespace rules: on a
// case-insensitive filesystem two identities differing only in case would
// share one ledger directory and silently merge unrelated attempt chains,
// so non-kebab identities get an encoded leaf; kebab identities keep their
// exact verbatim v1/<change> directory so existing ledgers stay reachable.
func TestEncodedNamespaceSeparation(t *testing.T) {
	setStoreRoot(t)

	upper, err := resolveStore("DEC-X", "r")
	if err != nil {
		t.Fatalf("resolveStore(DEC-X): %v", err)
	}
	lower, err := resolveStore("dec-x", "r")
	if err != nil {
		t.Fatalf("resolveStore(dec-x): %v", err)
	}
	if strings.EqualFold(upper.Dir, lower.Dir) {
		t.Fatalf("case variants share ledger directory %q on a case-insensitive filesystem", upper.Dir)
	}

	// The encoded leaf is <lowercased change>-<32 hex> under the _encoded
	// namespace; the suffix is exactly 32 lowercase hex characters. The
	// lowercase sibling is itself a legacy kebab identity, so it keeps the
	// verbatim directory — which is what keeps the two chains apart.
	leaf := filepath.Base(upper.Dir)
	suffix := leaf[strings.LastIndex(leaf, "-")+1:]
	if len(suffix) != 32 {
		t.Fatalf("encoded leaf %q suffix %q is %d hex characters, want exactly 32 (128 bits)", leaf, suffix, len(suffix))
	}
	if strings.Trim(suffix, "0123456789abcdef") != "" {
		t.Fatalf("encoded leaf %q suffix %q is not lowercase hex", leaf, suffix)
	}
	if filepath.Base(filepath.Dir(upper.Dir)) != encodedRuntimeChangeNamespace {
		t.Fatalf("encoded identity %q lives under %q, want %q", upper.Change, filepath.Base(filepath.Dir(upper.Dir)), encodedRuntimeChangeNamespace)
	}
	if lower.Dir != filepath.Join(storeRootOverride, RuntimeVersion, "dec-x") {
		t.Fatalf("lowercase sibling ledger directory = %q, want the verbatim v1/dec-x", lower.Dir)
	}

	// A kebab identity keeps the verbatim directory: every attempt chain
	// written by an earlier version stays reachable.
	kebab, err := resolveStore("kebab-change", "r")
	if err != nil {
		t.Fatalf("resolveStore(kebab-change): %v", err)
	}
	if kebab.Dir != filepath.Join(storeRootOverride, RuntimeVersion, "kebab-change") {
		t.Fatalf("kebab ledger directory = %q, want %q", kebab.Dir, filepath.Join(storeRootOverride, RuntimeVersion, "kebab-change"))
	}

	// The encoded namespace must be unreachable as a legacy identity, so an
	// encoded directory can never collide with a kebab-case change's ledger.
	if legacyRuntimeChangeDir(encodedRuntimeChangeNamespace) {
		t.Fatalf("encoded namespace %q is also a valid legacy change name", encodedRuntimeChangeNamespace)
	}
}

// TestEncodedNamespaceLedgersAreFunctional drives a full grant/status
// round-trip through the encoded directory for a case-variant identity.
func TestEncodedNamespaceLedgersAreFunctional(t *testing.T) {
	setStoreRoot(t)

	root := canonicalDir(t, t.TempDir())
	if _, err := Grant(GrantParams{
		ChangeName: "DEC-EXAMPLE-CHANGE", RepoRoot: "r",
		Roots: []string{root}, Reason: "maintainer authorized sibling repository edits",
		Actor: "maintainer", RequestID: "grant-encoded-1", ChangeInstance: "encoded-token",
	}); err != nil {
		t.Fatalf("grant on encoded identity: %v", err)
	}

	status, err := StatusWithInstance("DEC-EXAMPLE-CHANGE", "r", "encoded-token")
	if err != nil {
		t.Fatalf("StatusWithInstance on encoded identity: %v", err)
	}
	if len(status.GrantedRoots) != 1 || status.GrantedRoots[0] != root {
		t.Fatalf("encoded identity projected %#v, want [%q]", status.GrantedRoots, root)
	}
}
