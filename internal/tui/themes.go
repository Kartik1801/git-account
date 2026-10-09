package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartik-dhawan-dev/git-account/internal/theme"
)

type themeOpt struct {
	id, name, note string
}

func (m *Model) themeOpts() []themeOpt {
	auto, _ := theme.Pick(m.themes, "auto", m.dark)
	opts := []themeOpt{{"auto", "Auto", "now " + auto.Name}}
	for _, th := range m.themes {
		note := th.Kind
		if strings.Contains(strings.ToLower(th.Name), th.Kind) {
			note = "" // "Solarized Dark", not "Solarized Dark dark"
		}
		if th.Custom {
			note += ", your theme"
		}
		opts = append(opts, themeOpt{th.ID, th.Name, note})
	}
	return opts
}

func (m *Model) currentTheme() string {
	if m.mgr.Cfg.Theme == "" {
		return "auto"
	}
	return m.mgr.Cfg.Theme
}

func (m *Model) startTheme() {
	m.thSel = 0
	for i, o := range m.themeOpts() {
		if o.id == m.currentTheme() {
			m.thSel = i
		}
	}
	m.scr = scrTheme
	m.clearToast()
}

func (m *Model) keyTheme(k string) tea.Cmd {
	opts := m.themeOpts()
	switch k {
	case "up", "k":
		m.thSel = (m.thSel - 1 + len(opts)) % len(opts)
	case "down", "j":
		m.thSel = (m.thSel + 1) % len(opts)
	case "enter":
		o := opts[m.thSel]
		if err := m.mgr.SetTheme(o.id); err != nil {
			m.fail(err)
			return nil
		}
		m.applyTheme(o.id)
		m.scr = scrHome
		m.say(toneOK, "Theme: "+o.name+".")
		return nil
	case "esc", "q", "s":
		m.applyTheme(m.currentTheme())
		m.scr = scrHome
		m.say(toneDim, "Theme unchanged.")
		return nil
	default:
		return nil
	}
	m.applyTheme(opts[m.thSel].id)
	return nil
}

func (m *Model) viewTheme() (string, string) {
	s := m.st
	opts := m.themeOpts()
	listW := 44
	if m.cw() < 84 {
		listW = m.cw()
	}
	var list []string
	for i, o := range opts {
		sel := i == m.thSel
		mark := "  "
		if sel {
			mark = "▸ "
		}
		cur := " "
		if o.id == m.currentTheme() {
			cur = "●"
		}
		th, _ := theme.Pick(m.themes, o.id, m.dark)
		sw := theme.Swatch(th)
		left := m.row(sel, listW-6, seg{s.Accent, mark}, seg{s.OK, cur}, seg{s.FG, " " + o.name + " "}, seg{s.Dim, o.note})
		list = append(list, left+" "+sw)
	}
	// Scroll the list with the selection when it doesn't fit.
	if room := max(m.h-13, 6); len(list) > room {
		start := min(max(m.thSel-room/2, 0), len(list)-room)
		window := append([]string(nil), list[start:start+room]...)
		if start > 0 {
			window[0] = s.Dim.Render("  ↑ " + itoa(start) + " more")
		}
		if end := start + room; end < len(list) {
			window[room-1] = s.Dim.Render("  ↓ " + itoa(len(list)-end) + " more")
		}
		list = window
	}
	listBlock := strings.Join(list, "\n")

	preview := []string{
		s.Label.Render("PREVIEW"),
		m.row(true, 34, seg{s.Accent, "▸ "}, seg{s.OK, "●"}, seg{s.Accent.Bold(true), " work  "}, seg{s.Dim, "GitHub · "}, seg{s.Info, "auto ~/work"}),
		m.row(false, 34, seg{s.Accent, "  "}, seg{s.Faint, "○"}, seg{s.FG.Bold(true), " personal  "}, seg{s.Dim, "GitHub · "}, seg{s.Warn, "default"}),
		"",
		s.OK.Render("✓") + s.FG.Render(" github.com says Hi @you!"),
		s.Warn.Render("!") + s.FG.Render(" a key isn't loaded in the agent"),
		s.Err.Render("✗") + s.FG.Render(" Permission denied (publickey)"),
		"",
		s.Key.Render("enter") + " " + s.Dim.Render("use here") + "   " + s.Key.Render("y") + " " + s.Dim.Render("copy key"),
	}
	previewBox := s.Box.Render(strings.Join(preview, "\n"))

	body := m.header("Theme", s.Dim.Render("saved in config.toml")) + "\n\n"
	if m.cw() >= 84 {
		body += lipgloss.JoinHorizontal(lipgloss.Top, listBlock, "   ", previewBox)
	} else {
		body += listBlock
	}
	body += "\n\n" + m.wrap("Light themes are made for light terminals, and Auto picks for you. To add your own, put a .toml file in "+m.pretty(m.mgr.P.ThemesDir())+" with the same keys as the built-in themes.", 0, s.Dim)
	return body, m.footer(key{"↑↓", "preview"}, key{"enter", "use this theme"}, key{"esc", "cancel"})
}
