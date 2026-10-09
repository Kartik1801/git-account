package tui

import (
	"context"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/doctor"
)

type checkModel struct {
	gen     int
	local   bool // local checks finished
	results []doctor.Result
	pending int // connection tests still running
	sel     int
}

func (c *checkModel) done() bool { return c.local && c.pending == 0 }

type localMsg struct {
	gen     int
	results []doctor.Result
}

type connMsg struct {
	gen int
	res doctor.Result
}

type fixDoneMsg struct {
	label string
	err   error
}

func (m *Model) startCheck() tea.Cmd {
	c := &checkModel{gen: m.nextGen()}
	m.chk = c
	m.scr = scrCheck
	m.clearToast()
	mgr, gen := m.mgr, c.gen
	accounts := append([]config.Account(nil), mgr.Cfg.Accounts...)
	c.pending = len(accounts)
	cmds := []tea.Cmd{func() tea.Msg { return localMsg{gen: gen, results: doctor.Local(mgr)} }}
	for _, a := range accounts {
		a := a
		cmds = append(cmds, func() tea.Msg { return connMsg{gen: gen, res: doctor.Connection(context.Background(), mgr, a)} })
	}
	return tea.Batch(cmds...)
}

func (m *Model) onLocal(msg localMsg) tea.Cmd {
	c := m.chk
	if c == nil || c.gen != msg.gen {
		return nil
	}
	c.local = true
	c.results = append(msg.results, c.results...)
	m.checkFinished()
	return nil
}

func (m *Model) onConn(msg connMsg) {
	c := m.chk
	if c == nil || c.gen != msg.gen {
		return
	}
	c.pending--
	c.results = append(c.results, msg.res)
	m.checkFinished()
}

func (m *Model) checkFinished() {
	c := m.chk
	if !c.done() {
		return
	}
	doctor.Sort(c.results)
	c.sel = 0
	if n := doctor.Attention(c.results); n > 0 {
		m.say(toneWarn, plural(n, "thing", "things")+" to fix. Pick one and press enter.")
	} else if len(m.mgr.Cfg.Accounts) > 0 {
		m.say(toneOK, "All good. "+plural(len(m.mgr.Cfg.Accounts), "account signs", "accounts, each signs")+" in as the right user.")
	} else {
		m.say(toneOK, "All good.")
	}
}

func (m *Model) onFixDone(msg fixDoneMsg) tea.Cmd {
	m.busy = ""
	_ = m.mgr.Reload()
	cmd := m.startCheck()
	if msg.err != nil {
		m.say(toneErr, "That didn't work: "+msg.err.Error())
	} else {
		m.say(toneOK, "Done: "+msg.label+". Checking again…")
	}
	return tea.Batch(cmd, m.refresh())
}

func (m *Model) keyCheck(k string) tea.Cmd {
	c := m.chk
	switch k {
	case "esc", "q", "left":
		m.scr = scrHome
		m.chk = nil
		m.clearToast()
		return m.refresh()
	case "r":
		m.clearToast()
		return m.startCheck()
	case "up", "k":
		if len(c.results) > 0 {
			c.sel = (c.sel - 1 + len(c.results)) % len(c.results)
		}
	case "down", "j":
		if len(c.results) > 0 {
			c.sel = (c.sel + 1) % len(c.results)
		}
	case "enter", "f":
		if !c.done() || c.sel >= len(c.results) {
			return nil
		}
		r := c.results[c.sel]
		if r.Fix == nil {
			m.say(toneDim, "There's nothing to fix automatically here.")
			return nil
		}
		if r.Fix.Cmd != nil {
			return tea.ExecProcess(r.Fix.Cmd(), func(err error) tea.Msg { return execDoneMsg{kind: "fix", err: err} })
		}
		run, label := r.Fix.Run, r.Fix.Label
		m.busy = label + "…"
		return func() tea.Msg { return fixDoneMsg{label: label, err: run()} }
	}
	return nil
}

func osName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	}
	return "Linux"
}

func (m *Model) viewCheck() (string, string) {
	s := m.st
	c := m.chk
	var lines []string
	selFirst, selLast := 0, 0
	for i, r := range c.results {
		symSt, sym, st := s.OK, "✓", s.FG
		switch r.Level {
		case doctor.Info:
			symSt, sym, st = s.Dim, "·", s.Dim
		case doctor.Warn:
			symSt, sym = s.Warn, "!"
		case doctor.Problem:
			symSt, sym = s.Err, "✗"
		}
		sel := i == c.sel && c.done()
		mark := "  "
		if sel {
			mark = "▸ "
			selFirst = len(lines)
		}
		lines = append(lines, m.row(sel, m.cw(), seg{s.Accent, mark}, seg{symSt, sym}, seg{st, " " + r.Title}))
		if sel {
			if r.Detail != "" {
				lines = append(lines, strings.Split(m.wrap(r.Detail, 4, s.Dim), "\n")...)
			}
			if r.Fix != nil {
				lines = append(lines, "    "+s.Key.Render("enter")+" "+s.Accent.Render(r.Fix.Label))
			}
			selLast = len(lines)
		}
	}
	if !c.done() {
		msg := "Checking…"
		if c.local {
			msg = "Testing connections… " + plural(c.pending, "left", "left")
		}
		lines = append(lines, "", "  "+m.spin.View()+" "+s.FG.Render(msg))
	}

	// Keep the selected result on screen when the list is long.
	room := m.h - 10
	if room < 5 {
		room = 5
	}
	if len(lines) > room {
		start := 0
		if selLast > room {
			start = selLast - room
		}
		if selFirst < start {
			start = selFirst
		}
		end := start + room
		if end > len(lines) {
			end = len(lines)
		}
		lines = lines[start:end]
	}
	body := strings.Join(append([]string{m.header("Check", s.Dim.Render(osName())), ""}, lines...), "\n")
	return body, m.footer(key{"↑↓", "choose"}, key{"enter", "fix"}, key{"r", "run again"}, key{"esc", "back"})
}
