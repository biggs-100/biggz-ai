package fixtures

import "os/exec"

// exec.Command("git", "status") written in prose is not a spawn site.

const checkID = "git"

func lookUps() {
	exec.LookPath("git")
	lookPathFn("git")
}

func literals() {
	_ = "git"
	_ = []string{"git", "status"}
}
