package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ConsentRequest struct {
	Risk      string // "low" | "medium" | "high"
	GitDir    string
	LineageID string
}

const rddConsentFile = "asked.json"

const rddConsentSchema = "biggz-ai.rdd-consent/v1"

type rddConsent struct {
	Schema string `json:"schema"`
}

// CheckConsent checks if consent has already been recorded for this repo.
func CheckConsent(gitDir string) (bool, error) {
	p := filepath.Join(gitDir, rddGenerationsDir, rddConsentFile)
	_, err := os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// RecordConsent records consent as an immutable file under
// <git-dir>/biggz/rdd-mode/asked.json. If the file already exists it is a
// no-op (one-time consent).
func RecordConsent(gitDir string) error {
	if !plausibleGitDir(gitDir) {
		return fmt.Errorf(
			"%s is not a git directory (missing HEAD or objects/refs): refusing to record review consent there; run from inside a repository",
			gitDir)
	}
	dir := filepath.Join(gitDir, rddGenerationsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := filepath.Join(dir, rddConsentFile)
	if _, err := os.Stat(p); err == nil {
		return nil // already exists
	}
	data, _ := json.MarshalIndent(rddConsent{Schema: rddConsentSchema}, "", "  ")
	return os.WriteFile(p, data, 0644)
}

func isInteractive() bool {
	stat, _ := os.Stdout.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// PromptConsent handles the consent flow based on risk level. Low risk
// auto-consents. In non-interactive (CI) mode it prints a notice and records
// consent. In interactive mode it presents a three-option menu.
func PromptConsent(req ConsentRequest) error {
	if req.Risk == "low" {
		return RecordConsent(req.GitDir)
	}

	if !isInteractive() {
		fmt.Fprintln(os.Stderr, "Review needed. Run with --consent relay or 'biggz review start'")
		return RecordConsent(req.GitDir)
	}

	fmt.Printf("Review-driven development is enabled.\nRisk level: %s\n\n", req.Risk)
	fmt.Println("  [1] Review changes (recommended)")
	fmt.Println("  [2] Skip review (one-time)")
	fmt.Println("  [3] Disable RDD permanently")
	fmt.Print("\nSelect an option: ")

	var choice int
	fmt.Scanf("%d", &choice)

	switch choice {
	case 2:
		return RecordConsent(req.GitDir)
	case 3:
		_, err := RDDDisable("", "", "global")
		return err
	default:
		return RecordConsent(req.GitDir)
	}
}
