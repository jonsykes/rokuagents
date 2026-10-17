package protocol

import (
	"slices"
	"strconv"
	"strings"
)

// MinFirmware is the oldest display firmware this helper fully supports. Raise
// it only when the helper starts to rely on something newer firmware does: an
// older display keeps working as far as it can, and the helper warns that it
// needs an update.
const MinFirmware = "0.1.0"

// Release is a release version, X.Y.Z.
type Release [3]int

// Compare returns -1, 0 or +1 as v is older than, the same as or newer than w.
func (v Release) Compare(w Release) int { return slices.Compare(v[:], w[:]) }

// ParseVersion parses an X.Y.Z release version, ignoring a leading "v" and any
// suffix after "-" or "+" (pre-release, build metadata, git describe). It
// reports false for anything else, such as a bare commit hash, and for
// 0.0.0: that is what builds outside a release call themselves
// ("0.0.0-dev"), so these versions are unknown, not old.
func ParseVersion(s string) (Release, bool) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != len(Release{}) {
		return Release{}, false
	}
	var v Release
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Release{}, false
		}
		v[i] = n
	}
	return v, v != Release{}
}

// OlderThan reports whether version is a release older than other. A version
// that is not a release (see ParseVersion) is never older: a development
// build is not out of date, and nothing can be said about it.
func OlderThan(version, other string) bool {
	v, vOK := ParseVersion(version)
	w, wOK := ParseVersion(other)
	return vOK && wOK && v.Compare(w) < 0
}
