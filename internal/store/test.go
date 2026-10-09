package store

import (
	"context"
	"time"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/hosts"
	"github.com/kartik-dhawan-dev/git-account/internal/keys"
	"github.com/kartik-dhawan-dev/git-account/internal/sshx"
)

// Test connects to the account's host with keyPath and nothing else.
func (m *Manager) Test(ctx context.Context, a *config.Account, keyPath string) hosts.Result {
	return m.test(ctx, keyPath, m.Target(a))
}

// Test443 tries the host's port-443 server instead of port 22.
func (m *Manager) Test443(ctx context.Context, a *config.Account, keyPath string) hosts.Result {
	alt := m.Host(a).Alt443
	if alt == "" {
		return hosts.Result{Kind: "error", Message: "this host has no port 443 server"}
	}
	t := m.Target(a)
	t.Host, t.Port = alt, 443
	return m.test(ctx, keyPath, t)
}

func (m *Manager) test(ctx context.Context, keyPath string, t sshx.Target) hosts.Result {
	if m.SSH == nil {
		return hosts.Result{Kind: "error", Message: "OpenSSH isn't installed"}
	}
	info, err := keys.Load(keyPath)
	if err != nil {
		return hosts.Result{Kind: "error", Message: err.Error()}
	}
	if info.Encrypted {
		if fps, ok := m.SSH.AgentKeys(); !ok || !fps[info.Fingerprint] {
			return hosts.Result{Kind: "passphrase", Message: "the key has a passphrase and isn't loaded in the agent"}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return hosts.Parse(m.SSH.Test(ctx, keyPath, t))
}

// IsProfile reports whether file is one of git-account's account files.
func (m *Manager) IsProfile(file string) bool { return m.accountByProfile(file) != nil }
