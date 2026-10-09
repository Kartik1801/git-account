// Package doctor checks everything that can make git use the wrong account
// or fail to connect, and knows how to fix most of it.
package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kartik-dhawan-dev/git-account/internal/block"
	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
	"github.com/kartik-dhawan-dev/git-account/internal/keys"
	"github.com/kartik-dhawan-dev/git-account/internal/paths"
	"github.com/kartik-dhawan-dev/git-account/internal/sshx"
	"github.com/kartik-dhawan-dev/git-account/internal/store"
	"github.com/kartik-dhawan-dev/git-account/internal/sysx"
)

type Level int

const (
	OK Level = iota
	Info
	Warn
	Problem
)

// Fix repairs one result. Run works in-process; Cmd needs the terminal
// (ssh-add asking for a passphrase, a Windows admin prompt).
type Fix struct {
	Label string
	Run   func() error
	Cmd   func() *exec.Cmd
}

type Result struct {
	Level   Level
	Title   string
	Detail  string
	Account string
	Fix     *Fix
}

// Attention counts results that need a look.
func Attention(rs []Result) int {
	n := 0
	for _, r := range rs {
		if r.Level >= Warn {
			n++
		}
	}
	return n
}

// Sort puts problems first, keeping the order within each level.
func Sort(rs []Result) {
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].Level > rs[j].Level })
}

// Local runs every check that doesn't need the network.
func Local(m *store.Manager) []Result {
	var rs []Result
	add := func(r Result) { rs = append(rs, r) }
	pretty := m.P.Pretty
	sync := &Fix{Label: "Rebuild git-account's files", Run: m.Sync}

	if !m.GitAtLeast(2, 13) {
		add(Result{Level: Problem, Title: "git " + m.GitVersion + " is too old for automatic folders", Detail: "Install git 2.13 or newer."})
	} else {
		add(Result{Level: OK, Title: "git " + m.GitVersion})
	}
	if m.SSH == nil {
		add(Result{Level: Problem, Title: "OpenSSH isn't installed", Detail: sshInstallHint()})
	} else if v, err := m.SSH.Version(); err == nil {
		add(Result{Level: OK, Title: v})
	}
	if s := m.Interrupted(); s != nil {
		add(Result{Level: Problem, Title: "A change didn't finish: " + s.Label,
			Detail: "Some files may be half changed. Restoring puts them back the way they were.",
			Fix:    &Fix{Label: "Restore the files", Run: func() error { return m.Recover(s) }}})
	}
	if len(m.Cfg.Accounts) == 0 {
		add(Result{Level: Info, Title: "No accounts yet", Detail: "Press a on the home screen to add one."})
		return rs
	}

	global := m.P.GlobalGitconfig()
	cur, _, _ := fsx.ReadIfExists(global)
	stale := m.OutOfDate()
	switch {
	case !block.Has(string(cur)):
		add(Result{Level: Problem, Title: pretty(global) + " doesn't include git-account", Detail: "git ignores your accounts until it does.", Fix: sync})
	case len(stale) > 0:
		var names []string
		for _, s := range stale {
			names = append(names, pretty(s))
		}
		add(Result{Level: Warn, Title: "Some of git-account's files are out of date", Detail: strings.Join(names, ", "), Fix: sync})
	default:
		add(Result{Level: OK, Title: pretty(global) + " includes git-account"})
	}

	// Ask git what actually wins, outside any repository.
	outside := os.TempDir()
	if def, _ := m.Cfg.Find(m.Cfg.Default); def != nil && block.Has(string(cur)) {
		if _, file, ok, _ := m.Git.Origin(outside, "--global", "--includes", "--get", "user.email"); ok && !paths.Same(file, m.ProfilePath(def)) {
			add(Result{Level: Problem, Title: "Something after git-account's lines in " + pretty(global) + " sets user.email",
				Detail: "Your default account is ignored while it's there. Moving git-account's lines to the end fixes it.",
				Fix:    &Fix{Label: "Move git-account's lines to the end", Run: m.Sync}})
		}
	}
	if _, file, ok, _ := m.Git.Origin(outside, "--global", "--includes", "--get", "core.sshCommand"); ok && file != "" && !m.IsProfile(file) {
		add(Result{Level: Problem, Title: pretty(file) + " sets core.sshCommand for every repo",
			Detail: "It overrides each account's key. It's often left over from another multi-account tool.",
			Fix:    &Fix{Label: "Remove it", Run: func() error { return m.UnsetIn(file, "core.sshCommand") }}})
	}
	for _, v := range []string{"GIT_SSH_COMMAND", "GIT_SSH"} {
		if os.Getenv(v) != "" {
			add(Result{Level: Warn, Title: v + " is set in your environment", Detail: "It overrides every account's key. Remove it from your shell profile."})
		}
	}
	for _, v := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		if os.Getenv(v) != "" {
			add(Result{Level: Warn, Title: v + " is set in your environment", Detail: "It overrides every account's email. Remove it from your shell profile."})
		}
	}
	seen := map[string]bool{}
	for i := range m.Cfg.Accounts {
		d := m.Host(&m.Cfg.Accounts[i]).Domain
		if seen[d] {
			continue
		}
		seen[d] = true
		for _, o := range sshx.IdentityOverrides(m.P.SSHConfig(), d) {
			add(Result{Level: Warn, Title: pretty(m.P.SSHConfig()) + " gives " + d + " its own key",
				Detail: o + ". If " + d + " refuses an account's key, ssh tries this one next and may sign you in as someone else."})
		}
	}

	var fps map[string]bool
	reachable := false
	if m.SSH != nil {
		fps, reachable = m.SSH.AgentKeys()
	}
	encrypted := 0
	for i := range m.Cfg.Accounts {
		a := m.Cfg.Accounts[i]
		key := m.KeyPath(&a)
		info, err := keys.Load(key)
		if err != nil {
			add(Result{Level: Problem, Account: a.ID, Title: a.ID + ": can't read its key", Detail: err.Error() + ". Replace the key from the account screen."})
			continue
		}
		if info.HasPrivate {
			if ok, err := fsx.IsPrivate(key); err == nil && !ok {
				add(Result{Level: Problem, Account: a.ID, Title: a.ID + ": other users can read the key file",
					Detail: "OpenSSH refuses to use keys like this.",
					Fix:    &Fix{Label: "Make it readable only by you", Run: func() error { return fsx.RestrictToOwner(key) }}})
			}
		}
		if info.Encrypted {
			encrypted++
			if reachable && !fps[info.Fingerprint] && m.SSH != nil {
				ssh := m.SSH
				add(Result{Level: Warn, Account: a.ID, Title: a.ID + "'s key isn't loaded in the agent",
					Detail: "Every push asks for its passphrase until it is.",
					Fix:    &Fix{Label: "Load it (asks for the passphrase)", Cmd: func() *exec.Cmd { return ssh.LoadCmd(key) }}})
			}
		}
		created := a.KeyCreated
		if created.IsZero() {
			created = info.Created
		}
		if !created.IsZero() && time.Since(created) > 365*24*time.Hour {
			add(Result{Level: Info, Account: a.ID, Title: a.ID + "'s key is from " + created.Format("Jan 2006"),
				Detail: "Replace it from the account screen when it suits you. The old key keeps working until the new one is confirmed."})
		}
	}
	if m.SSH != nil {
		switch {
		case reachable:
			add(Result{Level: OK, Title: agentName() + " is running"})
		case encrypted > 0:
			r := Result{Level: Warn, Title: "No ssh-agent is running", Detail: "Keys with a passphrase ask for it on every push."}
			switch runtime.GOOS {
			case "windows":
				_, disabled, _ := sshx.AgentService()
				state := "stopped"
				if disabled {
					state = "disabled"
				}
				r.Detail += " The Windows ssh-agent service is " + state + "."
				r.Fix = &Fix{Label: "Start the service (asks for admin rights)", Cmd: sshx.EnableAgentServiceCmd}
			case "linux":
				r.Detail += ` Start one with: eval "$(ssh-agent -s)"`
			}
			add(r)
		}
	}
	return rs
}

// Connection tests one account against its host.
func Connection(ctx context.Context, m *store.Manager, a config.Account) Result {
	h := m.Host(&a)
	key := m.KeyPath(&a)
	r := m.Test(ctx, &a, key)
	res := Result{Account: a.ID}
	switch {
	case r.OK && a.Username != "" && r.Username != "" && !strings.EqualFold(r.Username, a.Username):
		res.Level = Problem
		res.Title = fmt.Sprintf("%s: this key signs in as @%s, not @%s", a.ID, r.Username, a.Username)
		res.Detail = fmt.Sprintf("It was probably added while you were signed in to the wrong %s account. Remove it there and add it to @%s, or keep @%s as this account's username.", h.Name, a.Username, r.Username)
		user := r.Username
		res.Fix = &Fix{Label: "Use @" + user + " as the username", Run: func() error { return m.Edit(a.ID, a.Name, a.Email, user) }}
	case r.OK:
		res.Level = OK
		res.Title = a.ID + " → " + h.Domain + " accepted the key"
		if r.Username != "" {
			res.Title = a.ID + " → " + h.Domain + " says Hi @" + r.Username
		}
	case r.Kind == "network" && h.Alt443 != "" && !contains(m.Cfg.SSH443, h.Domain):
		if alt := m.Test443(ctx, &a, key); alt.OK {
			res.Level = Problem
			res.Title = fmt.Sprintf("Port 22 to %s is blocked, but port 443 works", h.Domain)
			res.Detail = "Your network blocks the usual SSH port. Git can use port 443 instead, for every account on " + h.Domain + "."
			domain := h.Domain
			res.Fix = &Fix{Label: "Use port 443 for " + domain, Run: func() error { return m.Enable443(domain) }}
		} else {
			res.Level = Problem
			res.Title = a.ID + ": can't reach " + h.Domain
			res.Detail = capitalize(r.Message) + ". Tried ports 22 and 443."
		}
	case r.Kind == "denied":
		res.Level = Problem
		res.Title = a.ID + ": " + h.Domain + " didn't accept the key"
		res.Detail = "Add it on " + h.Name + "'s SSH keys page. The fix copies the key and opens the page."
		res.Fix = &Fix{Label: "Copy the key and open the page", Run: func() error {
			info, err := keys.Load(key)
			if err != nil {
				return err
			}
			if err := sysx.Copy(info.Line()); err != nil {
				return err
			}
			return sysx.OpenURL(h.KeysURL())
		}}
	case r.Kind == "passphrase" && m.SSH != nil:
		ssh := m.SSH
		res.Level = Warn
		res.Title = a.ID + ": couldn't test, the key's passphrase isn't loaded"
		res.Fix = &Fix{Label: "Load it (asks for the passphrase)", Cmd: func() *exec.Cmd { return ssh.LoadCmd(key) }}
	case r.Kind == "hostkey":
		res.Level = Problem
		res.Title = a.ID + ": " + h.Domain + "'s identity changed"
		res.Detail = "ssh refused to connect because the server's key doesn't match the one in known_hosts. Check the host's published fingerprints, then remove the old entry with: ssh-keygen -R " + h.Domain
	default:
		res.Level = Problem
		res.Title = a.ID + ": the test failed"
		res.Detail = capitalize(r.Message)
	}
	return res
}

func agentName() string {
	switch runtime.GOOS {
	case "darwin":
		return "ssh-agent (Keychain)"
	case "windows":
		return "The Windows ssh-agent service"
	}
	return "ssh-agent"
}

func sshInstallHint() string {
	switch runtime.GOOS {
	case "windows":
		return "Install it in Settings > System > Optional features > OpenSSH Client."
	case "darwin":
		return "macOS includes it; check that /usr/bin/ssh exists."
	}
	return "Install your distribution's openssh-client package."
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
