package fsx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// ErrBusy means another git-account process is changing files right now.
var ErrBusy = errors.New("another git-account window is making changes; try again in a moment")

// Lock takes an exclusive lock on path, waiting up to wait for it. Call the
// returned function to release it.
func Lock(path string, wait time.Duration) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	fl := flock.New(path)
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	ok, err := fl.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	if !ok {
		return nil, ErrBusy
	}
	return func() { _ = fl.Unlock() }, nil
}
