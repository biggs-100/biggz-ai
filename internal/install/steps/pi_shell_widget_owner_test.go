package steps

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// The identifiers unit 2.5 introduces. They are the text a TypeScript asset can
// only be met as, so they are baked in rather than imported: renaming either
// constant must force this guard to be updated deliberately instead of silently
// stopping to check anything.
const (
	// headerWidgetKeyLiteral is ctx.ui.setWidget's key for the below-editor
	// header row. pi scopes widgets by key, so a second asset registering the
	// same key would replace the owner's row with no error.
	headerWidgetKeyLiteral = "biggz-shell-below-input-header"
	// usageCommandLiteral is the /usage command the header row's usage segment
	// points at.
	usageCommandLiteral = "biggz-usage"
)

// headerSlotName is the identifier that claims pi's one header slot, the way
// setFooter claims the footer slot. It is deliberately different from the
// widget key above: setWidget("...", { placement: "belowEditor" }) is a keyed,
// coexisting slot, while setHeader is a single last-writer-wins slot.
const headerSlotName = "setHeader"

// headerSlotMention matches the identifier as a whole word, so a
// `setHeaderThrottle` helper never counts as a claim.
var headerSlotMention = regexp.MustCompile(`\b` + headerSlotName + `\b`)

// TestPiShellWidget_Owners pins the single-owner invariants unit 2.5 establishes
// when the shell factory takes the below-editor header widget and the usage
// command:
//
//	(a) exactly one deployed asset names the header widget key, and it is the
//	    shell factory — a duplicated key means one owner silently replaces the
//	    other;
//	(b) exactly one deployed asset registers the usage command name;
//	(c) NO deployed asset names setHeader: the startup banner is not part of
//	    this port, and pi's header slot is a single last-writer-wins slot, so
//	    claiming it must be a conscious decision that updates this guard;
//	(d) the deployed set still has exactly one footer owner, checked through the
//	    unit-2.4 guard's own scanner rather than a second one.
func TestPiShellWidget_Owners(t *testing.T) {
	deployed := piExtensionsDeployList()
	if len(deployed) == 0 {
		t.Fatal("piExtensionsDeployList() is empty: this guard would scan nothing, so a second widget or command owner could ship unnoticed")
	}

	// (a) the header widget key has exactly one owner.
	var keyOwners []string
	for _, entry := range deployed {
		data, err := fs.ReadFile(assets.FS, entry.asset)
		if err != nil {
			t.Fatalf("read deployed asset %s: %v", entry.asset, err)
		}
		if mentionsOnCodeLine(string(data), headerWidgetKeyLiteral) {
			keyOwners = append(keyOwners, entry.asset)
		}
	}
	if len(keyOwners) != 1 {
		t.Errorf("expected exactly one deployed pi asset to name the header widget key %q, found %d: %v\n"+
			"pi scopes widgets by key, so two assets registering the same key race and the one that loads last silently replaces the other's row; register the below-editor header widget from pi/biggz-shell.ts (unit 2.5) and only from there",
			headerWidgetKeyLiteral, len(keyOwners), keyOwners)
	} else if keyOwners[0] != "pi/biggz-shell.ts" {
		t.Errorf("the deployed header widget owner is %s, not pi/biggz-shell.ts\n"+
			"unit 2.5 registers the below-editor header row and the usage command from the shell factory; move the registration there", keyOwners[0])
	}

	// (b) the usage command name has exactly one owner.
	var commandOwners []string
	for _, entry := range deployed {
		data, err := fs.ReadFile(assets.FS, entry.asset)
		if err != nil {
			t.Fatalf("read deployed asset %s: %v", entry.asset, err)
		}
		if mentionsOnCodeLine(string(data), usageCommandLiteral) {
			commandOwners = append(commandOwners, entry.asset)
		}
	}
	if len(commandOwners) != 1 {
		t.Errorf("expected exactly one deployed pi asset to name the usage command %q, found %d: %v\n"+
			"a duplicate command registration is silently resolved by load order, so the command the header row's usage segment points at must have exactly one owner; register it from pi/biggz-shell.ts (unit 2.5)",
			usageCommandLiteral, len(commandOwners), commandOwners)
	} else if commandOwners[0] != "pi/biggz-shell.ts" {
		t.Errorf("the deployed usage command owner is %s, not pi/biggz-shell.ts\n"+
			"unit 2.5 registers the full-table usage command from the shell factory; move the registration there", commandOwners[0])
	}

	// (c) pi's header slot stays unclaimed.
	for _, entry := range deployed {
		data, err := fs.ReadFile(assets.FS, entry.asset)
		if err != nil {
			t.Fatalf("read deployed asset %s: %v", entry.asset, err)
		}
		if namesHeaderSlot(string(data)) {
			t.Errorf("deployed asset %s names %s (pi's one header slot)\n"+
				"the startup banner is not part of this port: pi's header slot is a single last-writer-wins slot with no owner in biggz, so claiming it must be a conscious decision; drop the call or extend this guard with the unit that ports the banner",
				entry.asset, headerSlotName)
		}
	}

	// (d) the footer slot keeps exactly one owner. Reuse the unit-2.4 scanner so
	// the two guards can never disagree about what a registration looks like.
	var footerOwners []string
	for _, entry := range deployed {
		data, err := fs.ReadFile(assets.FS, entry.asset)
		if err != nil {
			t.Fatalf("read deployed asset %s: %v", entry.asset, err)
		}
		if registersFooterSlot(string(data)) {
			footerOwners = append(footerOwners, entry.asset)
		}
	}
	if len(footerOwners) != 1 {
		t.Errorf("expected exactly one deployed pi extension to name %s (the footer slot), found %d: %v\n"+
			"the header widget must not disturb footer ownership: pi has one footer slot, so two deployed owners race and the one that loads last silently wins; keep the unit-2.4 owner (pi/biggz-shell.ts) and remove every other footer registration",
			footerSlotName, len(footerOwners), footerOwners)
	} else if footerOwners[0] != "pi/biggz-shell.ts" {
		t.Errorf("the deployed footer owner is %s, not pi/biggz-shell.ts\n"+
			"unit 2.4 handed the footer slot to the shell factory; restore that owner", footerOwners[0])
	}
}

// mentionsOnCodeLine reports whether source names needle on a line that is not
// comment-only. It uses the same line-local comment skip as
// registersFooterSlot: a whole-line `//` or `*` line is prose, while a mention
// after a trailing `//` still counts, which is the direction this guard must
// err in.
func mentionsOnCodeLine(source, needle string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}

// namesHeaderSlot reports whether source claims pi's header slot on a
// non-comment line, matching the identifier as a whole word.
func namesHeaderSlot(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		if headerSlotMention.MatchString(line) {
			return true
		}
	}
	return false
}
