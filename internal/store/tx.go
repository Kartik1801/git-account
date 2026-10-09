package store

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/kartik-dhawan-dev/git-account/internal/backup"
	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
)

// Tx is one change in progress. Every write goes through it so the file is
// backed up first.
type Tx struct {
	m      *Manager
	snap   *backup.Snapshot
	noSync bool // skip saving config.toml and regenerating files
}

// Change runs fn as one undoable change. fn edits m.Cfg and may write other
// files through tx; afterwards config.toml is saved and every generated
// file is rebuilt. If anything fails, all files are put back.
func (m *Manager) Change(label string, fn func(tx *Tx) error) error {
	unlock, err := fsx.Lock(m.P.LockFile(), 5*time.Second)
	if err != nil {
		return err
	}
	defer unlock()

	before := m.Cfg.Clone()
	snap, err := backup.Begin(m.P.BackupsDir(), label)
	if err != nil {
		return fmt.Errorf("couldn't make a backup, so nothing was changed: %w", err)
	}
	tx := &Tx{m: m, snap: snap}

	err = fn(tx)
	if err == nil && !tx.noSync {
		if err = m.Cfg.Validate(); err == nil {
			if err = tx.saveConfig(); err == nil {
				err = tx.sync()
			}
		}
	}
	if err != nil {
		rerr := snap.Restore()
		_ = snap.Discard()
		m.Cfg = before
		if rerr != nil {
			return fmt.Errorf("%w (restoring the backup also failed: %v)", err, rerr)
		}
		return err
	}
	if snap.Unchanged() {
		return snap.Discard()
	}
	if err := snap.Commit(); err != nil {
		return err
	}
	backup.Prune(m.P.BackupsDir(), 30)
	return nil
}

// Track backs up path before it is changed some other way, such as by git.
func (tx *Tx) Track(path string, private bool) error { return tx.snap.Track(path, private) }

// Write replaces a file, skipping the write when nothing changed.
func (tx *Tx) Write(path string, data []byte, private bool) error {
	if cur, ok, _ := fsx.ReadIfExists(path); ok && bytes.Equal(cur, data) {
		return nil
	}
	if err := tx.snap.Track(path, private); err != nil {
		return err
	}
	return fsx.WriteFile(path, data, private)
}

// Remove deletes a file after backing it up.
func (tx *Tx) Remove(path string, private bool) error {
	if !fsx.Exists(path) {
		return nil
	}
	if err := tx.snap.Track(path, private); err != nil {
		return err
	}
	return os.Remove(path)
}

func (tx *Tx) saveConfig() error {
	data, err := tx.m.Cfg.Encode()
	if err != nil {
		return err
	}
	return tx.Write(tx.m.P.ConfigFile(), data, false)
}
