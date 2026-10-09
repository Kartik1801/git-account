// Package paths works out where git-account keeps its files on each OS.
package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Paths holds every location git-account reads or writes.
type Paths struct {
	Home   string // the user's home folder
	Config string // config.toml, generated git config, custom themes
	State  string // backups, trash, lock file
	SSH    string // ~/.ssh
}

// Default returns the standard locations for this OS. GIT_ACCOUNT_DIR moves
// config and state into one folder, which tests and portable setups use.
func Default() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return Paths{}, errors.New("can't find your home folder; set HOME (or USERPROFILE on Windows)")
	}
	p := Paths{Home: filepath.Clean(home), SSH: filepath.Join(home, ".ssh")}
	if dir := os.Getenv("GIT_ACCOUNT_DIR"); dir != "" {
		p.Config = filepath.Join(dir, "config")
		p.State = filepath.Join(dir, "state")
		return p, nil
	}
	if runtime.GOOS == "windows" {
		p.Config = filepath.Join(envOr("APPDATA", filepath.Join(home, "AppData", "Roaming")), "git-account")
		p.State = filepath.Join(envOr("LOCALAPPDATA", filepath.Join(home, "AppData", "Local")), "git-account")
	} else {
		p.Config = filepath.Join(envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config")), "git-account")
		p.State = filepath.Join(envOr("XDG_STATE_HOME", filepath.Join(home, ".local", "state")), "git-account")
	}
	return p, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (p Paths) ConfigFile() string        { return filepath.Join(p.Config, "config.toml") }
func (p Paths) ManagedGitconfig() string  { return filepath.Join(p.Config, "gitconfig") }
func (p Paths) ProfilesDir() string       { return filepath.Join(p.Config, "profiles") }
func (p Paths) Profile(uid string) string { return filepath.Join(p.ProfilesDir(), uid+".gitconfig") }
func (p Paths) ThemesDir() string         { return filepath.Join(p.Config, "themes") }
func (p Paths) BackupsDir() string        { return filepath.Join(p.State, "backups") }
func (p Paths) TrashDir() string          { return filepath.Join(p.State, "trash") }
func (p Paths) LockFile() string          { return filepath.Join(p.State, "lock") }
func (p Paths) KeysDir() string           { return filepath.Join(p.SSH, "git-account") }
func (p Paths) KeyDir(id string) string   { return filepath.Join(p.KeysDir(), id) }
func (p Paths) SSHConfig() string         { return filepath.Join(p.SSH, "config") }

// GlobalGitconfig is the file `git config --global` writes to.
func (p Paths) GlobalGitconfig() string {
	if v := os.Getenv("GIT_CONFIG_GLOBAL"); v != "" {
		return p.Expand(v)
	}
	home := p.Home
	if runtime.GOOS == "windows" {
		if h := os.Getenv("HOME"); h != "" {
			home = h
		}
	}
	main := filepath.Join(home, ".gitconfig")
	if exists(main) {
		return main
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	if alt := filepath.Join(xdg, "git", "config"); exists(alt) {
		return alt
	}
	return main
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CaseInsensitive reports whether file names on this OS usually ignore case.
func CaseInsensitive() bool { return runtime.GOOS == "windows" || runtime.GOOS == "darwin" }

// Expand turns "~/x" into an absolute, clean path.
func (p Paths) Expand(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "~":
		return p.Home
	case strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`):
		s = filepath.Join(p.Home, s[2:])
	}
	if abs, err := filepath.Abs(s); err == nil {
		s = abs
	}
	return filepath.Clean(s)
}

// Same reports whether a and b name the same path.
func Same(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if CaseInsensitive() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// Within reports whether child is parent or inside it.
func Within(child, parent string) bool {
	c, pa := filepath.Clean(child), filepath.Clean(parent)
	if CaseInsensitive() {
		c, pa = strings.ToLower(c), strings.ToLower(pa)
	}
	if c == pa {
		return true
	}
	rel, err := filepath.Rel(pa, c)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Pretty shows a path the way people type it, with ~ for the home folder.
func (p Paths) Pretty(path string) string {
	if path == "" {
		return ""
	}
	if Same(path, p.Home) {
		return "~"
	}
	if Within(path, p.Home) {
		if rel, err := filepath.Rel(p.Home, path); err == nil && !strings.HasPrefix(rel, "..") {
			return "~" + string(filepath.Separator) + rel
		}
	}
	return path
}

// Portable is how a path is stored in config.toml: ~/ for the home folder
// and forward slashes, so the file works when copied to another machine.
func (p Paths) Portable(path string) string {
	if Same(path, p.Home) {
		return "~"
	}
	if Within(path, p.Home) {
		if rel, err := filepath.Rel(p.Home, path); err == nil && !strings.HasPrefix(rel, "..") {
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}

// Parent returns the folder containing path.
func Parent(path string) string { return filepath.Dir(filepath.Clean(path)) }

// IsRoot reports whether path is a filesystem or drive root.
func IsRoot(path string) bool {
	path = filepath.Clean(path)
	return filepath.Dir(path) == path
}
