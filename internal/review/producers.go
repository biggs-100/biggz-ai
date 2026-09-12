package review

// Producer parity guard (design D4). Every blocking review surface must ship
// with a producer — the exact command that satisfies it — registered for
// every supported host. Refusals consume this manifest instead of hardcoding
// command strings, and the guard fails whenever a surface or host is missing
// a producer (review spec: "Producer Parity Guard").

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/pathquote"
)

// ProducerSurface identifies a place where the RDD gate blocks publication
// until a review receipt exists: the five publication gate kinds plus the sdd
// verify preflight.
type ProducerSurface string

const (
	// ProducerSurfacePostApply is the post-apply publication gate.
	ProducerSurfacePostApply ProducerSurface = "post-apply"
	// ProducerSurfacePreCommit is the pre-commit publication gate.
	ProducerSurfacePreCommit ProducerSurface = "pre-commit"
	// ProducerSurfacePrePush is the pre-push publication gate.
	ProducerSurfacePrePush ProducerSurface = "pre-push"
	// ProducerSurfacePrePR is the pre-pr publication gate.
	ProducerSurfacePrePR ProducerSurface = "pre-pr"
	// ProducerSurfaceRelease is the release publication gate.
	ProducerSurfaceRelease ProducerSurface = "release"
	// ProducerSurfaceSDDVerify is the sdd verify preflight surface.
	ProducerSurfaceSDDVerify ProducerSurface = "sdd-verify"
)

// SupportedReviewHosts lists every host the review transport supports, in
// ambient preference order (opencode is the default host).
var SupportedReviewHosts = []string{"opencode", "pi"}

// ProducerSubjectToken is the placeholder every producer command carries in
// the manifest for the subject-file path. ResolveProducer interpolates the
// workspace-specific path through pathquote.Quote.
const ProducerSubjectToken = "<subject>"

// BlockingReviewSurfaces returns the RDD gate kinds plus the sdd verify
// preflight: every surface that must have a producer registered for every
// supported host.
func BlockingReviewSurfaces() []ProducerSurface {
	return []ProducerSurface{
		ProducerSurfacePostApply,
		ProducerSurfacePreCommit,
		ProducerSurfacePrePush,
		ProducerSurfacePrePR,
		ProducerSurfaceRelease,
		ProducerSurfaceSDDVerify,
	}
}

// ProducerManifest maps every blocking surface to every supported host and
// the exact producer command that satisfies it (surface × host → command).
// The same producer satisfies every surface; the surface dimension is the
// guard's coverage key, so adding a surface without producers fails until it
// is registered here.
func ProducerManifest() map[ProducerSurface]map[string]string {
	surfaces := BlockingReviewSurfaces()
	manifest := make(map[ProducerSurface]map[string]string, len(surfaces))
	for _, surface := range surfaces {
		hosts := make(map[string]string, len(SupportedReviewHosts))
		for _, host := range SupportedReviewHosts {
			hosts[host] = producerCommandTemplate(surface, host)
		}
		manifest[surface] = hosts
	}
	return manifest
}

// producerCommandTemplate renders the exact producer command one surface
// registers for one supported host. The command starts the review whose
// receipt satisfies the gate; the pi host additionally declares its relay
// agent because pi is only eligible while the relay handshake is valid.
func producerCommandTemplate(surface ProducerSurface, host string) string {
	command := "biggz review start --subject " + ProducerSubjectToken
	if host == "pi" {
		command += " --agent pi"
	}
	return command
}

// ProducerGap names a missing producer: a blocking surface that has no
// producer command registered for a supported host.
type ProducerGap struct {
	Surface ProducerSurface `json:"surface"`
	Host    string          `json:"host,omitempty"`
	Reason  string          `json:"reason"`
}

// ValidateProducerManifest reports every surface × host combination without a
// non-empty producer command. An empty result proves full coverage; adding a
// blocking surface or a supported host fails the guard until every producer
// is registered. The guard test feeds a deliberately incomplete manifest to
// prove it fails (review spec: "Producer Parity Guard").
func ValidateProducerManifest(manifest map[ProducerSurface]map[string]string) []ProducerGap {
	var gaps []ProducerGap
	for _, surface := range BlockingReviewSurfaces() {
		hosts, ok := manifest[surface]
		if !ok || len(hosts) == 0 {
			gaps = append(gaps, ProducerGap{Surface: surface, Reason: "surface has no producer registered for any host"})
			continue
		}
		for _, host := range SupportedReviewHosts {
			if strings.TrimSpace(hosts[host]) == "" {
				gaps = append(gaps, ProducerGap{Surface: surface, Host: host, Reason: "no producer command registered"})
			}
		}
	}
	return gaps
}

// ProducerResolution is the outcome of resolving the producer that satisfies
// one surface on one host.
type ProducerResolution struct {
	Surface    ProducerSurface `json:"surface"`
	Host       string          `json:"host"`
	Command    string          `json:"command,omitempty"`
	Producible bool            `json:"producible"`
}

// ResolveProducer resolves the exact producer command for one blocking
// surface and host, interpolating the quoted subject-file path. A surface or
// host without a registration returns Producible=false: the caller must
// report the typed unproducible state instead of guessing a command (rdd
// spec: unproducible receipts are reported honestly).
func ResolveProducer(surface ProducerSurface, host, subjectPath string) ProducerResolution {
	template := strings.TrimSpace(ProducerManifest()[surface][host])
	if template == "" {
		return ProducerResolution{Surface: surface, Host: host}
	}
	return ProducerResolution{
		Surface:    surface,
		Host:       host,
		Command:    strings.Replace(template, ProducerSubjectToken, pathquote.Quote(subjectPath), 1),
		Producible: true,
	}
}

// CurrentProducerHost reports the review host driving the current runtime.
// The pi host is driving exactly while its relay handshake is declared (the
// biggz-pi host exports it on every invocation it relays); otherwise the
// ambient host is opencode, the default supported host.
func CurrentProducerHost() string {
	if IsPiRelayAvailable() {
		return "pi"
	}
	return SupportedReviewHosts[0]
}
