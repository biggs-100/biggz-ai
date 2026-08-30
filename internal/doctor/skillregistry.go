package doctor

import (
	"context"

	"github.com/biggs-100/biggz-ai/internal/skillregistry"
)

const SkillRegistryCheckID CheckID = "skill-registry"

type SkillRegistryCheck struct {
	isPolling  func() bool
	isWatching func() bool
}

func NewSkillRegistryCheck() *SkillRegistryCheck {
	return &SkillRegistryCheck{isPolling: skillregistry.IsPolling, isWatching: skillregistry.IsWatching}
}
func NewSkillRegistryCheckWithCustom(pollFn, watchFn func() bool) *SkillRegistryCheck {
	if pollFn == nil {
		pollFn = func() bool { return false }
	}
	if watchFn == nil {
		watchFn = func() bool { return false }
	}
	return &SkillRegistryCheck{isPolling: pollFn, isWatching: watchFn}
}
func (c *SkillRegistryCheck) ID() CheckID { return SkillRegistryCheckID }
func (c *SkillRegistryCheck) Run(ctx context.Context) *Result {
	if c.isPolling != nil && c.isPolling() {
		return &Result{ID: SkillRegistryCheckID, Status: StatusWarn, Message: "skill registry watcher fallback poll active (fsnotify unavailable)", Severity: SeverityWarning}
	}
	if c.isWatching != nil && c.isWatching() {
		return &Result{ID: SkillRegistryCheckID, Status: StatusPass, Message: "skill registry watcher active", Severity: SeverityInfo}
	}
	return &Result{ID: SkillRegistryCheckID, Status: StatusPass, Message: "skill registry watcher idle (gated or no poll)", Severity: SeverityInfo}
}
func (c *SkillRegistryCheck) Remedy() *Remedy { return nil }
