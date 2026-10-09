//go:build windows

package sshx

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const agentPipe = `\\.\pipe\openssh-ssh-agent`

func dialAgent() (io.ReadWriteCloser, error) {
	pipe := os.Getenv("SSH_AUTH_SOCK")
	if pipe == "" {
		pipe = agentPipe
	}
	f, err := os.OpenFile(pipe, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("the Windows ssh-agent service isn't running")
	}
	return f, nil
}

// AgentService reports the state of the "OpenSSH Authentication Agent"
// service.
func AgentService() (running, disabled bool, err error) {
	q, err := exec.Command("sc.exe", "query", "ssh-agent").CombinedOutput()
	if err != nil && len(q) == 0 {
		return false, false, err
	}
	c, _ := exec.Command("sc.exe", "qc", "ssh-agent").CombinedOutput()
	return strings.Contains(string(q), "RUNNING"), strings.Contains(string(c), "DISABLED"), nil
}

// EnableAgentServiceCmd starts the agent service and sets it to start with
// Windows. It opens an administrator prompt.
func EnableAgentServiceCmd() *exec.Cmd {
	script := "Start-Process powershell -Verb RunAs -Wait -ArgumentList " +
		"'-NoProfile','-Command','Set-Service ssh-agent -StartupType Automatic; Start-Service ssh-agent'"
	return exec.Command("powershell", "-NoProfile", "-Command", script)
}
