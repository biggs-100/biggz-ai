package filecoord

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

func acquireCooperativeLock(path string) (*Lease, error) {
	if err := rejectSymlinkedPath(path); err != nil {
		return nil, &OperationalError{Cause: err}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, &OperationalError{Cause: err}
	}
	if err := rejectSymlinkedPath(dir); err != nil {
		return nil, &OperationalError{Cause: err}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, &BusyError{Cause: err}
		}
		return nil, &OperationalError{Cause: err}
	}
	_, _ = f.WriteString("locked\n")
	return &Lease{
		file: f,
		unlock: func(f *os.File) error {
			if f != nil {
				_ = f.Close()
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		},
		close: func() error { return nil },
	}, nil
}

func rejectSymlinkedPath(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	cleaned := filepath.Clean(path)
	if cleaned == "" || cleaned == "." {
		return nil
	}
	current := cleaned
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return errors.New("symlinked path component rejected: " + current)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return nil
}
