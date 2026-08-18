package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Source is how zap was installed on this machine. It decides whether zap may
// replace its own binary: package managers keep their own bookkeeping, so
// overwriting a managed binary would leave the manager out of sync.
type Source string

const (
	SourceHomebrew  Source = "Homebrew"
	SourceScoop     Source = "Scoop"
	SourceGoInstall Source = "go install"
	SourceManual    Source = "manual"
)

// SelfManaged reports whether zap may overwrite its own binary in place.
func (s Source) SelfManaged() bool {
	return s == SourceGoInstall || s == SourceManual
}

// UpgradeHint is the command a user would run by hand when a package manager
// owns the binary. Empty for self-managed installs.
func (s Source) UpgradeHint() string {
	return joinArgv(s.UpgradeCommand())
}

// ExePath returns the fully resolved path to the running zap binary.
func ExePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, rerr := filepath.EvalSymlinks(p); rerr == nil {
		p = resolved
	}
	return p, nil
}

// Detect classifies how the binary at exePath was installed.
//
// Ownership is checked before path shape on purpose: a binary hand-copied into
// /opt/homebrew/bin looks Homebrew-ish but is not managed by brew, and telling
// that user to run "brew upgrade zap" would be dead wrong.
func Detect(exePath string) Source {
	if brewOwns(exePath) {
		return SourceHomebrew
	}
	if scoopOwns(exePath) {
		return SourceScoop
	}
	if gopathOwns(exePath) {
		return SourceGoInstall
	}
	return SourceManual
}

// brewOwns asks brew where its zap lives; path heuristics are only a fallback
// for when the brew CLI is unavailable but the Cellar layout is unmistakable.
func brewOwns(exePath string) bool {
	if brew, err := exec.LookPath("brew"); err == nil {
		out, err := exec.Command(brew, "--prefix", "zap").Output()
		if err == nil {
			prefix := strings.TrimSpace(string(out))
			if prefix != "" && strings.HasPrefix(exePath, prefix) {
				return true
			}
			// brew answered and does not own this path.
			return false
		}
	}
	return strings.Contains(strings.ToLower(exePath), "/cellar/")
}

func scoopOwns(exePath string) bool {
	lower := strings.ToLower(filepath.ToSlash(exePath))
	return strings.Contains(lower, "/scoop/")
}

func gopathOwns(exePath string) bool {
	dir := filepath.Dir(exePath)
	if gobin := os.Getenv("GOBIN"); gobin != "" && sameDir(dir, gobin) {
		return true
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		return sameDir(dir, filepath.Join(gopath, "bin"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		return sameDir(dir, filepath.Join(home, "go", "bin"))
	}
	return false
}

func sameDir(a, b string) bool {
	ra, err1 := filepath.Abs(a)
	rb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return filepath.Clean(ra) == filepath.Clean(rb)
}
