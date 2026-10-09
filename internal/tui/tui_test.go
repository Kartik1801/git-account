package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/doctor"
	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
	"github.com/kartik-dhawan-dev/git-account/internal/keys"
	"github.com/kartik-dhawan-dev/git-account/internal/store"
	"github.com/kartik-dhawan-dev/git-account/internal/theme"
)

func sandbox(t *testing.T) (*store.Manager, string) {
	t.Helper()
	root := fsx.LongPath(t.TempDir())
	home := filepath.Join(root, "home")
	os.MkdirAll(home, 0o755)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_ACCOUNT_DIR", filepath.Join(root, "ga"))
	m, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"personal", "work"} {
		path := m.NewKeyPath(id, "ed25519")
		if _, _, err := keys.Generate(path, "ed25519", id+"@example.com", nil); err != nil {
			t.Fatal(err)
		}
		a := config.Account{ID: id, Name: "Test", Email: id + "@example.com", Host: "github", Username: id, Key: path}
		if id == "work" {
			a.Folders = []string{"~/work"}
		}
		if err := m.Add(a); err != nil {
			t.Fatal(err)
		}
	}
	return m, home
}

func press(t *testing.T, m *Model, keys ...string) {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m.Update(msg)
		if v := m.View(); v == "" {
			t.Fatalf("empty view after %q", k)
		}
	}
}

func view(m *Model) string { return m.View() }

// Every screen renders at a normal and a small terminal size, in every theme.
func TestScreensRender(t *testing.T) {
	mgr, home := sandbox(t)
	for _, size := range [][2]int{{120, 40}, {60, 20}} {
		m := New(mgr, filepath.Join(home, "work"), true)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		if !strings.Contains(view(m), "personal") {
			t.Fatalf("home doesn't list accounts:\n%s", view(m))
		}
		press(t, m, "down", "up", "right", "down", "down", "esc")
		press(t, m, "?", "esc")
		press(t, m, "s")
		for range theme.Builtin {
			press(t, m, "down")
		}
		press(t, m, "esc")
		// Forms: edit, rename, folders, and back.
		press(t, m, "right", "e", "esc", "n", "esc", "f", "tab", "esc", "esc")
		// Wizard up to the key step; connecting would open a browser.
		press(t, m, "a", "enter", "enter", "enter")
		if m.add == nil || m.add.err == "" {
			t.Errorf("wizard should refuse the duplicate short name")
		}
		m.add.id.SetValue("client-x")
		m.add.email.SetValue("x@example.com")
		press(t, m, "enter", "enter", "enter")
		if m.add.step != 1 {
			t.Fatalf("wizard should be on the host step, step=%d err=%q", m.add.step, m.add.err)
		}
		press(t, m, "down", "down", "down")
		m.add.user.SetValue("me")
		m.add.hostname.SetValue("git.example.com")
		press(t, m, "enter", "enter", "enter")
		if m.add.step != 2 {
			t.Fatalf("wizard should be on the key step, step=%d err=%q", m.add.step, m.add.err)
		}
		press(t, m, "down", "up", "esc", "esc", "esc")
		if m.scr != scrHome {
			t.Errorf("esc should lead back home, screen=%d", m.scr)
		}
		// Check screen with injected results instead of network tests.
		m.chk = &checkModel{gen: 99, pending: 1}
		m.scr = scrCheck
		m.Update(localMsg{gen: 99, results: []doctor.Result{{Level: doctor.OK, Title: "git ok"}, {Level: doctor.Problem, Title: "broken", Detail: "why", Fix: &doctor.Fix{Label: "fix it"}}}})
		m.Update(connMsg{gen: 99, res: doctor.Result{Level: doctor.OK, Title: "work → github.com says Hi @work"}})
		if v := view(m); !strings.Contains(v, "broken") || !strings.Contains(v, "fix it") {
			t.Errorf("check screen should show the problem and its fix first:\n%s", v)
		}
		press(t, m, "down", "esc")
	}
}

func TestUseHereAndUndoFromTheApp(t *testing.T) {
	mgr, home := sandbox(t)
	dir := filepath.Join(home, "side", "blog")
	os.MkdirAll(dir, 0o755)
	if _, err := mgr.Git.Run(dir, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	m := New(mgr, dir, true)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.here.Account != "personal" || m.here.Kind != "default" {
		t.Fatalf("here = %+v", m.here)
	}
	// select work and press enter
	for m.selected().ID != "work" {
		press(t, m, "down")
	}
	press(t, m, "enter")
	if m.here.Account != "work" || m.here.Kind != "repo" {
		t.Errorf("after enter here = %+v, toast %q", m.here, m.toast)
	}
	press(t, m, "u")
	if m.here.Account != "personal" {
		t.Errorf("after undo here = %+v", m.here)
	}
	// remove needs a confirmation
	press(t, m, "x", "n")
	if len(mgr.Cfg.Accounts) != 2 {
		t.Error("n must keep the account")
	}
	press(t, m, "x", "y")
	if len(mgr.Cfg.Accounts) != 1 {
		t.Error("y must remove the account")
	}
}

func TestPaintCoversEveryCell(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	th, _ := theme.Find(theme.Builtin, "dracula")
	out := theme.New(th, true).Paint("a\x1b[0mb", 10, 3)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %q", out)
	}
	bg := "\x1b[48;2;40;42;54m"
	if !strings.Contains(lines[0], "\x1b[0m"+bg+"b") {
		t.Errorf("background must come back after a reset: %q", lines[0])
	}
	for i, l := range lines {
		if !strings.HasPrefix(l, bg) || lipgloss.Width(l) != 10 {
			t.Errorf("line %d isn't painted to full width: %q", i, l)
		}
	}
	if out := theme.New(th, false).Paint("x", 10, 3); out != "x" {
		t.Errorf("auto themes must not paint, got %q", out)
	}
}
