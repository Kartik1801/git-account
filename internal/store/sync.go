package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kartik-dhawan-dev/git-account/internal/block"
	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
	"github.com/kartik-dhawan-dev/git-account/internal/gitx"
	"github.com/kartik-dhawan-dev/git-account/internal/hosts"
	"github.com/kartik-dhawan-dev/git-account/internal/paths"
)

// Generated returns every file git-account builds from config.toml.
func (m *Manager) Generated() map[string][]byte {
	out := map[string][]byte{}
	for i := range m.Cfg.Accounts {
		a := &m.Cfg.Accounts[i]
		out[m.ProfilePath(a)] = m.renderProfile(a)
	}
	out[m.P.ManagedGitconfig()] = m.renderManaged()
	return out
}

func (m *Manager) renderProfile(a *config.Account) []byte {
	return gitx.Render(
		[]string{
			fmt.Sprintf("Managed by git-account for the account %q.", a.ID),
			"Changes here are overwritten. Run `git-account` to change it.",
		},
		[]gitx.Section{
			{Name: "user", Entries: []gitx.Entry{{Key: "name", Value: a.Name}, {Key: "email", Value: a.Email}}},
			{Name: "core", Entries: []gitx.Entry{{Key: "sshCommand", Value: m.SSHCommand(m.KeyPath(a))}}},
		})
}

func gitdirPrefix() string {
	if paths.CaseInsensitive() {
		return "gitdir/i:"
	}
	return "gitdir:"
}

// folderPattern is the includeIf pattern for a folder: everything inside it.
func folderPattern(abs string) string {
	return gitdirPrefix() + strings.TrimSuffix(gitx.Slash(fsx.LongPath(abs)), "/") + "/"
}

func (m *Manager) renderManaged() []byte {
	var secs []gitx.Section
	if a, _ := m.Cfg.Find(m.Cfg.Default); a != nil {
		secs = append(secs, gitx.Section{
			Comment: "Default account: " + a.ID,
			Name:    "include",
			Entries: []gitx.Entry{{Key: "path", Value: gitx.Slash(m.ProfilePath(a))}},
		})
	}
	for _, r := range m.rules() {
		secs = append(secs, gitx.Section{
			Comment: r.account.ID + ": automatic in " + r.folder,
			Name:    "includeIf",
			Sub:     folderPattern(r.abs),
			Entries: []gitx.Entry{{Key: "path", Value: gitx.Slash(m.ProfilePath(r.account))}},
		})
	}
	return gitx.Render([]string{"Managed by git-account. Changes here are overwritten."}, secs)
}

// OutOfDate lists generated files that don't match config.toml.
func (m *Manager) OutOfDate() []string {
	var stale []string
	for path, want := range m.Generated() {
		if cur, ok, _ := fsx.ReadIfExists(path); !ok || !bytes.Equal(cur, want) {
			stale = append(stale, path)
		}
	}
	if len(m.Cfg.Accounts) > 0 {
		cur, _, _ := fsx.ReadIfExists(m.P.GlobalGitconfig())
		if !block.Has(string(cur)) {
			stale = append(stale, m.P.GlobalGitconfig())
		}
	}
	return stale
}

func (tx *Tx) sync() error {
	m := tx.m
	files := m.Generated()
	for path, data := range files {
		if err := tx.Write(path, data, false); err != nil {
			return err
		}
	}
	if entries, err := os.ReadDir(m.P.ProfilesDir()); err == nil {
		for _, e := range entries {
			p := filepath.Join(m.P.ProfilesDir(), e.Name())
			if _, keep := files[p]; !keep && strings.HasSuffix(e.Name(), ".gitconfig") {
				if err := tx.Remove(p, false); err != nil {
					return err
				}
			}
		}
	}
	if err := tx.globalInclude(); err != nil {
		return err
	}
	return tx.sshBlock()
}

// globalInclude keeps one block at the end of your global git config that
// includes git-account's file. Being last is what lets it win over a
// [user] section you already have.
func (tx *Tx) globalInclude() error {
	path := tx.m.P.GlobalGitconfig()
	cur, _, err := fsx.ReadIfExists(path)
	if err != nil {
		return err
	}
	next, err := block.Append(string(cur), []string{
		"[include]",
		"\tpath = " + gitx.Quote(gitx.Slash(tx.m.P.ManagedGitconfig())),
	})
	if err != nil {
		return fmt.Errorf("%s: %w", tx.m.P.Pretty(path), err)
	}
	return tx.Write(path, []byte(next), false)
}

// sshBlock sends SSH for hosts whose port 22 is blocked over port 443. It
// goes at the top of ~/.ssh/config because ssh uses the first value it finds.
func (tx *Tx) sshBlock() error {
	path := tx.m.P.SSHConfig()
	cur, existed, err := fsx.ReadIfExists(path)
	if err != nil {
		return err
	}
	var body []string
	for _, d := range tx.m.Cfg.SSH443 {
		if alt := hosts.Alt443(d); alt != "" {
			body = append(body, "Host "+d, "  Hostname "+alt, "  Port 443")
		}
	}
	if len(body) == 0 {
		if !existed {
			return nil
		}
		next, found, err := block.Remove(string(cur))
		if err != nil || !found {
			return err
		}
		return tx.Write(path, []byte(next), true)
	}
	// Anything after the block belongs to every host again.
	body = append(body, "Host *")
	next, err := block.Prepend(string(cur), body)
	if err != nil {
		return fmt.Errorf("%s: %w", tx.m.P.Pretty(path), err)
	}
	return tx.Write(path, []byte(next), true)
}
