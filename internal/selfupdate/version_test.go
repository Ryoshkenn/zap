package selfupdate

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
		want  Version
	}{
		{"v1.2.3", true, Version{Major: 1, Minor: 2, Patch: 3}},
		{"1.2.3", true, Version{Major: 1, Minor: 2, Patch: 3}},
		{"v1.2.3-rc.1", true, Version{Major: 1, Minor: 2, Patch: 3, PreRelease: "rc.1"}},
		{"v1.2.0-dev+4dfe35f", true, Version{Major: 1, Minor: 2, Patch: 0, PreRelease: "dev"}},
		{"dev", false, Version{}},
		{"", false, Version{}},
		{"v1.2", false, Version{}},
		{"v1.2.x", false, Version{}},
	}
	for _, c := range cases {
		got := ParseVersion(c.in)
		if got.Valid() != c.valid {
			t.Errorf("ParseVersion(%q).Valid() = %v, want %v", c.in, got.Valid(), c.valid)
			continue
		}
		if !c.valid {
			continue
		}
		if got.Major != c.want.Major || got.Minor != c.want.Minor || got.Patch != c.want.Patch || got.PreRelease != c.want.PreRelease {
			t.Errorf("ParseVersion(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestCompareOrdering(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.3", 0},
		{"v1.2.4", "v1.2.3", 1},
		{"v1.3.0", "v1.2.9", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.2.3", "v1.2.4", -1},
		// A release outranks its own pre-releases.
		{"v1.2.3", "v1.2.3-rc.1", 1},
		{"v1.2.3-rc.1", "v1.2.3", -1},
		{"v1.2.3-rc.2", "v1.2.3-rc.1", 1},
		// Build metadata is ignored for precedence.
		{"v1.2.3+aaa", "v1.2.3+bbb", 0},
	}
	for _, c := range cases {
		if got := ParseVersion(c.a).Compare(ParseVersion(c.b)); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// A local "dev" build must never be told it is out of date — otherwise every
// developer running from source gets nagged to downgrade to the last tag.
func TestDevBuildIsNeverOutdated(t *testing.T) {
	if ParseVersion("v9.9.9").IsNewerThan(ParseVersion("dev")) {
		t.Error("a dev build should not be considered outdated")
	}
}
