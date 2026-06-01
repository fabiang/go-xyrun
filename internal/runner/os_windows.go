//go:build windows

package runner

import (
	"os/exec"

	"github.com/fabiang/go-xyrun/internal/models"
)

func (a *App) applyOSChildOpts(cmd *exec.Cmd, job *models.Job, envMap map[string]string) error {
	// windows child options empty for now, xyrun.js does windowsHide = true which is done via SysProcAttr
	return nil
}

func (a *App) sendSigTerm(pid int) {
	// Not practically supported on windows
}
