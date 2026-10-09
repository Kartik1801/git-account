// Package store is git-account's engine. It changes accounts in
// config.toml and regenerates the git and ssh files that follow from it.
// Every change backs up the files it touches first and can be undone.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kartik-dhawan-dev/git-account/internal/backup"
	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/gitx"
	"github.com/kartik-dhawan-dev/git-account/internal/hosts"
	"github.com/kartik-dhawan-dev/git-account/internal/keys"
	"github.com/kartik-dhawan-dev/git-account/internal/paths"
	"github.com/kartik-dhawan-dev/git-account/internal/sshx"
)

// ErrNotRepo means the folder isn't inside a git repository.
var ErrNotRepo = errors.New("this folder isn't inside a git repository")

type Manager struct {
	P          paths.Paths
	Git        *gitx.Git
	SSH        *sshx.SSH // nil when OpenSSH isn't installed
	Cfg        *config.Config
	GitVersion string
	gitMajor   int
	gitMinor   int
}

// Open loads config.toml and finds git and ssh.
func Open() (*Manager, error) {
	p, err := paths.Default()
	if err != nil {
		return nil, err
	}
	g, err := gitx.Find()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(p.ConfigFile())
	if err != nil {
		return nil, err
	}
	m := &Manager{P: p, Git: g, Cfg: cfg}
	m.SSH, _ = sshx.Find()
	m.gitMajor, m.gitMinor, m.GitVersion, _ = g.Version()
	m.pruneTrash(30 * 24 * time.Hour)
	return m, nil
}

// GitAtLeast reports whether git is at least major.minor.
func (m *Manager) GitAtLeast(major, minor int) bool {
	return m.gitMajor > major || (m.gitMajor == major && m.gitMinor >= minor)
}

// Reload rereads config.toml.
func (m *Manager) Reload() error {
	cfg, err := config.Load(m.P.ConfigFile())
	if err != nil {
		return err
	}
	m.Cfg = cfg
	return nil
}

// Account returns the account called id.
func (m *Manager) Account(id string) (*config.Account, error) {
	a, _ := m.Cfg.Find(id)
	if a == nil {
		return nil, fmt.Errorf("there's no account called %s", id)
	}
	return a, nil
}

func (m *Manager) KeyPath(a *config.Account) string     { return m.P.Expand(a.Key) }
func (m *Manager) Host(a *config.Account) hosts.Host    { return hosts.For(a.Host, a.Hostname) }
func (m *Manager) ProfilePath(a *config.Account) string { return m.P.Profile(a.UID) }

// Target is where connection tests for a go.
func (m *Manager) Target(a *config.Account) sshx.Target {
	user := a.SSHUser
	if user == "" {
		user = "git"
	}
	return sshx.Target{User: user, Host: m.Host(a).Domain, Port: a.Port}
}

// SSHCommand is the core.sshCommand for an account's key.
func (m *Manager) SSHCommand(keyPath string) string { return m.SSH.Command(keyPath) }

// NewKeyPath picks a free file name for a new key of an account.
func (m *Manager) NewKeyPath(id, typ string) string {
	if typ == "" {
		typ = "ed25519"
	}
	dir := m.P.KeyDir(id)
	base := filepath.Join(dir, "id_"+typ)
	if !exists(base) && !exists(base+".pub") {
		return base
	}
	stamped := base + "_" + time.Now().Format("20060102")
	p := stamped
	for i := 2; exists(p) || exists(p+".pub"); i++ {
		p = fmt.Sprintf("%s_%d", stamped, i)
	}
	return p
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// KeyInfo loads an account's key.
func (m *Manager) KeyInfo(a *config.Account) (*keys.Info, error) { return keys.Load(m.KeyPath(a)) }

// Folder rules sorted so deeper folders come later and win.
type rule struct {
	account *config.Account
	folder  string
	abs     string
}

func (m *Manager) rules() []rule {
	var rs []rule
	for i := range m.Cfg.Accounts {
		a := &m.Cfg.Accounts[i]
		for _, f := range a.Folders {
			rs = append(rs, rule{account: a, folder: f, abs: m.P.Expand(f)})
		}
	}
	sort.SliceStable(rs, func(i, j int) bool { return len(rs[i].abs) < len(rs[j].abs) })
	return rs
}

// Here says which account applies in a folder and why.
type Here struct {
	Dir     string
	Repo    string // repository root; "" outside a repository
	Account string // "" when user.email doesn't come from git-account
	Kind    string // repo, folder, default, other, none
	Folder  string // the matching folder when Kind is folder
	File    string // the file setting user.email when Kind is other
	Email   string
}

// Resolve asks git which user.email applies in dir and maps it back to an
// account and the reason.
func (m *Manager) Resolve(dir string) Here {
	h := Here{Dir: dir}
	if top, ok := m.Git.TopLevel(dir); ok {
		h.Repo = top
	}
	val, file, ok, err := m.Git.Origin(dir, "--get", "user.email")
	if err != nil || !ok {
		h.Kind = "none"
		return h
	}
	h.Email = val
	a := m.accountByProfile(file)
	if a == nil {
		h.Kind, h.File = "other", file
		return h
	}
	h.Account = a.ID
	if h.Repo != "" {
		if uid, ok := m.Pinned(dir); ok && uid == a.UID {
			h.Kind = "repo"
			return h
		}
		rs := m.rules()
		for i := len(rs) - 1; i >= 0; i-- {
			if rs[i].account.ID == a.ID && paths.Within(h.Repo, rs[i].abs) {
				h.Kind, h.Folder = "folder", rs[i].folder
				return h
			}
		}
	}
	h.Kind = "default"
	return h
}

func (m *Manager) accountByProfile(file string) *config.Account {
	if file == "" {
		return nil
	}
	for i := range m.Cfg.Accounts {
		if paths.Same(file, m.ProfilePath(&m.Cfg.Accounts[i])) {
			return &m.Cfg.Accounts[i]
		}
	}
	return nil
}

// Pinned returns the uid of the account pinned to the repository at dir.
func (m *Manager) Pinned(dir string) (string, bool) {
	vals, _ := m.Git.GetAll(dir, "--local", "--get-all", "include.path")
	for i := len(vals) - 1; i >= 0; i-- {
		p := filepath.FromSlash(vals[i])
		if paths.Within(p, m.P.ProfilesDir()) {
			return strings.TrimSuffix(filepath.Base(p), ".gitconfig"), true
		}
	}
	return "", false
}

// History lists the changes that can be undone, newest first.
func (m *Manager) History() []*backup.Snapshot {
	list, _ := backup.List(m.P.BackupsDir())
	var out []*backup.Snapshot
	for _, s := range list {
		if s.Done {
			out = append(out, s)
		}
	}
	return out
}

// Interrupted returns a change that never finished, for example because the
// computer lost power halfway through.
func (m *Manager) Interrupted() *backup.Snapshot {
	list, _ := backup.List(m.P.BackupsDir())
	for _, s := range list {
		if !s.Done && time.Since(s.Created) > 10*time.Second {
			return s
		}
	}
	return nil
}

func (m *Manager) pruneTrash(age time.Duration) {
	entries, err := os.ReadDir(m.P.TrashDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > age {
			_ = os.RemoveAll(filepath.Join(m.P.TrashDir(), e.Name()))
		}
	}
}
