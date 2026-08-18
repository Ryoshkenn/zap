package selfupdate

import "testing"

// A binary hand-copied into /opt/homebrew/bin is NOT brew-managed. The old
// path-substring check in uninstall.go called this Homebrew, which would send
// the user to "brew upgrade zap" for a formula brew has never heard of.
func TestManualCopyInsideHomebrewPrefixIsNotHomebrew(t *testing.T) {
	if got := Detect("/opt/homebrew/bin/zap"); got == SourceHomebrew {
		t.Errorf("Detect(/opt/homebrew/bin/zap) = %v; brew does not own this binary", got)
	}
}

func TestCellarPathIsHomebrew(t *testing.T) {
	if got := Detect("/opt/homebrew/Cellar/zap/1.1.0/bin/zap"); got != SourceHomebrew {
		t.Errorf("Detect(Cellar path) = %v, want %v", got, SourceHomebrew)
	}
}

func TestSelfManagedGating(t *testing.T) {
	if SourceHomebrew.SelfManaged() || SourceScoop.SelfManaged() {
		t.Error("package-managed installs must never self-replace their binary")
	}
	if !SourceManual.SelfManaged() || !SourceGoInstall.SelfManaged() {
		t.Error("manual and go-install builds should be self-updatable")
	}
	if SourceHomebrew.UpgradeHint() != "brew upgrade zap" {
		t.Errorf("unexpected homebrew hint %q", SourceHomebrew.UpgradeHint())
	}
}
