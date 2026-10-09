package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
	"github.com/kartik-dhawan-dev/git-account/internal/keys"
	"github.com/kartik-dhawan-dev/git-account/internal/store"
)

// sandbox points HOME, git's global config and git-account's own folders
// at a temporary directory, so tests never touch the real machine.
func sandbox(t *testing.T) (home string) {
	t.Helper()
	root := fsx.LongPath(t.TempDir())
	home = filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_ACCOUNT_DIR", filepath.Join(root, "ga"))
	t.Setenv("GIT_SSH_COMMAND", "")
	return home
}

func open(t *testing.T) *store.Manager {
	t.Helper()
	m, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func repo(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "init", "-q")
	return dir
}

func email(t *testing.T, dir string) string {
	t.Helper()
	return run(t, dir, "config", "user.email")
}

func account(t *testing.T, m *store.Manager, id, mail string, folders ...string) config.Account {
	t.Helper()
	path := m.NewKeyPath(id, "ed25519")
	if _, _, err := keys.Generate(path, "ed25519", mail, nil); err != nil {
		t.Fatal(err)
	}
	return config.Account{ID: id, Name: "Test " + id, Email: mail, Host: "github", Username: id + "-user", Key: path, Folders: folders}
}

func TestFoldersDefaultPinUndo(t *testing.T) {
	home := sandbox(t)
	// An existing identity in ~/.gitconfig must lose to the default account.
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Old\n\temail = old@example.com\n[alias]\n\tst = status\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := open(t)
	if err := m.Add(account(t, m, "personal", "me@example.com")); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(account(t, m, "work", "me@acme.example", "~/work")); err != nil {
		t.Fatal(err)
	}

	api := repo(t, filepath.Join(home, "work", "api"))
	deep := repo(t, filepath.Join(home, "work", "team", "svc"))
	blog := repo(t, filepath.Join(home, "side", "blog"))

	if got := email(t, api); got != "me@acme.example" {
		t.Errorf("repo in ~/work uses %q, want the work email", got)
	}
	if got := email(t, deep); got != "me@acme.example" {
		t.Errorf("nested repo in ~/work uses %q, want the work email", got)
	}
	if got := email(t, blog); got != "me@example.com" {
		t.Errorf("repo outside ~/work uses %q, want the default account", got)
	}
	if cmd := run(t, api, "config", "core.sshCommand"); !strings.Contains(cmd, "IdentitiesOnly=yes") || !strings.Contains(cmd, "work") {
		t.Errorf("sshCommand in ~/work is %q", cmd)
	}
	if got := run(t, blog, "config", "alias.st"); got != "status" {
		t.Errorf("your own settings must survive, alias.st = %q", got)
	}

	if h := m.Resolve(api); h.Kind != "folder" || h.Account != "work" {
		t.Errorf("Resolve(api) = %+v", h)
	}
	if h := m.Resolve(blog); h.Kind != "default" || h.Account != "personal" {
		t.Errorf("Resolve(blog) = %+v", h)
	}

	// Pin work to the blog repo, which has its own user.email too.
	run(t, blog, "config", "user.email", "local@example.com")
	removed, err := m.UseHere(blog, "work")
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != "user.email" {
		t.Errorf("removed = %v, want [user.email]", removed)
	}
	if got := email(t, blog); got != "me@acme.example" {
		t.Errorf("pinned repo uses %q", got)
	}
	if h := m.Resolve(blog); h.Kind != "repo" || h.Account != "work" {
		t.Errorf("Resolve(pinned) = %+v", h)
	}

	// Renaming keeps the pin working.
	if err := m.Rename("work", "job"); err != nil {
		t.Fatal(err)
	}
	if h := m.Resolve(blog); h.Account != "job" {
		t.Errorf("after rename Resolve = %+v", h)
	}

	// Undo the rename, then the pin: the repo's own email comes back.
	for _, want := range []string{"renamed work to job", "used work in blog"} {
		label, err := m.Undo()
		if err != nil {
			t.Fatal(err)
		}
		if label != want {
			t.Errorf("undo label = %q, want %q", label, want)
		}
	}
	if got := email(t, blog); got != "local@example.com" {
		t.Errorf("after undo the repo uses %q, want its own email back", got)
	}

	// Uninstall leaves the original identity in place.
	if err := m.Uninstall(false); err != nil {
		t.Fatal(err)
	}
	if got := email(t, api); got != "old@example.com" {
		t.Errorf("after uninstall ~/work uses %q, want the original email", got)
	}
}

func TestFailedChangeRollsBack(t *testing.T) {
	home := sandbox(t)
	m := open(t)
	if err := m.Add(account(t, m, "personal", "me@example.com")); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(home, ".gitconfig"))
	cfgBefore, _ := os.ReadFile(m.P.ConfigFile())

	bad := account(t, m, "broken", "not-an-email")
	if err := m.Add(bad); err == nil {
		t.Fatal("adding an invalid account should fail")
	}
	after, _ := os.ReadFile(filepath.Join(home, ".gitconfig"))
	cfgAfter, _ := os.ReadFile(m.P.ConfigFile())
	if string(before) != string(after) || string(cfgBefore) != string(cfgAfter) {
		t.Error("a failed change must leave files exactly as they were")
	}
	if len(m.Cfg.Accounts) != 1 {
		t.Errorf("in-memory config has %d accounts after a failed add", len(m.Cfg.Accounts))
	}
	if len(m.History()) != 1 {
		t.Errorf("a failed change must not add an undo step, history = %d", len(m.History()))
	}
}

func TestRemoveTrashesKeyAndUndoRestoresIt(t *testing.T) {
	sandbox(t)
	m := open(t)
	a := account(t, m, "work", "me@acme.example")
	if err := m.Add(a); err != nil {
		t.Fatal(err)
	}
	key := m.P.Expand(a.Key)
	if err := m.Remove("work"); err != nil {
		t.Fatal(err)
	}
	if fsx.Exists(key) {
		t.Error("key should be gone after remove")
	}
	if entries, _ := os.ReadDir(m.P.TrashDir()); len(entries) != 1 {
		t.Errorf("trash has %d entries, want 1", len(entries))
	}
	if _, err := m.Undo(); err != nil {
		t.Fatal(err)
	}
	if !fsx.Exists(key) {
		t.Error("undo should bring the key back")
	}
	if ok, err := fsx.IsPrivate(key); err != nil || !ok {
		t.Errorf("restored key must stay private (ok=%v err=%v)", ok, err)
	}
	if a, _ := m.Cfg.Find("work"); a == nil {
		t.Error("undo should bring the account back")
	}
}

func TestNoOpChangeAddsNoUndoStep(t *testing.T) {
	sandbox(t)
	m := open(t)
	if err := m.Add(account(t, m, "personal", "me@example.com")); err != nil {
		t.Fatal(err)
	}
	if err := m.Sync(); err != nil {
		t.Fatal(err)
	}
	if n := len(m.History()); n != 1 {
		t.Errorf("sync with nothing to do left %d undo steps, want 1", n)
	}
	if stale := m.OutOfDate(); len(stale) != 0 {
		t.Errorf("out of date right after sync: %v", stale)
	}
}
