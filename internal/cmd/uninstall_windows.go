//go:build windows

package cmd

import (
	"fmt"
	"os/exec"
)

// removeSelf deletes the zap binary. Windows locks a running executable, so we
// can't delete it directly. Instead we spawn a detached cmd that waits briefly
// for this process to exit, then deletes the file.
func removeSelf(path string) error {
	c := exec.Command("cmd", "/C", "ping 127.0.0.1 -n 2 >nul & del /F /Q "+`"`+path+`"`)
	if err := c.Start(); err != nil {
		return err
	}
	fmt.Println("(the binary will be deleted once zap exits)")
	return nil
}
