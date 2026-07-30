// Package update implements the update engine for biggz-ai.
//
// It discovers, verifies, and downloads releases from GitHub, supporting
// stable and beta channels via the BIGGZ_CHANNEL environment variable.
package update

import (
	"os"
	"strings"
)

// Channel represents the release channel selection.
type Channel int

const (
	// ChannelStable selects the latest stable (non-prerelease) release.
	ChannelStable Channel = iota
	// ChannelBeta includes prerelease versions when selecting the latest release.
	ChannelBeta
)

// ParseChannel reads the BIGGZ_CHANNEL environment variable and returns
// the corresponding Channel. Defaults to ChannelStable when the variable
// is unset or contains an unrecognized value.
func ParseChannel() Channel {
	switch strings.ToLower(os.Getenv("BIGGZ_CHANNEL")) {
	case "beta", "prerelease":
		return ChannelBeta
	default:
		return ChannelStable
	}
}

// SelectRelease picks the latest release from releases based on the channel.
//
// For ChannelStable, it returns the most recent non-prerelease release.
// For ChannelBeta, it returns the most recent release including prereleases.
// Returns nil when releases is empty.
func SelectRelease(releases []Release, ch Channel) *Release {
	if len(releases) == 0 {
		return nil
	}

	if ch == ChannelBeta {
		return &releases[0]
	}

	for i := range releases {
		if !releases[i].Prerelease {
			return &releases[i]
		}
	}

	// Fallback: no stable release found, return the latest anyway.
	return &releases[0]
}
