package fixtures

import (
	"context"
	"os/exec"
	osexec "os/exec"
)

func direct() {
	exec.Command("git", "status", "--porcelain")
}

func aliased() {
	osexec.Command("git", "log", "--oneline")
}

func contextual(ctx context.Context) {
	exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
}

func wrapped() {
	runCmd("git", "checkout", "-b", "topic")
}

func wrappedWithContext(ctx context.Context) {
	newProjectCommandContext(ctx, "git", "rev-parse", "--show-toplevel")
}
