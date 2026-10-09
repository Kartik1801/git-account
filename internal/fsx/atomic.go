// Package fsx has the file operations git-account relies on: writes that
// never leave half a file behind, owner-only permissions and a process lock.
package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// WriteFile replaces path with data atomically: it writes a temporary file
// next to it, flushes it to disk and renames it over the original. A crash
// leaves either the old file or the new one, never a mix. When private is
// true the file is readable only by the current user.
func WriteFile(path string, data []byte, private bool) error {
	dir := filepath.Dir(path)
	if private {
		if err := MkdirPrivate(dir); err != nil {
			return err
		}
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}

	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	done := false
	defer func() {
		if !done {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()

	if private {
		if err := RestrictToOwner(tmp); err != nil {
			return err
		}
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if !private && runtime.GOOS != "windows" {
		if err := os.Chmod(tmp, mode); err != nil {
			return err
		}
	}
	if err := rename(tmp, path); err != nil {
		return err
	}
	done = true
	return nil
}

// rename retries briefly on Windows, where antivirus or an editor can hold
// the target open for a moment.
func rename(from, to string) error {
	var err error
	for i := 0; i < 10; i++ {
		if err = os.Rename(from, to); err == nil || runtime.GOOS != "windows" {
			return err
		}
		time.Sleep(time.Duration(i+1) * 20 * time.Millisecond)
	}
	return err
}

// MkdirPrivate creates dir (and missing parents) readable only by the
// current user. An existing folder is left as it is.
func MkdirPrivate(dir string) error {
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return RestrictToOwner(dir)
}

// ReadIfExists reads path, reporting ok=false instead of an error when the
// file doesn't exist.
func ReadIfExists(path string) (data []byte, ok bool, err error) {
	data, err = os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
