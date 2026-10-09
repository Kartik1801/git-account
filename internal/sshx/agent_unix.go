//go:build !windows

package sshx

import (
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"time"
)

func dialAgent() (io.ReadWriteCloser, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("no ssh-agent is running (SSH_AUTH_SOCK isn't set)")
	}
	return net.DialTimeout("unix", sock, 2*time.Second)
}

// AgentService reports the Windows ssh-agent service; elsewhere there is none.
func AgentService() (running, disabled bool, err error) {
	return false, false, errors.New("only on Windows")
}

// EnableAgentServiceCmd is Windows only.
func EnableAgentServiceCmd() *exec.Cmd { return nil }
