package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kartik-dhawan-dev/git-account/internal/backup"
	"github.com/kartik-dhawan-dev/git-account/internal/block"
	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
	"github.com/kartik-dhawan-dev/git-account/internal/gitx"
	"github.com/kartik-dhawan-dev/git-account/internal/paths"
)

// Add saves a new account. The key must already exist.
func (m *Manager) Add(a config.Account) error {
	return m.Change("added "+a.ID, func(tx *Tx) error {
		if x, _ := m.Cfg.Find(a.ID); x != nil {
			return fmt.Errorf("you already have an account called %s", a.ID)
		}
		if a.UID == "" {
			a.UID = config.NewUID()
		}
		now := time.Now()
		if a.Created.IsZero() {
			a.Created = now
		}
		if a.KeyCreated.IsZero() {
			a.KeyCreated = now
		}
		a.Key = m.P.Portable(m.P.Expand(a.Key))
		folders, err := m.normalizeFolders(a.Folders)
		if err != nil {
			return err
		}
		a.Folders = nil
		m.Cfg.Accounts = append(m.Cfg.Accounts, a)
		for _, f := range folders {
			m.claimFolder(a.ID, f)
		}
		if m.Cfg.Default == "" {
			m.Cfg.Default = a.ID
		}
		return nil
	})
}

// Remove deletes an account. A key git-account created goes to the trash
// for 30 days; a key you brought yourself is left where it is.
func (m *Manager) Remove(id string) error {
	return m.Change("removed "+id, func(tx *Tx) error {
		a, i := m.Cfg.Find(id)
		if a == nil {
			return fmt.Errorf("there's no account called %s", id)
		}
		if key := m.KeyPath(a); paths.Within(key, m.P.KeysDir()) {
			if err := tx.trashKey(a.ID, key); err != nil {
				return err
			}
		}
		m.Cfg.Accounts = append(m.Cfg.Accounts[:i], m.Cfg.Accounts[i+1:]...)
		if m.Cfg.Default == id {
			m.Cfg.Default = ""
			if len(m.Cfg.Accounts) > 0 {
				m.Cfg.Default = m.Cfg.Accounts[0].ID
			}
		}
		return nil
	})
}

func (tx *Tx) trashKey(id, key string) error {
	dest := filepath.Join(tx.m.P.TrashDir(), time.Now().Format("20060102-150405")+"-"+id)
	for _, f := range []string{key, key + ".pub"} {
		data, ok, err := fsx.ReadIfExists(f)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := fsx.WriteFile(filepath.Join(dest, filepath.Base(f)), data, true); err != nil {
			return err
		}
		if err := tx.Remove(f, true); err != nil {
			return err
		}
	}
	_ = os.Remove(filepath.Dir(key))
	return nil
}

// SetDefault makes id the account used everywhere no other rule applies.
func (m *Manager) SetDefault(id string) error {
	return m.Change("made "+id+" the default", func(tx *Tx) error {
		if _, err := m.Account(id); err != nil {
			return err
		}
		m.Cfg.Default = id
		return nil
	})
}

// Edit changes the commit name, email and host username of an account.
func (m *Manager) Edit(id, name, email, username string) error {
	return m.Change("edited "+id, func(tx *Tx) error {
		a, err := m.Account(id)
		if err != nil {
			return err
		}
		a.Name, a.Email, a.Username = strings.TrimSpace(name), strings.TrimSpace(email), strings.TrimSpace(username)
		return nil
	})
}

// Rename changes an account's short name. Repos pinned to it keep working
// because profiles are named by uid.
func (m *Manager) Rename(oldID, newID string) error {
	return m.Change("renamed "+oldID+" to "+newID, func(tx *Tx) error {
		if err := config.ValidateID(newID); err != nil {
			return err
		}
		a, err := m.Account(oldID)
		if err != nil {
			return err
		}
		if newID != oldID {
			if x, _ := m.Cfg.Find(newID); x != nil {
				return fmt.Errorf("you already have an account called %s", newID)
			}
		}
		a.ID = newID
		if m.Cfg.Default == oldID {
			m.Cfg.Default = newID
		}
		return nil
	})
}

func (m *Manager) normalizeFolders(in []string) ([]string, error) {
	var out []string
	for _, f := range in {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		abs := m.P.Expand(f)
		if paths.Same(abs, m.P.Home) || paths.IsRoot(abs) {
			return nil, fmt.Errorf("%s would match every repo; pick a folder like ~/work", m.P.Pretty(abs))
		}
		if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
			return nil, fmt.Errorf("%s is a file, not a folder", m.P.Pretty(abs))
		}
		p := m.P.Portable(abs)
		if err := config.ValidateFolder(p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// claimFolder gives folder to account id, taking it from any other account.
func (m *Manager) claimFolder(id, folder string) {
	for i := range m.Cfg.Accounts {
		a := &m.Cfg.Accounts[i]
		var keep []string
		for _, f := range a.Folders {
			if !strings.EqualFold(f, folder) {
				keep = append(keep, f)
			}
		}
		if a.ID == id {
			keep = append(keep, folder)
		}
		a.Folders = keep
	}
}

// AddFolder makes every repo inside folder use account id.
func (m *Manager) AddFolder(id, folder string) (string, error) {
	var stored string
	err := m.Change("set "+folder+" for "+id, func(tx *Tx) error {
		if _, err := m.Account(id); err != nil {
			return err
		}
		fs, err := m.normalizeFolders([]string{folder})
		if err != nil {
			return err
		}
		if len(fs) == 0 {
			return errors.New("the folder is empty")
		}
		stored = fs[0]
		m.claimFolder(id, stored)
		return nil
	})
	return stored, err
}

// RemoveFolder stops a folder from choosing account id.
func (m *Manager) RemoveFolder(id, folder string) error {
	return m.Change("removed "+folder+" from "+id, func(tx *Tx) error {
		a, err := m.Account(id)
		if err != nil {
			return err
		}
		want := m.P.Portable(m.P.Expand(folder))
		var keep []string
		for _, f := range a.Folders {
			if !strings.EqualFold(f, want) && !strings.EqualFold(f, folder) {
				keep = append(keep, f)
			}
		}
		if len(keep) == len(a.Folders) {
			return fmt.Errorf("%s isn't set for %s", folder, id)
		}
		a.Folders = keep
		return nil
	})
}

// UseHere pins account id to the repository containing dir. The repo's own
// user.name, user.email and core.sshCommand are removed, because they would
// win over the account. It returns the keys it removed.
func (m *Manager) UseHere(dir, id string) ([]string, error) {
	if _, ok := m.Git.TopLevel(dir); !ok {
		return nil, ErrNotRepo
	}
	cfgFile, err := m.Git.LocalConfigFile(dir)
	if err != nil {
		return nil, err
	}
	var removed []string
	err = m.Change("used "+id+" in "+filepath.Base(dir), func(tx *Tx) error {
		a, err := m.Account(id)
		if err != nil {
			return err
		}
		if err := tx.Track(cfgFile, false); err != nil {
			return err
		}
		if err := m.unpin(dir); err != nil {
			return err
		}
		for _, k := range []string{"user.name", "user.email", "core.sshCommand"} {
			if _, ok, _ := m.Git.Get(dir, "--local", "--get", k); ok {
				if _, err := m.Git.Run(dir, "config", "--local", "--unset-all", k); err != nil {
					return err
				}
				removed = append(removed, k)
			}
		}
		_, err = m.Git.Run(dir, "config", "--local", "--add", "include.path", gitx.Slash(m.ProfilePath(a)))
		return err
	})
	return removed, err
}

// Unpin removes the account pinned to the repository containing dir.
func (m *Manager) Unpin(dir string) error {
	cfgFile, err := m.Git.LocalConfigFile(dir)
	if err != nil {
		return ErrNotRepo
	}
	return m.Change("unpinned "+filepath.Base(dir), func(tx *Tx) error {
		if err := tx.Track(cfgFile, false); err != nil {
			return err
		}
		return m.unpin(dir)
	})
}

func (m *Manager) unpin(dir string) error {
	vals, err := m.Git.GetAll(dir, "--local", "--get-all", "include.path")
	if err != nil {
		return err
	}
	for _, v := range vals {
		if paths.Within(filepath.FromSlash(v), m.P.ProfilesDir()) {
			pattern := "^" + regexp.QuoteMeta(v) + "$"
			if _, err := m.Git.Run(dir, "config", "--local", "--unset", "include.path", pattern); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReplaceKey switches an account to a new key. The old key goes to the
// trash if git-account created it.
func (m *Manager) ReplaceKey(id, newKey string) error {
	return m.Change("replaced "+id+"'s key", func(tx *Tx) error {
		a, err := m.Account(id)
		if err != nil {
			return err
		}
		old := m.KeyPath(a)
		if !paths.Same(old, newKey) && paths.Within(old, m.P.KeysDir()) {
			if err := tx.trashKey(a.ID, old); err != nil {
				return err
			}
		}
		a.Key = m.P.Portable(m.P.Expand(newKey))
		a.KeyCreated = time.Now()
		return nil
	})
}

// Enable443 sends SSH for domain over port 443.
func (m *Manager) Enable443(domain string) error {
	return m.Change("use port 443 for "+domain, func(tx *Tx) error {
		for _, d := range m.Cfg.SSH443 {
			if strings.EqualFold(d, domain) {
				return nil
			}
		}
		m.Cfg.SSH443 = append(m.Cfg.SSH443, domain)
		return nil
	})
}

// UnsetIn removes key from a git config file that isn't git-account's,
// such as a global core.sshCommand left over from another tool.
func (m *Manager) UnsetIn(file, key string) error {
	return m.Change("removed "+key+" from "+m.P.Pretty(file), func(tx *Tx) error {
		if err := tx.Track(file, false); err != nil {
			return err
		}
		_, err := m.Git.Run("", "config", "--file", file, "--unset-all", key)
		return err
	})
}

// Sync rebuilds every generated file from config.toml.
func (m *Manager) Sync() error { return m.Change("synced files", func(tx *Tx) error { return nil }) }

// SetTheme saves the theme. It isn't an undoable change.
func (m *Manager) SetTheme(id string) error {
	unlock, err := fsx.Lock(m.P.LockFile(), 5*time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	m.Cfg.Theme = id
	data, err := m.Cfg.Encode()
	if err != nil {
		return err
	}
	return fsx.WriteFile(m.P.ConfigFile(), data, false)
}

// Undo reverts the most recent change and returns its label.
func (m *Manager) Undo() (string, error) {
	unlock, err := fsx.Lock(m.P.LockFile(), 5*time.Second)
	if err != nil {
		return "", err
	}
	defer unlock()
	h := m.History()
	if len(h) == 0 {
		return "", errors.New("there's nothing to undo")
	}
	return h[0].Label, m.restore(h[0])
}

// Recover puts back the files of a change that never finished.
func (m *Manager) Recover(s *backup.Snapshot) error {
	unlock, err := fsx.Lock(m.P.LockFile(), 5*time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	return m.restore(s)
}

func (m *Manager) restore(s *backup.Snapshot) error {
	if err := s.Restore(); err != nil {
		return err
	}
	_ = s.Discard()
	return m.Reload()
}

// Uninstall removes git-account's blocks from your files and its generated
// files. config.toml stays so you can reinstall; purge also deletes it and
// the keys git-account created.
func (m *Manager) Uninstall(purge bool) error {
	return m.Change("uninstalled", func(tx *Tx) error {
		tx.noSync = true
		for _, f := range []string{m.P.GlobalGitconfig(), m.P.SSHConfig()} {
			cur, ok, err := fsx.ReadIfExists(f)
			if err != nil || !ok {
				continue
			}
			next, found, err := block.Remove(string(cur))
			if err != nil {
				return fmt.Errorf("%s: %w", m.P.Pretty(f), err)
			}
			if found {
				if err := tx.Write(f, []byte(next), f == m.P.SSHConfig()); err != nil {
					return err
				}
			}
		}
		for path := range m.Generated() {
			if err := tx.Remove(path, false); err != nil {
				return err
			}
		}
		_ = os.Remove(m.P.ProfilesDir())
		if !purge {
			return nil
		}
		for i := range m.Cfg.Accounts {
			key := m.KeyPath(&m.Cfg.Accounts[i])
			if paths.Within(key, m.P.KeysDir()) {
				if err := tx.trashKey(m.Cfg.Accounts[i].ID, key); err != nil {
					return err
				}
			}
		}
		_ = os.Remove(m.P.KeysDir())
		return tx.Remove(m.P.ConfigFile(), false)
	})
}
