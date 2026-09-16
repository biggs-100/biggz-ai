package sdd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- fixtures ---

// syncFullSpecContent is full-spec shaped: H1 + Purpose + Requirements with a
// requirement heading, and no delta section. This is the shape sdd-spec
// mandates for NEW domains.
const syncFullSpecContent = "# Fullspec Domain Specification\n\n" +
	"## Purpose\n\n" +
	"State the purpose of the fullspec domain.\n\n" +
	"## Requirements\n\n" +
	"### Requirement: Full Spec Requirement\n\n" +
	"The system SHALL exist in the fullspec domain.\n\n" +
	"#### Scenario: Full works\n\n" +
	"- **WHEN** the fullspec domain runs\n" +
	"- **THEN** it works\n"

// syncContentWithoutBlocks has content but neither requirement blocks nor a
// delta section: rules 2 and 3 must fail closed on it.
const syncContentWithoutBlocks = "This delta file has prose content but no requirement headings.\n"

// --- unit table: syncResolveDomainWrite contract (rules 1-3, D4-D8) ---

func TestSyncResolveDomainWrite(t *testing.T) {
	fullSpecSource := func(path string) domainSource {
		return domainSource{path: path, bytes: []byte(syncFullSpecContent), fullSpec: true}
	}
	removedSource := func(path string) domainSource {
		return domainSource{path: path, bytes: []byte("## REMOVED Requirements\n\n### Requirement: Gone Requirement\n")}
	}

	cases := []struct {
		name        string
		setup       func(t *testing.T, dir string) (domainInfo, string, []string)
		writeMain   bool
		wantWrite   bool
		wantBlocked bool
	}{
		{
			name: "empty source blocks",
			setup: func(t *testing.T, dir string) (domainInfo, string, []string) {
				src := domainSource{path: filepath.Join(dir, "empty-domain", "spec.md"), bytes: []byte("  \n\t\n"), empty: true}
				return domainInfo{domain: "empty-domain", sources: []domainSource{src}}, filepath.Join(dir, "main", "spec.md"), []string{src.path}
			},
			wantBlocked: true,
		},
		{
			name: "content without requirement blocks blocks",
			setup: func(t *testing.T, dir string) (domainInfo, string, []string) {
				src := domainSource{path: filepath.Join(dir, "noblocks-domain", "spec.md"), bytes: []byte(syncContentWithoutBlocks)}
				return domainInfo{domain: "noblocks-domain", sources: []domainSource{src}}, filepath.Join(dir, "main", "spec.md"), []string{src.path, "Requirement"}
			},
			wantBlocked: true,
		},
		{
			name: "two fullspec candidates block naming both",
			setup: func(t *testing.T, dir string) (domainInfo, string, []string) {
				first := fullSpecSource(filepath.Join(dir, "a-domain", "spec.md"))
				second := fullSpecSource(filepath.Join(dir, "b-domain", "spec.md"))
				return domainInfo{domain: "a-domain", sources: []domainSource{first, second}}, filepath.Join(dir, "main", "spec.md"), []string{first.path, second.path}
			},
			wantBlocked: true,
		},
		{
			name: "byte-equal target skips",
			setup: func(t *testing.T, dir string) (domainInfo, string, []string) {
				return domainInfo{domain: "equal-domain", sources: []domainSource{fullSpecSource(filepath.Join(dir, "equal-domain", "spec.md"))}}, filepath.Join(dir, "main", "spec.md"), nil
			},
			writeMain: true,
		},
		{
			name: "differing existing target blocks",
			setup: func(t *testing.T, dir string) (domainInfo, string, []string) {
				return domainInfo{domain: "differing-domain", sources: []domainSource{fullSpecSource(filepath.Join(dir, "differing-domain", "spec.md"))}}, filepath.Join(dir, "main", "spec.md"), []string{filepath.Join(dir, "differing-domain", "spec.md")}
			},
			writeMain:   true,
			wantBlocked: true,
		},
		{
			name: "removed against absent main blocks",
			setup: func(t *testing.T, dir string) (domainInfo, string, []string) {
				src := removedSource(filepath.Join(dir, "removed-domain", "spec.md"))
				pr, err := ParseDeltaSpec(string(src.bytes))
				if err != nil {
					t.Fatalf("parse fixture: %v", err)
				}
				src.deltas = pr.Deltas
				return domainInfo{domain: "removed-domain", sources: []domainSource{src}, deltas: pr.Deltas}, filepath.Join(dir, "main", "spec.md"), []string{src.path}
			},
			wantBlocked: true,
		},
		{
			name: "absent main and one fullspec source returns raw bytes",
			setup: func(t *testing.T, dir string) (domainInfo, string, []string) {
				return domainInfo{domain: "new-domain", sources: []domainSource{fullSpecSource(filepath.Join(dir, "new-domain", "spec.md"))}}, filepath.Join(dir, "main", "spec.md"), nil
			},
			wantWrite: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			info, mainPath, msgNeedles := tc.setup(t, dir)
			if tc.writeMain {
				content := syncFullSpecContent
				if tc.wantBlocked {
					content = "# Differing Living Spec\n\n## Requirements\n\n### Requirement: Other\n\nSomething else.\n"
				}
				if err := os.MkdirAll(filepath.Dir(mainPath), 0o755); err != nil {
					t.Fatalf("mkdir main: %v", err)
				}
				if err := os.WriteFile(mainPath, []byte(content), 0o644); err != nil {
					t.Fatalf("write main: %v", err)
				}
			}

			got, res, msg, err := syncResolveDomainWrite(info, mainPath)
			if err != nil {
				t.Fatalf("syncResolveDomainWrite: %v", err)
			}
			// Honest status: `applied` appears only where bytes were returned,
			// and every no-write row must not report `applied`.
			if res == SyncApplied && got == nil {
				t.Fatalf("honest status violated: %q with nil bytes", res)
			}
			switch {
			case tc.wantBlocked:
				if res != SyncBlocked {
					t.Fatalf("result = %q, want %q (msg %q)", res, SyncBlocked, msg)
				}
				if got != nil {
					t.Fatalf("blocked row returned bytes: %q", got)
				}
				if !strings.HasPrefix(msg, "sync blocked:") || !strings.Contains(msg, info.domain) || !strings.Contains(msg, ";") {
					t.Fatalf("blocked message %q must carry the fixed shape (prefix, domain, remedy)", msg)
				}
				for _, needle := range msgNeedles {
					if !strings.Contains(msg, needle) {
						t.Fatalf("message %q must contain %q", msg, needle)
					}
				}
			case tc.wantWrite:
				if res != SyncApplied {
					t.Fatalf("result = %q, want %q (msg %q)", res, SyncApplied, msg)
				}
				if !bytes.Equal(got, []byte(syncFullSpecContent)) {
					t.Fatalf("resolved bytes are not the raw source bytes:\n%s", got)
				}
			default:
				if res == SyncApplied {
					t.Fatalf("no-write row reported %q", res)
				}
				if got != nil {
					t.Fatalf("skip row returned bytes: %q", got)
				}
			}
			if !tc.wantWrite {
				if tc.writeMain {
					data, readErr := os.ReadFile(mainPath)
					if readErr != nil {
						t.Fatalf("main must survive: %v", readErr)
					}
					if !bytes.Equal(data, []byte(syncFullSpecContent)) && !strings.Contains(string(data), "Differing Living Spec") {
						t.Fatalf("main was rewritten on a no-write row:\n%s", data)
					}
				} else if _, statErr := os.Stat(mainPath); !os.IsNotExist(statErr) {
					t.Fatalf("no-write row touched %s (stat err %v)", mainPath, statErr)
				}
			}
		})
	}
}
