package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/kartik-dhawan-dev/git-account/internal/doctor"
)

// TestDump writes every screen as plain text for a human to look at.
// Run with DUMP=dir.
func TestDump(t *testing.T) {
	dir := os.Getenv("DUMP")
	if dir == "" {
		t.Skip("set DUMP to a folder")
	}
	mgr, home := sandbox(t)
	repo := filepath.Join(home, "work", "payments-api")
	os.MkdirAll(repo, 0o755)
	mgr.Git.Run(repo, "init", "-q")
	m := New(mgr, repo, true)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
	m.Update(m.loadCmd()())
	save := func(name string) {
		v := ansi.Strip(m.View())
		var ls []string
		for _, l := range strings.Split(v, "\n") {
			ls = append(ls, strings.TrimRight(l, " "))
		}
		os.WriteFile(filepath.Join(dir, name+".txt"), []byte(strings.Join(ls, "\n")), 0o644)
	}
	save("01-home")
	press(t, m, "right")
	save("02-account")
	press(t, m, "esc", "a")
	save("03-add-you")
	m.add.id.SetValue("client-x")
	m.add.email.SetValue("kartik@clientx.example")
	press(t, m, "enter", "enter", "enter", "down")
	m.add.user.SetValue("kartik.cx")
	save("04-add-host")
	press(t, m, "enter")
	save("05-add-key")
	press(t, m, "esc", "esc", "esc")
	m.chk = &checkModel{gen: 7, pending: 1}
	m.scr = scrCheck
	m.Update(localMsg{gen: 7, results: doctor.Local(mgr)})
	save("06-check-running")
	m.Update(connMsg{gen: 7, res: doctor.Result{Level: doctor.Problem, Title: "work: github.com didn't accept the key", Detail: "Add it on GitHub's SSH keys page. The fix copies the key and opens the page.", Fix: &doctor.Fix{Label: "Copy the key and open the page"}}})
	save("07-check-done")
	press(t, m, "esc", "s", "down", "down")
	save("08-theme")
	press(t, m, "esc", "?")
	save("09-help")
	press(t, m, "esc", "right", "f")
	save("10-folder")
}
