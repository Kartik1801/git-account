// Package backup keeps a copy of every file before git-account changes it,
// which is what undo restores.
package backup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
)

type Entry struct {
	Path    string `json:"path"`
	Existed bool   `json:"existed"`
	Private bool   `json:"private,omitempty"`
	Mode    uint32 `json:"mode,omitempty"`
	File    string `json:"file,omitempty"`
}

// Snapshot is one change: the files it touched as they were before.
type Snapshot struct {
	Dir     string    `json:"-"`
	Label   string    `json:"label"`
	Created time.Time `json:"created"`
	Done    bool      `json:"done"`
	Entries []Entry   `json:"entries"`
	seen    map[string]bool
}

const manifest = "manifest.json"

// Begin starts a snapshot under root. Snapshots can contain private keys,
// so the folder is readable only by you.
func Begin(root, label string) (*Snapshot, error) {
	if err := fsx.MkdirPrivate(root); err != nil {
		return nil, err
	}
	now := time.Now()
	base := now.UTC().Format("20060102-150405.000000")
	dir := filepath.Join(root, base)
	for i := 2; fsx.Exists(dir); i++ {
		dir = filepath.Join(root, fmt.Sprintf("%s-%d", base, i))
	}
	if err := fsx.MkdirPrivate(dir); err != nil {
		return nil, err
	}
	s := &Snapshot{Dir: dir, Label: label, Created: now, seen: map[string]bool{}}
	return s, s.save()
}

func key(path string) string {
	p := filepath.Clean(path)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		p = strings.ToLower(p)
	}
	return p
}

// Track records path as it is now. Call it before changing the file; a
// second call for the same path does nothing.
func (s *Snapshot) Track(path string, private bool) error {
	k := key(path)
	if s.seen[k] {
		return nil
	}
	data, existed, err := fsx.ReadIfExists(path)
	if err != nil {
		return err
	}
	e := Entry{Path: path, Existed: existed, Private: private}
	if existed {
		if fi, err := os.Stat(path); err == nil {
			e.Mode = uint32(fi.Mode().Perm())
		}
		e.File = fmt.Sprintf("%03d", len(s.Entries))
		if err := fsx.WriteFile(filepath.Join(s.Dir, e.File), data, true); err != nil {
			return err
		}
	}
	s.Entries = append(s.Entries, e)
	s.seen[k] = true
	return s.save()
}

// Unchanged reports whether every tracked file is still as it was.
func (s *Snapshot) Unchanged() bool {
	for _, e := range s.Entries {
		cur, ok, err := fsx.ReadIfExists(e.Path)
		if err != nil || ok != e.Existed {
			return false
		}
		if !ok {
			continue
		}
		old, err := os.ReadFile(filepath.Join(s.Dir, e.File))
		if err != nil || !bytes.Equal(old, cur) {
			return false
		}
	}
	return true
}

// Commit marks the change as finished.
func (s *Snapshot) Commit() error {
	s.Done = true
	return s.save()
}

// Restore puts every tracked file back the way it was.
func (s *Snapshot) Restore() error {
	var errs []error
	for i := len(s.Entries) - 1; i >= 0; i-- {
		e := s.Entries[i]
		if !e.Existed {
			if err := os.Remove(e.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, err)
			}
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, e.File))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := fsx.WriteFile(e.Path, data, e.Private); err != nil {
			errs = append(errs, err)
			continue
		}
		if !e.Private && runtime.GOOS != "windows" && e.Mode != 0 {
			_ = os.Chmod(e.Path, os.FileMode(e.Mode))
		}
	}
	return errors.Join(errs...)
}

// Discard deletes the snapshot.
func (s *Snapshot) Discard() error { return os.RemoveAll(s.Dir) }

func (s *Snapshot) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFile(filepath.Join(s.Dir, manifest), data, true)
}

// List returns snapshots under root, newest first.
func List(root string) ([]*Snapshot, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Snapshot
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, manifest))
		if err != nil {
			continue
		}
		s := &Snapshot{}
		if json.Unmarshal(data, s) != nil {
			continue
		}
		s.Dir = dir
		s.seen = map[string]bool{}
		for _, en := range s.Entries {
			s.seen[key(en.Path)] = true
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return filepath.Base(out[i].Dir) > filepath.Base(out[j].Dir) })
	return out, nil
}

// Prune keeps the newest keep finished snapshots.
func Prune(root string, keep int) {
	list, _ := List(root)
	n := 0
	for _, s := range list {
		if !s.Done {
			continue
		}
		n++
		if n > keep {
			_ = s.Discard()
		}
	}
}
