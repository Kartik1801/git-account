// Package gitx runs the git binary. git-account never parses .gitconfig
// files itself; it asks git, so quoting and includes behave exactly as git
// sees them.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Git struct {
	Bin string
}

// Error is a failed git command.
type Error struct {
	Args   []string
	Code   int
	Stderr string
}

func (e *Error) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		msg = fmt.Sprintf("exit status %d", e.Code)
	}
	return "git " + strings.Join(e.Args, " ") + ": " + msg
}

// ExitCode returns git's exit code for err, or -1.
func ExitCode(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return -1
}

// Find locates git on the PATH.
func Find() (*Git, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return nil, errors.New("git isn't installed or isn't on your PATH")
	}
	return &Git{Bin: bin}, nil
}

// Run runs git in dir and returns its standard output.
func (g *Git) Run(dir string, args ...string) (string, error) {
	cmd := exec.Command(g.Bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		code := -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		}
		return out.String(), &Error{Args: args, Code: code, Stderr: errb.String() + errText(err, ee)}
	}
	return out.String(), nil
}

func errText(err error, ee *exec.ExitError) string {
	if ee != nil {
		return ""
	}
	return err.Error()
}

// Version returns git's major and minor version and the full version text.
func (g *Git) Version() (major, minor int, raw string, err error) {
	out, err := g.Run("", "--version")
	if err != nil {
		return 0, 0, "", err
	}
	f := strings.Fields(strings.TrimSpace(out))
	if len(f) < 3 {
		return 0, 0, strings.TrimSpace(out), fmt.Errorf("can't read the git version from %q", out)
	}
	nums := strings.Split(f[2], ".")
	major, _ = strconv.Atoi(nums[0])
	if len(nums) > 1 {
		minor, _ = strconv.Atoi(nums[1])
	}
	return major, minor, f[2], nil
}

// TopLevel returns the root of the repository containing dir.
func (g *Git) TopLevel(dir string) (string, bool) {
	out, err := g.Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", false
	}
	return filepath.Clean(filepath.FromSlash(strings.TrimSpace(out))), true
}

// LocalConfigFile returns the repository's config file, shared by all its
// worktrees.
func (g *Git) LocalConfigFile(dir string) (string, error) {
	out, err := g.Run(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	common := filepath.FromSlash(strings.TrimSpace(out))
	if !filepath.IsAbs(common) {
		common = filepath.Join(dir, common)
	}
	return filepath.Join(common, "config"), nil
}

// Get runs `git config <args>` and returns the value. ok is false when the
// key isn't set.
func (g *Git) Get(dir string, args ...string) (string, bool, error) {
	out, err := g.Run(dir, append([]string{"config"}, args...)...)
	if err != nil {
		if ExitCode(err) == 1 {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimRight(out, "\r\n"), true, nil
}

// GetAll runs `git config -z <args>` for a multi-valued key.
func (g *Git) GetAll(dir string, args ...string) ([]string, error) {
	out, err := g.Run(dir, append([]string{"config", "-z"}, args...)...)
	if err != nil {
		if ExitCode(err) == 1 {
			return nil, nil
		}
		return nil, err
	}
	var vals []string
	for _, v := range strings.Split(out, "\x00") {
		if v != "" {
			vals = append(vals, v)
		}
	}
	return vals, nil
}

// Origin returns the value git uses for a key and the file it came from,
// using `git config --show-origin -z <args>`, for example
// Origin(dir, "--get", "user.email").
func (g *Git) Origin(dir string, args ...string) (value, file string, ok bool, err error) {
	out, err := g.Run(dir, append([]string{"config", "--show-origin", "-z"}, args...)...)
	if err != nil {
		if ExitCode(err) == 1 {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	parts := strings.Split(out, "\x00")
	if len(parts) < 2 {
		return "", "", false, nil
	}
	origin := parts[0]
	if strings.HasPrefix(origin, "file:") {
		file = filepath.Clean(filepath.FromSlash(strings.TrimPrefix(origin, "file:")))
	}
	return parts[1], file, true, nil
}
