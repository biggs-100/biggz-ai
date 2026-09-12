package review

// Producer parity guard tests (task 5.2, design D4; review spec "Producer
// Parity Guard"): a blocking review surface must not ship without a producer
// for every supported host, the full manifest passes, and the OpenCode plugin
// actually wires capture.

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/pathquote"
)

func TestRDDParity(t *testing.T) {
	t.Run("FullCoveragePasses", func(t *testing.T) {
		manifest := ProducerManifest()
		if gaps := ValidateProducerManifest(manifest); len(gaps) != 0 {
			t.Fatalf("full manifest must pass the guard, got gaps: %+v", gaps)
		}
		surfaces := BlockingReviewSurfaces()
		if len(surfaces) != 6 {
			t.Fatalf("expected the five gate kinds plus the sdd verify preflight, got %d: %v", len(surfaces), surfaces)
		}
		for _, surface := range surfaces {
			hosts, ok := manifest[surface]
			if !ok {
				t.Fatalf("surface %q missing from the manifest", surface)
			}
			for _, host := range SupportedReviewHosts {
				command := strings.TrimSpace(hosts[host])
				if command == "" {
					t.Fatalf("surface %q host %q has no producer command", surface, host)
				}
				if !strings.HasPrefix(command, "biggz review start --subject") {
					t.Fatalf("surface %q host %q producer must start the review from the subject file, got %q", surface, host, command)
				}
				if !strings.Contains(command, ProducerSubjectToken) {
					t.Fatalf("surface %q host %q producer must carry the subject placeholder, got %q", surface, host, command)
				}
				if strings.Contains(command, "--lineage") {
					t.Fatalf("producer for %q/%q must not embed a lineage id (design D1), got %q", surface, host, command)
				}
			}
		}
	})

	t.Run("MissingProducerFailsGuard", func(t *testing.T) {
		// A supported host with no registered producer fails the guard.
		brokenHost := maps.Clone(ProducerManifest())
		brokenHost[ProducerSurfacePrePush] = maps.Clone(brokenHost[ProducerSurfacePrePush])
		delete(brokenHost[ProducerSurfacePrePush], "pi")
		gaps := ValidateProducerManifest(brokenHost)
		if len(gaps) != 1 || gaps[0].Surface != ProducerSurfacePrePush || gaps[0].Host != "pi" {
			t.Fatalf("missing pi producer must fail the guard for pre-push, got: %+v", gaps)
		}

		// A blocking surface with no registration at all fails the guard.
		brokenSurface := maps.Clone(ProducerManifest())
		delete(brokenSurface, ProducerSurfaceSDDVerify)
		gaps = ValidateProducerManifest(brokenSurface)
		if len(gaps) != 1 || gaps[0].Surface != ProducerSurfaceSDDVerify {
			t.Fatalf("missing blocking surface must fail the guard, got: %+v", gaps)
		}

		// A registered-but-empty producer command fails the guard.
		brokenEmpty := maps.Clone(ProducerManifest())
		brokenEmpty[ProducerSurfaceRelease] = maps.Clone(brokenEmpty[ProducerSurfaceRelease])
		brokenEmpty[ProducerSurfaceRelease]["opencode"] = "   "
		gaps = ValidateProducerManifest(brokenEmpty)
		if len(gaps) != 1 || gaps[0].Surface != ProducerSurfaceRelease || gaps[0].Host != "opencode" {
			t.Fatalf("empty producer command must fail the guard, got: %+v", gaps)
		}
	})

	t.Run("ResolveProducerConsumesManifest", func(t *testing.T) {
		subject := filepath.Join("ws", "openspec", "changes", "c", "review-subject.json")

		// An unregistered host is unproducible: the caller must report the
		// typed unproducible state, never guess a command.
		if resolution := ResolveProducer(ProducerSurfaceSDDVerify, "unknown-host", subject); resolution.Producible {
			t.Fatalf("unregistered host must be unproducible, got: %+v", resolution)
		}

		// opencode resolves the exact command with the quoted subject path.
		resolution := ResolveProducer(ProducerSurfaceSDDVerify, "opencode", subject)
		want := "biggz review start --subject " + pathquote.Quote(subject)
		if !resolution.Producible || resolution.Command != want {
			t.Fatalf("opencode resolution = %+v, want command %q", resolution, want)
		}

		// pi resolves its relay-gated variant of the same producer.
		pi := ResolveProducer(ProducerSurfacePrePush, "pi", subject)
		if !pi.Producible || !strings.HasSuffix(pi.Command, " --agent pi") {
			t.Fatalf("pi resolution = %+v, want the --agent pi producer", pi)
		}
	})

	t.Run("PluginWiresCapture", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("..", "assets", "opencode", "plugins", "review-result-artifacts.ts"))
		if err != nil {
			t.Fatalf("read the OpenCode review transport plugin: %v", err)
		}
		plugin := string(raw)
		for _, marker := range []string{
			`"capture-result"`,
			`"--input", "-"`,
			`"--preflight"`,
			`"tool.execute.after"`,
			"GENTLE_AI_REVIEW_BINDING",
			"preserved-results",
			"reviewer_preserve_budget_exhausted",
		} {
			if !strings.Contains(plugin, marker) {
				t.Fatalf("the plugin must wire capture; missing marker %q", marker)
			}
		}
	})
}
