//go:build !windows

package fsx

import "os"

// RestrictToOwner makes path readable and writable only by its owner.
func RestrictToOwner(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if fi.IsDir() {
		mode = 0o700
	}
	return os.Chmod(path, mode)
}

// IsPrivate reports whether nobody but the owner can read path.
func IsPrivate(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fi.Mode().Perm()&0o077 == 0, nil
}

// LongPath returns path unchanged; short 8.3 names only exist on Windows.
func LongPath(path string) string { return path }
