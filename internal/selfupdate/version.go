package selfupdate

import (
	"strconv"
	"strings"
)

// Version is a parsed semantic version. Build metadata is ignored for
// ordering, per semver; pre-release identifiers order below their release.
type Version struct {
	Major, Minor, Patch int
	PreRelease          string
	valid               bool
}

// ParseVersion parses "v1.2.3", "1.2.3", "v1.2.3-rc.1" or "v1.2.3-dev+abc123".
// Unparseable input (notably the "dev" ldflags default) yields valid == false.
func ParseVersion(s string) Version {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return Version{}
	}
	// Strip build metadata: it never affects precedence.
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var pre string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre, s = s[i+1:], s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], PreRelease: pre, valid: true}
}

// Valid reports whether the string parsed as a semantic version.
func (v Version) Valid() bool { return v.valid }

// Compare returns -1 if v < o, 0 if equal, +1 if v > o.
func (v Version) Compare(o Version) int {
	for _, pair := range [][2]int{{v.Major, o.Major}, {v.Minor, o.Minor}, {v.Patch, o.Patch}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case v.PreRelease == o.PreRelease:
		return 0
	case v.PreRelease == "": // release > pre-release
		return 1
	case o.PreRelease == "":
		return -1
	case v.PreRelease < o.PreRelease:
		return -1
	default:
		return 1
	}
}

// IsNewerThan reports whether v is strictly newer than current. A current
// version that does not parse (a "dev" build) is never considered outdated,
// so local builds are not nagged to "upgrade" to an older tagged release.
func (v Version) IsNewerThan(current Version) bool {
	if !v.valid || !current.valid {
		return false
	}
	return v.Compare(current) > 0
}
