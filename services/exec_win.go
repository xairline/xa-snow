//go:build windows

package services

import (
	"os/exec"
	"runtime"
	"syscall"
)

func (g *gribService) exec(cmd *exec.Cmd) error {
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		g.Logger.Errorf("Error getting snow depth: %v,%s", err, string(output))
		return err
	}
	return nil
}
