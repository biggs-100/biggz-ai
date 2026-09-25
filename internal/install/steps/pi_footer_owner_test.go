package steps

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// footerSlotName is the identifier an extension names to take over pi's one
// footer slot.
const footerSlotName = "setFooter"

// footerSlotMention matches that identifier as a whole word, so a
// `setFooterThrottle` helper never counts as a registration.
var footerSlotMention = regexp.MustCompile(`\b` + footerSlotName + `\b`)

// TestPiFooterOwner_SingleDeployedFooter is the unit-2.4 invariant guard: after
// the port, exactly one deployed pi extension registers the footer slot — the
// new shell extension, which paints the ported shell bar — the retired
// powerline footer is gone from the deploy list and listed for removal from an
// already-installed home, and no deployed asset reads a statusLine* theme key
// that the shell bar does not resolve.
//
// The invariant is a correctness property, not a style preference: pi has one
// footer slot, so two deployed extensions calling ctx.ui.setFooter race for it
// and the one that loads last silently wins.
func TestPiFooterOwner_SingleDeployedFooter(t *testing.T) {
	deployed := piExtensionsDeployList()
	if len(deployed) == 0 {
		t.Fatal("piExtensionsDeployList() is empty: the footer-owner guard would scan nothing, so a second footer owner could ship unnoticed")
	}

	// (a) exactly one deployed footer owner, and (b) it is the shell extension.
	var owners []string
	shellFactory := false
	for _, entry := range deployed {
		data, err := fs.ReadFile(assets.FS, entry.asset)
		if err != nil {
			t.Fatalf("read deployed asset %s: %v", entry.asset, err)
		}
		if registersFooterSlot(string(data)) {
			owners = append(owners, entry.asset)
		}
		if entry.asset == "pi/biggz-shell.ts" && entry.factory {
			shellFactory = true
		}
	}
	if len(owners) != 1 {
		t.Errorf("expected exactly one deployed pi extension to name %s (the footer slot), found %d: %v\n"+
			"pi has one footer slot, so two deployed owners race and the one that loads last silently wins; deploy exactly the unit-2.4 shell extension (pi/biggz-shell.ts) and remove every other footer registration from piExtensionsDeployList",
			footerSlotName, len(owners), owners)
	} else if owners[0] != "pi/biggz-shell.ts" {
		t.Errorf("the deployed footer owner is %s, not pi/biggz-shell.ts\n"+
			"unit 2.4 moves footer ownership to the ported shell extension; biggz-footer.js is retired and kept in the repository only as the rollback reference", owners[0])
	}
	if !shellFactory {
		t.Errorf("pi/biggz-shell.ts is not deployed as a factory: pi loads only direct extensions/*.ts entries exporting `export default function(pi)`, so the shell extension must be deployed with factory: true")
	}

	// (c) the deployed footer path must not depend on statusLine* keys.
	for _, entry := range deployed {
		data, err := fs.ReadFile(assets.FS, entry.asset)
		if err != nil {
			t.Fatalf("read deployed asset %s: %v", entry.asset, err)
		}
		for _, m := range statusLineKeyPattern.FindAllStringSubmatch(string(data), -1) {
			t.Errorf("deployed asset %s references the statusLine* theme key %q, which the shell bar does not resolve\n"+
				"the deployed footer path must paint through theme roles the seven shipped biggz themes define (the shell bar's ROLE palette); rename the key or retire the asset from piExtensionsDeployList", entry.asset, m[1])
		}
	}

	// (d) retirement must be complete in both directions.
	retired := make(map[string]bool, len(retiredPiExtensionTargets))
	for _, target := range retiredPiExtensionTargets {
		retired[target] = true
	}
	if !retired["biggz-footer.js"] {
		t.Errorf("retiredPiExtensionTargets does not contain biggz-footer.js\n" +
			"removing the footer from the deploy list is not enough: an already-installed home keeps the retired copy, so Apply must also delete it there")
	}
	for _, entry := range deployed {
		if retired[entry.target] {
			t.Errorf("retired target %s is still in piExtensionsDeployList()\n"+
				"a target cannot be both deployed and retired; either drop it from the deploy list or remove it from retiredPiExtensionTargets", entry.target)
		}
	}
}

// registersFooterSlot reports whether source names the footer slot on a line
// that is not comment-only.
//
// The check is deliberately wider than the direct `ctx.ui.setFooter(...)` call
// this repository writes today, because the invariant is "one deployed owner",
// not "one deployed call site in the shape someone happened to write": a
// bracket form `ctx.ui["setFooter"](...)`, a destructured or aliased reference,
// and a registration whose parenthesis sits on the next line all take the same
// slot and all keep the identifier on a non-comment line. A mention inside a
// `/* */` block or after a trailing `//` counts as a registration too: that
// fails loudly, which is the direction this guard must err in. Whole-line `//`
// and `*` lines are skipped so prose about the slot is not read as code.
func registersFooterSlot(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		if footerSlotMention.MatchString(line) {
			return true
		}
	}
	return false
}

// TestRegistersFooterSlot_CatchesEveryRegistrationShape pins the width of the
// scanner above. The first three cases are the shapes a literal `.setFooter(`
// substring scan missed while the guard reported coverage.
func TestRegistersFooterSlot_CatchesEveryRegistrationShape(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   bool
	}{
		{"direct call", `ctx.ui.setFooter((tui, theme, data) => bar(tui, theme, data));`, true},
		{"bracket form", `ctx.ui["setFooter"]((tui, theme, data) => bar(tui, theme, data));`, true},
		{"aliased reference", `const take = ctx.ui.setFooter; take((tui, theme, data) => bar(tui, theme, data));`, true},
		{"split call", "ctx.ui.setFooter\n\t((tui, theme, data) => bar(tui, theme, data));", true},
		{"trailing comment still counts", `installBar(); // ctx.ui.setFooter(cb)`, true},
		{"comment-only line is prose", "\t// this asset never calls ctx.ui.setFooter(cb).", false},
		{"block comment line is prose", "\t * the setFooter slot is owned elsewhere.", false},
		{"unrelated identifier", `ctx.ui.setFooterThrottle(50);`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := registersFooterSlot(tc.source); got != tc.want {
				t.Errorf("registersFooterSlot(%q) = %v, want %v: a scanner narrower than the invariant reports coverage it does not have", tc.source, got, tc.want)
			}
		})
	}
}
