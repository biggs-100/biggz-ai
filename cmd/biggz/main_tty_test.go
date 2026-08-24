package main

import (
	"os"
	"strings"
	"testing"
)

func TestTTYRequiresBothStreams(t *testing.T) {
	stdinFD := os.Stdin.Fd()
	stdoutFD := os.Stdout.Fd()

	tests := []struct {
		name    string
		isTTY   func(uintptr) bool
		wantErr bool
	}{
		{
			name:    "both TTY -> ok",
			isTTY:   func(fd uintptr) bool { return true },
			wantErr: false,
		},
		{
			name:    "missing stdin",
			isTTY:   func(fd uintptr) bool { return fd == stdoutFD },
			wantErr: true,
		},
		{
			name:    "missing stdout",
			isTTY:   func(fd uintptr) bool { return fd == stdinFD },
			wantErr: true,
		},
		{
			name:    "both pipe -> error",
			isTTY:   func(fd uintptr) bool { return false },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := isattyFn
			t.Cleanup(func() { isattyFn = orig })
			isattyFn = tt.isTTY

			err := checkTUIInteractive()
			if tt.wantErr && err == nil {
				t.Fatalf("checkTUIInteractive() = nil, want error %q", nonInteractiveTUIError)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("checkTUIInteractive() = %v, want nil", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), nonInteractiveTUIError) {
				t.Fatalf("checkTUIInteractive() error = %q, want %q", err.Error(), nonInteractiveTUIError)
			}
		})
	}
}

func TestTTYAllowsInteractiveStdinAndStdout(t *testing.T) {
	orig := isattyFn
	t.Cleanup(func() { isattyFn = orig })

	calls := 0
	isattyFn = func(fd uintptr) bool {
		calls++
		return true
	}

	if err := checkTUIInteractive(); err != nil {
		t.Fatalf("checkTUIInteractive() = %v, want nil", err)
	}
	if calls != 2 {
		t.Fatalf("isattyFn called %d times, want 2 (stdin and stdout)", calls)
	}
}

func TestTTYErrorMessage(t *testing.T) {
	if nonInteractiveTUIError == "" {
		t.Fatal("nonInteractiveTUIError is empty")
	}
	if !strings.Contains(nonInteractiveTUIError, "biggz") {
		t.Fatalf("nonInteractiveTUIError = %q, want to contain 'biggz'", nonInteractiveTUIError)
	}
	if !strings.Contains(strings.ToLower(nonInteractiveTUIError), "stdin") || !strings.Contains(strings.ToLower(nonInteractiveTUIError), "stdout") {
		t.Fatalf("nonInteractiveTUIError = %q, want to mention both stdin and stdout", nonInteractiveTUIError)
	}
}
