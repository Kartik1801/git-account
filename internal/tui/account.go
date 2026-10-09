package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
)

type action struct {
	key, label string
	danger     bool
}

func (m *Model) actions(a *config.Account) []action {
	h := m.mgr.Host(a)
	list := []action{{key: "y", label: "Copy public key"}, {key: "o", label: "Open " + h.Name + " SSH key page"}, {key: "t", label: "Test connection"}}
	if info := m.infos[a.ID]; info != nil && info.Encrypted && !m.agent[info.Fingerprint] {
		list = append(list, action{key: "s", label: "Load key into the agent"})
	}
	if m.here.Repo != "" {
		list = append(list, action{key: "h", label: "Use in this repo"})
	}
	return append(list,
		action{key: "f", label: "Automatic folders"},
		action{key: "d", label: "Make default"},
		action{key: "e", label: "Edit name and email"},
		action{key: "n", label: "Rename"},
		action{key: "r", label: "Replace key"},
		action{key: "x", label: "Remove account", danger: true},
	)
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "min ago", "min ago")
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour ago", "hours ago")
	}
	return t.Format("2 Jan")
}

func (m *Model) viewAccount() (string, string) {
	s := m.st
	a := m.selected()
	if a == nil {
		m.scr = scrHome
		return m.viewHome()
	}
	h := m.mgr.Host(a)
	title := s.Strong.Render(a.ID)
	if m.here.Account == a.ID {
		title += " " + m.pill("here")
	}
	if m.mgr.Cfg.Default == a.ID {
		title += " " + s.PillWarn.Render(" default ")
	}
	head := m.spread(s.Accent.Render("◆")+" "+title, s.Dim.Render("esc back"))

	info := m.infos[a.ID]
	host := h.Domain
	if a.Username != "" {
		host += " · @" + a.Username
	}
	type kv struct {
		k string
		v string
	}
	rows := []kv{{"Name", s.FG.Render(a.Name)}, {"Email", s.FG.Render(a.Email)}, {"Host", s.FG.Render(host)},
		{"Key", s.FG.Render(m.pretty(m.mgr.KeyPath(a)))}}
	if info != nil {
		pass := "no passphrase"
		if info.Encrypted {
			pass = "passphrase"
		}
		rows = append(rows, kv{"Fingerprint", s.FG.Render(midFP(info.Fingerprint))}, kv{"Type", s.FG.Render(info.ShortType() + ", " + pass)})
		created := a.KeyCreated
		if created.IsZero() {
			created = info.Created
		}
		c := s.FG.Render(created.Format("Jan 2006"))
		if time.Since(created) > 365*24*time.Hour {
			c += s.Warn.Render(" · over a year old")
		}
		rows = append(rows, kv{"Created", c})
		switch {
		case !info.Encrypted:
			rows = append(rows, kv{"Agent", s.Dim.Render("not needed")})
		case m.agent[info.Fingerprint]:
			rows = append(rows, kv{"Agent", s.OK.Render("✓ loaded")})
		default:
			rows = append(rows, kv{"Agent", s.Warn.Render("! not loaded")})
		}
	} else {
		rows = append(rows, kv{"Status", s.Err.Render("✗ key file missing")})
	}
	var fs []string
	for _, f := range a.Folders {
		fs = append(fs, m.prettyFolder(f))
	}
	if len(fs) > 0 {
		rows = append(rows, kv{"Automatic", s.Info.Render(strings.Join(fs, ", "))})
	} else {
		rows = append(rows, kv{"Automatic", s.Dim.Render("no folders")})
	}
	if t, ok := m.tests[a.ID]; ok {
		v := s.Err.Render("✗ " + t.res.Message)
		if t.res.OK {
			v = s.OK.Render("✓ ok")
			if t.res.Username != "" {
				v = s.OK.Render("✓ Hi @" + t.res.Username)
			}
		}
		rows = append(rows, kv{"Last test", v + s.Dim.Render(" · "+ago(t.at))})
	}

	twoCol := m.cw() >= 84
	leftW := m.cw()
	if twoCol {
		leftW = m.cw()*56/100 - 1
	}
	var info_ []string
	for _, r := range rows {
		info_ = append(info_, s.Dim.Render(padRight(r.k, 12))+r.v)
	}
	infoBox := s.Box.Width(leftW - 2).Render(strings.Join(info_, "\n"))

	acts := m.actions(a)
	if m.act >= len(acts) {
		m.act = len(acts) - 1
	}
	actW := m.cw() - leftW - 2
	if !twoCol {
		actW = m.cw()
	}
	var al []string
	for i, ac := range acts {
		st := s.FG
		if ac.danger {
			st = s.Err
		}
		mark := "  "
		if i == m.act {
			mark = "▸ "
		}
		label := padRight(ac.label, actW-6)
		al = append(al, m.row(i == m.act, actW, seg{s.Accent, mark}, seg{st, label}, seg{s.Key, " " + ac.key}))
	}
	actBlock := strings.Join(al, "\n")

	body := head + "\n\n"
	if twoCol {
		body += lipgloss.JoinHorizontal(lipgloss.Top, infoBox, "  ", actBlock)
	} else {
		body += infoBox + "\n\n" + actBlock
	}
	if m.confirm {
		body += "\n\n" + s.Err.Render("Remove "+a.ID+"?") + " " + s.Dim.Render("Its key goes to the trash for 30 days, and u on the home screen brings it back.")
	}
	ks := []key{{"↑↓", "choose"}, {"enter", "run"}, {"letter", "run directly"}, {"esc", "back"}}
	if m.confirm {
		ks = []key{{"y", "remove"}, {"n", "keep"}}
	}
	return body, m.footer(ks...)
}

func (m *Model) keyAccount(k string) tea.Cmd {
	a := m.selected()
	if a == nil {
		m.scr = scrHome
		return nil
	}
	acts := m.actions(a)
	if m.confirm {
		switch k {
		case "y":
			m.confirm = false
			return m.removeSelected()
		case "n", "esc":
			m.confirm = false
			m.say(toneDim, "Kept "+a.ID+".")
		}
		return nil
	}
	switch k {
	case "up", "k":
		m.act = (m.act - 1 + len(acts)) % len(acts)
	case "down", "j":
		m.act = (m.act + 1) % len(acts)
	case "esc", "left", "backspace", "q":
		m.scr = scrHome
		m.clearToast()
	case "enter":
		return m.runAction(a, acts[m.act].key)
	default:
		for i, ac := range acts {
			if ac.key == k {
				m.act = i
				return m.runAction(a, k)
			}
		}
	}
	return nil
}

func (m *Model) runAction(a *config.Account, k string) tea.Cmd {
	switch k {
	case "y":
		return m.copyKey(a)
	case "o":
		return m.openPage(a)
	case "t":
		return m.testCmd(a)
	case "s":
		return m.loadKey(a)
	case "h":
		return m.useHere(a)
	case "f":
		m.startFolder(a, scrAccount)
	case "d":
		return m.makeDefault(a)
	case "e":
		m.startEdit(a)
	case "n":
		m.startRename(a)
	case "r":
		m.startRotate(a)
	case "x":
		m.confirm = true
		m.clearToast()
	}
	return nil
}
