//go:build !windows

package cmd

import "os"

// removeSelf deletes the zap binary. On Unix a running executable can be
// unlinked while it's still executing, so this works even mid-command.
func removeSelf(path string) error {
	return os.Remove(path)
}
