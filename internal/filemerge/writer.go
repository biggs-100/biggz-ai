package filemerge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// runtimeGOOS and syncDirFn are package-level vars so tests can override them
// without spawning a real Windows process.
var runtimeGOOS = func() string { return runtime.GOOS }

// renameFn publishes the staged replacement. Package-level var so tests can
// simulate a rename that reports success without taking effect.
var renameFn = os.Rename

// stagedFile is the staged destination of a durable write.
type stagedFile interface {
	io.Writer
	Name() string
	Chmod(fs.FileMode) error
	Sync() error
	Close() error
}

var createStagedFile = func(dir string) (stagedFile, error) {
	return os.CreateTemp(dir, ".biggz-*.tmp")
}

var syncDirFn = func(dir string) error {
	fd, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open parent directory %q: %w", dir, err)
	}
	defer fd.Close()
	return fd.Sync()
}

const maxAtomicFileSize = 64 << 20

// WriteResult reports what happened to the destination path.
type WriteResult struct {
	Changed bool
	Created bool
}

// WriteFile writes content to path atomically by writing to a temp file in the
// same directory first, then renaming to the target path. This ensures the
// write either completes fully or leaves the original file unchanged.
// The perm parameter sets the file mode on the final file.
// This is a handle-relative writer: the temp is created beside the destination
// so rename is atomic when on the same filesystem, and durability is ensured
// via staged sync + parent directory sync.
func WriteFile(path string, content []byte, perm os.FileMode) error {
	_, _, err := replaceDurably(path, bytes.NewReader(content), perm)
	return err
}

// WriteFileAtomic atomically writes content to path. It reads the existing file
// (if any) and compares its bytes to content. If identical, it skips the write
// and returns WriteResult{Changed: false, Created: false}. If the file does not
// exist, it creates it via temp+rename and returns Created: true. If the file
// exists and content differs, it overwrites atomically and returns Changed: true.
// Parent directories are created as needed (handle-relative): the writer
// resolves symlink/junction parents via EvalSymlinks and ensures the parent
// directory exists and is writable before staging. Windows quoting uses
// pathquote.Quote for any user-facing invocation that references the path.
func WriteFileAtomic(path string, content []byte, perm os.FileMode) (WriteResult, error) {
	if perm == 0 {
		perm = 0o644
	}
	created := false
	existing, err := readComparableFile(path)
	if err == nil {
		if bytes.Equal(existing, content) {
			return WriteResult{}, nil
		}
	} else if !os.IsNotExist(err) {
		return WriteResult{}, fmt.Errorf("read existing file %q: %w", path, err)
	} else {
		created = true
	}
	landed, _, err := replaceDurably(path, bytes.NewReader(content), perm)
	result := WriteResult{Changed: landed, Created: created && landed}
	if err != nil {
		return result, err
	}
	return result, nil
}

// StreamResult describes the bytes that landed at the destination.
type StreamResult struct {
	Bytes  int64
	Digest string
}

// WriteStreamAtomic replaces path with everything readable from src and returns
// the size and SHA-256 of the bytes read back from path afterwards.
func WriteStreamAtomic(path string, src io.Reader, perm fs.FileMode) (StreamResult, error) {
	_, result, err := replaceDurably(path, src, perm)
	return result, err
}

// replaceDurably is the single durability sequence: stage beside destination,
// apply final metadata, sync, close, publish with rename, read back, then sync
// parent directory. Handle-relative: staging directory is derived from path's
// parent after resolving symlinks/junctions so the rename is intra-directory.
func replaceDurably(path string, src io.Reader, perm fs.FileMode) (landed bool, result StreamResult, err error) {
	if perm == 0 {
		perm = 0o644
	}
	dir := filepath.Dir(path)
	if err := ensureAtomicParentDir(dir, path); err != nil {
		return false, StreamResult{}, err
	}
	tmp, err := createStagedFile(dir)
	if err != nil {
		return false, StreamResult{}, fmt.Errorf("create temp file for %q: %w", path, err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	staged := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, staged), src)
	if err != nil {
		_ = tmp.Close()
		return false, StreamResult{}, fmt.Errorf("write temp file for %q: %w", path, err)
	}
	stagedDigest := hex.EncodeToString(staged.Sum(nil))
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return false, StreamResult{}, fmt.Errorf("set permissions on temp file for %q: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return false, StreamResult{}, fmt.Errorf("sync temp file for %q: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return false, StreamResult{}, fmt.Errorf("close temp file for %q: %w", path, err)
	}
	if err := renameFn(tmpPath, path); err != nil {
		return false, StreamResult{}, fmt.Errorf("replace %q atomically: %w", path, err)
	}
	if f, err := os.Open(path); err == nil {
		syncErr := f.Sync()
		_ = f.Close()
		if syncErr != nil && !errors.Is(syncErr, os.ErrPermission) {
			return false, StreamResult{}, fmt.Errorf("sync %q after replacement: %w", path, syncErr)
		}
	} else {
		return false, StreamResult{}, fmt.Errorf("open %q for sync after replacement: %w", path, err)
	}
	diskDigest, diskBytes, err := digestFileOnDisk(path)
	if err != nil {
		return false, StreamResult{}, fmt.Errorf("read back %q after replacement: %w", path, err)
	}
	if diskBytes != written || diskDigest != stagedDigest {
		return false, StreamResult{}, fmt.Errorf(
			"replace %q atomically: the replacement did not land. The destination holds %d bytes (%s); %d bytes (%s) were written",
			path, diskBytes, diskDigest, written, stagedDigest)
	}
	cleanup = false
	result = StreamResult{Bytes: diskBytes, Digest: diskDigest}
	if err := SyncDir(dir); err != nil {
		return true, result, fmt.Errorf("sync parent directory for %q: %w", path, err)
	}
	return true, result, nil
}

// SyncDir flushes dir's entries to stable storage so a published name survives recovery.
// On Windows, ErrPermission from directory sync is tolerated.
func SyncDir(dir string) error {
	err := syncDirFn(dir)
	if err == nil {
		return nil
	}
	if runtimeGOOS() == "windows" && errors.Is(err, os.ErrPermission) {
		return nil
	}
	return err
}

// FileDigest returns the SHA-256 and size of the file at path, read from disk.
func FileDigest(path string) (digest string, size int64, err error) {
	return digestFileOnDisk(path)
}

func digestFileOnDisk(path string) (digest string, size int64, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", 0, fmt.Errorf("destination %q is a symlink", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	sum := sha256.New()
	size, err = io.Copy(sum, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(sum.Sum(nil)), size, nil
}

func readComparableFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink %q", path)
	}
	if info.Size() > maxAtomicFileSize {
		return nil, fmt.Errorf("file %q exceeds max atomic compare size %d bytes", path, maxAtomicFileSize)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAtomicFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAtomicFileSize {
		return nil, fmt.Errorf("file %q exceeds max atomic compare size %d bytes", path, maxAtomicFileSize)
	}
	return data, nil
}

func ensureAtomicParentDir(dir, path string) error {
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create parent directories for %q: %w", path, err)
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return fmt.Errorf("stat parent directory for %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return fmt.Errorf("resolving symlink parent %q for %q: %w", dir, path, err)
		}
		info, err = os.Stat(resolved)
		if err != nil {
			return fmt.Errorf("stat symlink target %q for %q: %w", resolved, path, err)
		}
		dir = resolved
	} else {
		resolved, resolvedInfo, handled, err := resolveAtomicParentJunction(dir, info)
		if err != nil {
			return fmt.Errorf("resolve parent directory for %q: %w", path, err)
		}
		if handled {
			dir = resolved
			info = resolvedInfo
		}
	}
	if !info.IsDir() {
		return fmt.Errorf("parent path %q for %q is not a directory", dir, path)
	}
	if info.Mode().Perm()&0o200 == 0 {
		if err := os.Chmod(dir, 0o755); err != nil {
			return fmt.Errorf("relax parent directory permissions for %q: %w", path, err)
		}
	}
	return nil
}

// resolveAtomicParentJunction handles Windows directory junctions. On non-Windows
// it is a no-op. Kept in this file to avoid extra build-tagged files while
// preserving handle-relative semantics.
func resolveAtomicParentJunction(dir string, info os.FileInfo) (string, os.FileInfo, bool, error) {
	if runtimeGOOS() != "windows" {
		return "", nil, false, nil
	}
	if info.Mode()&os.ModeIrregular == 0 {
		return "", nil, false, nil
	}
	if _, err := os.Readlink(dir); err != nil {
		return "", nil, false, fmt.Errorf("resolving directory junction parent %q: %w", dir, err)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", nil, false, fmt.Errorf("resolving directory junction parent %q: %w", dir, err)
	}
	targetInfo, err := os.Stat(resolved)
	if err != nil {
		return "", nil, false, fmt.Errorf("stat directory junction target %q: %w", resolved, err)
	}
	return resolved, targetInfo, true, nil
}
