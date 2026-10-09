// Package sshx runs OpenSSH: it builds the ssh command git uses for each
// account, tests connections and talks to ssh-agent.
package sshx

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh/agent"

	"github.com/kartik-dhawan-dev/git-account/internal/block"
)

type SSH struct {
	Bin    string // ssh used for tests and written into core.sshCommand
	AddBin string // the matching ssh-add
	Apple  bool   // Apple's build, which supports UseKeychain
}

// Find picks the ssh to use. On Windows that is the built-in OpenSSH, whose
// ssh-agent service is the one Windows users can enable; Git's bundled ssh
// can't reach that agent.
func Find() (*SSH, error) {
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		dir := filepath.Join(root, "System32", "OpenSSH")
		if _, err := os.Stat(filepath.Join(dir, "ssh.exe")); err == nil {
			return &SSH{Bin: filepath.Join(dir, "ssh.exe"), AddBin: filepath.Join(dir, "ssh-add.exe")}, nil
		}
	}
	bin, err := exec.LookPath("ssh")
	if err != nil {
		return nil, errors.New("OpenSSH isn't installed or isn't on your PATH")
	}
	add, err := exec.LookPath("ssh-add")
	if err != nil {
		add = filepath.Join(filepath.Dir(bin), "ssh-add")
	}
	return &SSH{Bin: bin, AddBin: add, Apple: runtime.GOOS == "darwin" && bin == "/usr/bin/ssh"}, nil
}

// Version returns the output of ssh -V.
func (s *SSH) Version() (string, error) {
	out, err := exec.Command(s.Bin, "-V").CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// shellQuote quotes a word for the POSIX shell git runs core.sshCommand
// with (Git for Windows uses its bundled sh).
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// Command is the core.sshCommand for a key. IdentitiesOnly stops ssh from
// offering other keys from the agent first, which is how people end up
// pushing as the wrong account.
func (s *SSH) Command(keyPath string) string {
	bin := "ssh"
	if runtime.GOOS == "windows" && s != nil && s.Bin != "" {
		bin = shellQuote(filepath.ToSlash(s.Bin))
	}
	parts := []string{bin, "-i", shellQuote(filepath.ToSlash(keyPath)), "-o", "IdentitiesOnly=yes", "-o", "AddKeysToAgent=yes"}
	if s != nil && s.Apple {
		parts = append(parts, "-o", "IgnoreUnknown=UseKeychain", "-o", "UseKeychain=yes")
	}
	return strings.Join(parts, " ")
}

// Target is where a connection test goes.
type Target struct {
	User string
	Host string
	Port int
}

// Test runs `ssh -T` with only this key and returns everything ssh printed.
func (s *SSH) Test(ctx context.Context, keyPath string, t Target) string {
	args := []string{"-T", "-i", keyPath,
		"-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10", "-o", "LogLevel=ERROR"}
	if t.Port > 0 {
		args = append(args, "-p", strconv.Itoa(t.Port))
	}
	args = append(args, t.User+"@"+t.Host)
	cmd := exec.CommandContext(ctx, s.Bin, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "Connection timed out"
	}
	if len(out) == 0 && err != nil {
		return err.Error()
	}
	return string(out)
}

// AgentKeys returns the fingerprints loaded in ssh-agent. reachable is
// false when no agent is running.
func (s *SSH) AgentKeys() (fps map[string]bool, reachable bool) {
	out, err := exec.Command(s.AddBin, "-l", "-E", "sha256").CombinedOutput()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return nil, false
		}
		code = ee.ExitCode()
	}
	switch code {
	case 0:
		fps = map[string]bool{}
		sc := bufio.NewScanner(bytes.NewReader(out))
		for sc.Scan() {
			if f := strings.Fields(sc.Text()); len(f) >= 2 {
				fps[f[1]] = true
			}
		}
		return fps, true
	case 1:
		return map[string]bool{}, true
	default:
		return nil, false
	}
}

// LoadCmd runs ssh-add for a key. It needs the terminal, because ssh-add
// asks for the passphrase itself.
func (s *SSH) LoadCmd(keyPath string) *exec.Cmd {
	var args []string
	if s.Apple {
		args = append(args, "--apple-use-keychain")
	}
	return exec.Command(s.AddBin, append(args, keyPath)...)
}

// AddToAgent hands a key that is already decrypted in memory to the running
// ssh-agent, so a new key works right away without typing its passphrase
// again.
func AddToAgent(priv crypto.PrivateKey, comment string) error {
	conn, err := dialAgent()
	if err != nil {
		return err
	}
	defer conn.Close()
	return agent.NewClient(conn).Add(agent.AddedKey{PrivateKey: priv, Comment: comment})
}

// IdentityOverrides lists IdentityFile lines that ~/.ssh/config applies to
// domain. ssh offers those keys too, so a refused account key can fall
// back to someone else's.
func IdentityOverrides(configPath, domain string) []string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var found []string
	var patterns []string
	inBlock := false
	for _, raw := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		switch line {
		case block.Start:
			inBlock = true
			continue
		case block.End:
			inBlock = false
			continue
		}
		if inBlock || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val := splitDirective(line)
		switch strings.ToLower(key) {
		case "host":
			patterns = strings.Fields(val)
		case "match":
			patterns = nil
		case "identityfile":
			for _, p := range patterns {
				if strings.EqualFold(p, domain) {
					found = append(found, "Host "+strings.Join(patterns, " ")+": IdentityFile "+val)
					break
				}
			}
		}
	}
	return found
}

func splitDirective(line string) (string, string) {
	i := strings.IndexAny(line, " \t=")
	if i < 0 {
		return line, ""
	}
	return line[:i], strings.TrimSpace(strings.TrimLeft(line[i:], " \t="))
}
