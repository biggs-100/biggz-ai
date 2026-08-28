//go:build windows

package review

func flockExclusive(fd uintptr) error {
	return nil
}

func flockShared(fd uintptr) error {
	return nil
}

func flockUnlock(fd uintptr) error {
	return nil
}
