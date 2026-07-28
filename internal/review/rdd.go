// Package review — RDD (Review-Driven Development) kill-switch.
package review

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type RDDMode string

const (
	RDDModeUnset    RDDMode = ""
	RDDModeEnabled  RDDMode = "enabled"
	RDDModeDisabled RDDMode = "disabled"
)

type RDDStatusReport struct {
	Schema        string  `json:"schema"`
	EffectiveMode RDDMode `json:"effective_mode"`
	GlobalMode    RDDMode `json:"global_mode"`
	CloneMode     RDDMode `json:"clone_mode"`
}

const rddStatusSchema = "biggz-ai.rdd-status/v1"
const rddStateFile = "rdd-mode.json"

type rddState struct {
	Schema string  `json:"schema"`
	Mode   RDDMode `json:"mode"`
}

// RDDStatus returns the effective mode. Any "off" wins.
func RDDStatus(cloneGitDir string) (*RDDStatusReport, error) {
	globalMode := readGlobalMode()
	cloneMode := readCloneMode(cloneGitDir)
	effective := RDDModeEnabled
	if globalMode == RDDModeDisabled || cloneMode == RDDModeDisabled {
		effective = RDDModeDisabled
	}
	return &RDDStatusReport{
		Schema: rddStatusSchema, EffectiveMode: effective,
		GlobalMode: globalMode, CloneMode: cloneMode,
	}, nil
}

// RDDEnable enables globally.
func RDDEnable(cloneGitDir string) (*RDDStatusReport, error) {
	if err := writeGlobalMode(RDDModeEnabled); err != nil {
		return nil, err
	}
	return RDDStatus(cloneGitDir)
}

// RDDDisable disables globally or clone-local.
func RDDDisable(cloneGitDir string) (*RDDStatusReport, error) {
	if cloneGitDir != "" {
		if err := writeCloneMode(cloneGitDir, RDDModeDisabled); err != nil {
			return nil, err
		}
	} else {
		if err := writeGlobalMode(RDDModeDisabled); err != nil {
			return nil, err
		}
	}
	return RDDStatus(cloneGitDir)
}

func globalStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".biggz")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, rddStateFile), nil
}

func readGlobalMode() RDDMode {
	p, err := globalStatePath()
	if err != nil {
		return RDDModeUnset
	}
	return readFile(p)
}

func writeGlobalMode(m RDDMode) error {
	p, err := globalStatePath()
	if err != nil {
		return err
	}
	return writeFile(p, m)
}

func readCloneMode(gitDir string) RDDMode {
	if gitDir == "" {
		return RDDModeUnset
	}
	return readFile(filepath.Join(gitDir, rddStateFile))
}

func writeCloneMode(gitDir string, m RDDMode) error {
	os.MkdirAll(gitDir, 0755)
	return writeFile(filepath.Join(gitDir, rddStateFile), m)
}

func readFile(path string) RDDMode {
	data, err := os.ReadFile(path)
	if err != nil {
		return RDDModeUnset
	}
	var s rddState
	if json.Unmarshal(data, &s) != nil {
		return RDDModeUnset
	}
	if s.Mode != RDDModeEnabled && s.Mode != RDDModeDisabled {
		return RDDModeUnset
	}
	return s.Mode
}

func writeFile(path string, m RDDMode) error {
	data, _ := json.MarshalIndent(rddState{Schema: rddStatusSchema, Mode: m}, "", "  ")
	return os.WriteFile(path, data, 0644)
}
