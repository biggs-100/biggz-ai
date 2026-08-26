// Package backup provides snapshot and restore functionality for biggz-ai.
//
// It creates timestamped backups of project state (openspec/ directory and
// agent config) and can restore from any existing backup.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Backup describes a single backup snapshot.
type Backup struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size_bytes"`
	Paths     []string  `json:"paths"`
	// Skipped lists paths that were not backed up, with the reason
	// (missing input path, non-regular file, read error). The backup
	// itself still succeeds; callers surface these as warnings.
	Skipped []string `json:"skipped,omitempty"`
}

// Create snapshots the given paths into a timestamped backup file.
// Backup is stored in ~/.biggz/backups/ as a tar.gz file.
func Create(rootDir string, paths []string) (*Backup, error) {
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home dir: %w", err)
		}
		rootDir = filepath.Join(home, ".biggz", "backups")
	}
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir backups: %w", err)
	}

	now := time.Now()
	id := fmt.Sprintf("backup-%s", now.Format("20060102-150405"))
	backupPath := filepath.Join(rootDir, id+".tar.gz")

	// Create tar.gz
	f, err := os.Create(backupPath)
	if err != nil {
		return nil, fmt.Errorf("create backup: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	var totalSize int64
	var backedUp []string
	var skipped []string

	// addFile buffers the file content first and derives the tar header
	// size from what was actually read. Deriving it from a pre-read Stat
	// instead would crash the whole archive with "archive/tar: write too
	// long" when a file grows between stat and read (live SQLite WALs,
	// replaced binaries). Read failures are recorded in skipped, never
	// fatal: one bad file must not lose the rest of the snapshot. It
	// returns true when the file was archived.
	addFile := func(path, name string, fi os.FileInfo) bool {
		if !fi.Mode().IsRegular() {
			skipped = append(skipped, fmt.Sprintf("%s: not a regular file", path))
			return false
		}
		data, err := os.ReadFile(path)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", path, err))
			return false
		}
		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", path, err))
			return false
		}
		header.Name = filepath.ToSlash(name)
		header.Size = int64(len(data))

		if err := tw.WriteHeader(header); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", path, err))
			return false
		}
		if _, err := tw.Write(data); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", path, err))
			return false
		}
		totalSize += int64(len(data))
		return true
	}

	for _, base := range paths {
		info, err := os.Stat(base)
		if err != nil {
			if os.IsNotExist(err) {
				skipped = append(skipped, fmt.Sprintf("%s: not found", base))
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", base, err)
		}

		if info.IsDir() {
			err = filepath.Walk(base, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					// Unreadable entry (e.g. permission denied): record it and
					// keep going so one bad path doesn't kill the snapshot.
					skipped = append(skipped, fmt.Sprintf("%s: %v", path, err))
					return nil
				}
				if fi.IsDir() {
					// Skip VCS and backup/cache dirs to avoid bloat and recursion.
					switch fi.Name() {
					case ".git", "backups", "cache":
						return filepath.SkipDir
					}
					return nil // tar handles dirs via their contents
				}

				rel, err := filepath.Rel(base, path)
				if err != nil {
					skipped = append(skipped, fmt.Sprintf("%s: %v", path, err))
					return nil
				}
				addFile(path, rel, fi)
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walk %s: %w", base, err)
			}
			backedUp = append(backedUp, base)
		} else if addFile(base, filepath.Base(base), info) {
			backedUp = append(backedUp, base)
		}
	}

	return &Backup{
		ID:        id,
		CreatedAt: now,
		Size:      totalSize,
		Paths:     backedUp,
		Skipped:   skipped,
	}, nil
}

// List returns all available backups sorted by date (newest first).
func List(rootDir string) ([]Backup, error) {
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home dir: %w", err)
		}
		rootDir = filepath.Join(home, ".biggz", "backups")
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read backups: %w", err)
	}

	var backups []Backup
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tar.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}

		id := strings.TrimSuffix(e.Name(), ".tar.gz")
		backups = append(backups, Backup{
			ID:        id,
			CreatedAt: info.ModTime(),
			Size:      info.Size(),
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// Prune removes old backups keeping only the newest keep entries.
// rootDir defaults to ~/.biggz/backups when empty. keep defaults to 10 when <=0.
func Prune(rootDir string, keep int) error {
	if keep <= 0 {
		keep = 10
	}
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("home dir: %w", err)
		}
		rootDir = filepath.Join(home, ".biggz", "backups")
	}
	backups, err := List(rootDir)
	if err != nil {
		return err
	}
	if len(backups) <= keep {
		return nil
	}
	// List returns newest first; remove oldest beyond keep.
	for i := keep; i < len(backups); i++ {
		p := filepath.Join(rootDir, backups[i].ID+".tar.gz")
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("prune %s: %w", backups[i].ID, err)
		}
	}
	return nil
}

// Restore extracts a backup to the given target directory.
func Restore(rootDir, backupID, targetDir string) error {
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("home dir: %w", err)
		}
		rootDir = filepath.Join(home, ".biggz", "backups")
	}

	backupPath := filepath.Join(rootDir, backupID+".tar.gz")
	f, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		target := filepath.Join(targetDir, filepath.FromSlash(header.Name))

		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("mkdir parent: %w", err)
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("read %s: %w", header.Name, err)
		}

		if err := os.WriteFile(target, data, os.FileMode(header.Mode)); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
	}

	return nil
}
