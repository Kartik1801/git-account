package tui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
)

func (m *Model) hereLine() string {
	s := m.st
	h := m.here
	label := s.Dim.Render("Here  ")
	switch h.Kind {
	case "repo":
		return label + m.pill(h.Account) + "  " + s.Dim.Render("set for this repo")
	case "folder":
		return label + m.pill(h.Account) + "  " + s.Dim.Render("automatic in "+m.prettyFolder(h.Folder))
	case "default":
		why := "your default"
		if h.Repo == "" {
			why += " · not in a git repo"
		}
		return label + m.pill(h.Account) + "  " + s.Dim.Render(why)
	case "other":
		return label + s.Warn.Render("not managed") + "  " + s.Dim.Render(h.Email+" comes from "+m.pretty(h.File))
	}
	return label + s.Dim.Render("no account applies here yet")
}

func (m *Model) viewHome() (string, string) {
	s := m.st
	cw := m.cw()
	b := []string{m.header("git-account", s.Dim.Render(m.pretty(m.cwd))), "", m.hereLine()}
	if m.issues > 0 {
		b = append(b, "", s.Warn.Render("! ")+s.FG.Render(plural(m.issues, "thing needs", "things need")+" attention")+s.Dim.Render(" · press c to check and fix"))
	}
	b = append(b, "", s.Label.Render("ACCOUNTS"))
	accts := m.mgr.Cfg.Accounts
	if len(accts) == 0 {
		b = append(b, "  "+s.FG.Render("No accounts yet."), "  "+s.Dim.Render("Press a to add your first one. It takes about a minute."))
	}
	idW, emailW := 0, 0
	for _, a := range accts {
		if l := len(a.ID); l > idW {
			idW = l
		}
		if l := len([]rune(a.Email)); l > emailW {
			emailW = l
		}
	}
	if idW > 18 {
		idW = 18
	}
	for i := range accts {
		b = append(b, m.accountRow(&accts[i], i == m.sel, idW, emailW, cw))
	}
	if len(accts) > 0 {
		b = append(b, s.Faint.Render("    + add account"))
	}
	a := m.selected()
	if a != nil {
		b = append(b, "", m.detailLine(a))
	}
	if m.confirm && a != nil {
		b = append(b, "", s.Err.Render("Remove "+a.ID+"?")+" "+s.Dim.Render("Its key goes to the trash for 30 days, and u brings it back."))
	}

	var ks []key
	switch {
	case m.confirm:
		ks = []key{{"y", "remove"}, {"n", "keep"}}
	case len(accts) == 0:
		ks = []key{{"a", "add account"}, {"c", "check"}, {"s", "theme"}, {"?", "all keys"}, {"q", "quit"}}
	default:
		ks = []key{{"↑↓", "move"}, {"enter", "use here"}, {"→", "actions"}, {"y", "copy key"}, {"a", "add"},
			{"c", "check"}, {"s", "theme"}, {"u", "undo"}, {"?", "all keys"}, {"q", "quit"}}
	}
	return strings.Join(b, "\n"), m.footer(ks...)
}

func (m *Model) accountRow(a *config.Account, sel bool, idW, maxEmail, cw int) string {
	s := m.st
	marker := "  "
	if sel {
		marker = "▸ "
	}
	dot, dotSt := "○", s.Faint
	if m.here.Account == a.ID {
		dot, dotSt = "●", s.OK
	}
	nameSt := s.FG.Bold(true)
	if sel {
		nameSt = s.Accent.Bold(true)
	}
	segs := []seg{{s.Accent, marker}, {dotSt, dot}, {s.FG, " "}, {nameSt, padRight(a.ID, idW)}, {s.FG, "  "}}

	tags := []seg{{s.Dim, m.mgr.Host(a).Name}}
	if a.ID == m.mgr.Cfg.Default {
		tags = append(tags, seg{s.Faint, " · "}, seg{s.Warn, "default"})
	}
	for _, f := range a.Folders {
		tags = append(tags, seg{s.Faint, " · "}, seg{s.Info, "auto " + m.prettyFolder(f)})
	}
	used := 0
	for _, sg := range append(segs, tags...) {
		used += len([]rune(sg.s))
	}
	emailW := cw - used - 2
	if emailW > maxEmail {
		emailW = maxEmail
	}
	email := a.Email
	if emailW < 8 {
		email = ""
		emailW = 0
	} else if len([]rune(email)) > emailW {
		email = string([]rune(email)[:emailW-1]) + "…"
	}
	segs = append(segs, seg{s.Dim, padRight(email, emailW)}, seg{s.FG, "  "})
	segs = append(segs, tags...)
	return m.row(sel, cw, segs...)
}

func (m *Model) detailLine(a *config.Account) string {
	s := m.st
	h := m.mgr.Host(a)
	parts := []string{a.Name}
	if a.Username != "" {
		parts = append(parts, "@"+a.Username+" on "+h.Domain)
	} else {
		parts = append(parts, h.Domain)
	}
	line := "  " + s.Dim.Render(strings.Join(parts, " · "))
	if info := m.infos[a.ID]; info != nil {
		line += s.Dim.Render(" · " + shortFP(info.Fingerprint))
	} else {
		line += s.Err.Render(" · key missing")
	}
	return line + s.Faint.Render("   → for actions")
}

func (m *Model) keyHome(k string) tea.Cmd {
	n := len(m.mgr.Cfg.Accounts)
	a := m.selected()
	if m.confirm {
		switch k {
		case "y":
			m.confirm = false
			return m.removeSelected()
		case "n", "esc":
			m.confirm = false
			if a != nil {
				m.say(toneDim, "Kept "+a.ID+".")
			}
		}
		return nil
	}
	switch k {
	case "a":
		return m.startAdd()
	case "c":
		return m.startCheck()
	case "s":
		m.startTheme()
		return nil
	case "u":
		return m.undo()
	case "?":
		m.scr = scrHelp
		return nil
	case "q":
		return tea.Quit
	}
	if a == nil {
		return nil
	}
	switch k {
	case "up", "k":
		m.sel = (m.sel - 1 + n) % n
		m.clearToast()
	case "down", "j":
		m.sel = (m.sel + 1) % n
		m.clearToast()
	case "enter":
		return m.useHere(a)
	case "right", "l":
		m.act = 0
		m.scr = scrAccount
		m.clearToast()
	case "y":
		return m.copyKey(a)
	case "o":
		return m.openPage(a)
	case "t":
		return m.testCmd(a)
	case "d":
		return m.makeDefault(a)
	case "f":
		m.startFolder(a, scrHome)
	case "x":
		m.confirm = true
		m.clearToast()
	}
	return nil
}

func repoName(here string) string {
	if here == "" {
		return "this repo"
	}
	return filepath.Base(here)
}
