//go:build !windows

package runner

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/fabiang/go-xyrun/internal/models"
)

func (a *App) applyOSChildOpts(cmd *exec.Cmd, job *models.Job, envMap map[string]string) error {
	cred := &syscall.Credential{}
	setCred := false

	var uid, gid int

	if job.UID != nil && job.UID != "" {
		uStr := fmt.Sprintf("%v", job.UID)
		var u *user.User
		var err error
		if uidInt, errParse := strconv.Atoi(uStr); errParse == nil {
			u, err = user.LookupId(uStr)
			if err != nil {
				uid = uidInt
			}
		} else {
			u, err = user.Lookup(uStr)
		}

		if u != nil {
			uidInt, _ := strconv.Atoi(u.Uid)
			gidInt, _ := strconv.Atoi(u.Gid)
			uid = uidInt
			gid = gidInt
			envMap["USER"] = u.Username
			envMap["USERNAME"] = u.Username
			envMap["HOME"] = u.HomeDir
		} else {
			if syscall.Getuid() != uid {
				return fmt.Errorf("Plugin Error: User does not exist: %v", job.UID)
			}
		}

		cred.Uid = uint32(uid)
		cred.Gid = uint32(gid)
		setCred = true
	}

	if job.GID != nil && job.GID != "" {
		gStr := fmt.Sprintf("%v", job.GID)
		var g *user.Group
		var err error
		if gidInt, errParse := strconv.Atoi(gStr); errParse == nil {
			g, err = user.LookupGroupId(gStr)
			if err != nil {
				gid = gidInt
			}
		} else {
			g, err = user.LookupGroup(gStr)
		}

		if g != nil {
			gidInt, _ := strconv.Atoi(g.Gid)
			gid = gidInt
		} else {
			return fmt.Errorf("Plugin Error: Group does not exist: %v", job.GID)
		}

		cred.Gid = uint32(gid)
		setCred = true
	}

	if setCred {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = cred
	}
	return nil
}

func (a *App) sendSigTerm(pid int) {
	syscall.Kill(pid, syscall.SIGTERM)
}
