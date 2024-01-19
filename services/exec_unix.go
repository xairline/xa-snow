//go:build !windows

package services

import "os/exec"

func (g *gribService) exec(cmd *exec.Cmd) error {
	output, err := cmd.CombinedOutput()
	if err != nil {
		g.Logger.Errorf("Error getting snow depth: %v,%s", err, string(output))
		return err
	}
	return nil
}
