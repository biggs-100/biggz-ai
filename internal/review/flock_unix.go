//go:build !windows

package review

import "syscall"

func flockExclusive(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB)
}

func flockShared(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_SH|syscall.LOCK_NB)
}

func flockUnlock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_UN)
}
