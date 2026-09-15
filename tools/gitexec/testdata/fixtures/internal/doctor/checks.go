package doctor

import (
	"context"
	"os/exec"
)

func (c *checker) seam() {
	c.execFn("git", "rev-parse", "--git-dir")
}

func dead(ctx context.Context) {
	exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
}
