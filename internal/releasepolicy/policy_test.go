package releasepolicy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateMarkerPlacement(t *testing.T) {
	tests := []struct {
		name      string
		insideDist bool
		wantErr   bool
	}{
		{name: "marker inside dist is rejected", insideDist: true, wantErr: true},
		{name: "marker outside dist is accepted", insideDist: false, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			dist := filepath.Join(root, "dist")
			if err := os.MkdirAll(dist, 0o755); err != nil {
				t.Fatal(err)
			}
			runID := "12345:1:preflight"
			var marker string
			if tt.insideDist {
				marker = filepath.Join(dist, "marker")
			} else {
				// marker outside dist but on same volume, absolute path
				markerDir := t.TempDir()
				markerDir, _ = filepath.EvalSymlinks(markerDir)
				marker = filepath.Join(markerDir, "marker")
			}
			if err := os.WriteFile(marker, []byte(runID+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			// Ensure snapshot exists for the accepted case so Validate's
			// best-effort dist/artifacts.json check does not fail on missing
			// file. It is optional, but create it with a fresh mtime >= marker.
			if !tt.insideDist {
				artifact := filepath.Join(dist, "artifacts.json")
				if err := os.WriteFile(artifact, []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
				// Bump artifact mtime after marker to satisfy ModTime >= markerTime
				now := time.Now().Add(time.Second)
				if err := os.Chtimes(artifact, now, now); err != nil {
					t.Fatal(err)
				}
			}
			err = Validate(root, marker, runID)
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestValidateSnapshotFile(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	dist := filepath.Join(root, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	markerDir := t.TempDir()
	markerDir, _ = filepath.EvalSymlinks(markerDir)
	marker := filepath.Join(markerDir, "marker")
	runID := "run-1"
	if err := os.WriteFile(marker, []byte(runID+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Ensure marker exists before snapshot for ModTime ordering
	time.Sleep(10 * time.Millisecond)
	artifact := filepath.Join(dist, "ok.json")
	if err := os.WriteFile(artifact, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	markerTime := time.Now().Add(-time.Second)
	// Valid path inside dist
	if err := validateSnapshotFile(root, "dist/ok.json", markerTime); err != nil {
		t.Fatalf("validateSnapshotFile valid = %v, want nil", err)
	}
	// Path outside dist must be rejected
	if err := validateSnapshotFile(root, "outside.json", markerTime); err == nil {
		t.Fatal("validateSnapshotFile outside dist should fail")
	}
	// Absolute path rejected
	if err := validateSnapshotFile(root, "/absolute.json", markerTime); err == nil {
		t.Fatal("validateSnapshotFile absolute path should fail")
	}
	// Stale artifact (modtime before marker) rejected
	stale := filepath.Join(dist, "stale.json")
	if err := os.WriteFile(stale, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-10 * time.Second)
	_ = os.Chtimes(stale, past, past)
	futureMarker := time.Now()
	if err := validateSnapshotFile(root, "dist/stale.json", futureMarker); err == nil {
		t.Fatal("validateSnapshotFile stale artifact should fail")
	}
}
