package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/platform"
)

const PiWebSearchCheckID CheckID = "pi-web-search"

// PiWebSearchCheck verifies pi web-search extension file+env only, no live probe.
type PiWebSearchCheck struct {
	statFn    func(string) (os.FileInfo, error)
	getenv    func(string) string
	homeDirFn func() (string, error)
}

func NewPiWebSearchCheck() *PiWebSearchCheck {
	return &PiWebSearchCheck{statFn: os.Stat, getenv: os.Getenv, homeDirFn: os.UserHomeDir}
}

func NewPiWebSearchCheckWithCustom(statFn func(string) (os.FileInfo, error), getenv func(string) string, homeDirFn func() (string, error)) *PiWebSearchCheck {
	if statFn == nil {
		statFn = os.Stat
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	if homeDirFn == nil {
		homeDirFn = os.UserHomeDir
	}
	return &PiWebSearchCheck{statFn: statFn, getenv: getenv, homeDirFn: homeDirFn}
}

func (c *PiWebSearchCheck) ID() CheckID { return PiWebSearchCheckID }

func (c *PiWebSearchCheck) Run(_ context.Context) *Result {
	home, _ := c.homeDirFn()
	candidates := []string{filepath.Join(home, ".pi", "agent", "extensions", "biggz-web-search.js")}
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		candidates = append(candidates, filepath.Join(v, "extensions", "biggz-web-search.js"))
	}
	var foundPath string
	for _, cand := range candidates {
		if info, err := c.statFn(cand); err == nil && !info.IsDir() {
			foundPath = cand
			break
		}
	}
	_ = c.getenv // keep getter for headless check; provider is now default via DuckDuckGo (no key required)
	headless := c.getenv("BIGGZ_WEB_FETCH_HEADLESS") == "1"
	expected := candidates[0]
	if foundPath == "" {
		return &Result{ID: PiWebSearchCheckID, Status: StatusFail, Message: fmt.Sprintf("pi web search extension not found at %s (run: biggz install --agent pi)", expected), Severity: SeverityCritical, Error: "biggz-web-search.js missing"}
	}
	// DuckDuckGo is default (no env gate) — file existence alone means ready; pi-web-search (provider-native) handles web_search separately
	msg := fmt.Sprintf("pi web search ready (%s)", foundPath)
	if c.getenv("TAVILY_API_KEY") != "" {
		msg += " [Tavily]"
	} else if c.getenv("BRAVE_API_KEY") != "" {
		msg += " [Brave]"
	} else {
		msg += " [DuckDuckGo default]"
	}
	if headless {
		msg += " [headless tier enabled]"
	}
	return &Result{ID: PiWebSearchCheckID, Status: StatusPass, Message: msg, Severity: SeverityInfo}
}

func (c *PiWebSearchCheck) Remedy() *Remedy {
	return &Remedy{
		ID:          string(PiWebSearchCheckID),
		Description: "Install pi web search extension (biggz install --agent pi)",
		Action: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			bin := "biggz"
			if exe, err := os.Executable(); err == nil {
				cand := filepath.Join(filepath.Dir(exe), "biggz.exe")
				if _, err := os.Stat(cand); err == nil {
					bin = cand
				} else {
					cand2 := filepath.Join(filepath.Dir(exe), "biggz")
					if _, err := os.Stat(cand2); err == nil {
						bin = cand2
					}
				}
			}
			cmd := exec.CommandContext(ctx, bin, "install", "--agent", "pi")
			platform.EnsureCommandDir(cmd)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("biggz install --agent pi: %w (output: %s)", err, strings.TrimSpace(string(out)))
			}
			return nil
		},
	}
}
