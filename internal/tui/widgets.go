package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// cw is the width content is laid out in.
func (m *Model) cw() int {
	w := m.w - 4
	if w > 104 {
		w = 104
	}
	if w < 30 {
		w = 30
	}
	return w
}

func (m *Model) header(title, right string) string {
	s := m.st
	left := s.Accent.Render("◆") + " " + s.Strong.Render(title)
	return m.spread(left, right)
}

// spread puts left and right at the two edges of the content width.
func (m *Model) spread(left, right string) string {
	gap := m.cw() - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		max := m.cw() - lipgloss.Width(left) - 2
		if max < 4 {
			return left
		}
		right = ansi.Truncate(right, max, "…")
		gap = m.cw() - lipgloss.Width(left) - lipgloss.Width(right)
	}
	return left + strings.Repeat(" ", gap) + right
}

type key struct{ k, label string }

// footer is the message line, a rule and the key hints.
func (m *Model) footer(ks ...key) string {
	s := m.st
	var lines []string
	cur := ""
	for _, k := range ks {
		item := s.Key.Render(k.k) + " " + s.Dim.Render(k.label)
		if cur != "" && lipgloss.Width(cur)+3+lipgloss.Width(item) > m.cw() {
			lines = append(lines, cur)
			cur = item
			continue
		}
		if cur != "" {
			cur += "   "
		}
		cur += item
	}
	lines = append(lines, cur)
	rule := s.Faint.Render(strings.Repeat("─", m.cw()))
	return m.toastView() + "\n" + rule + "\n" + strings.Join(lines, "\n")
}

func (m *Model) toastView() string {
	s := m.st
	if m.busy != "" {
		return m.spin.View() + " " + s.FG.Render(m.busy)
	}
	if m.toast == "" {
		return ""
	}
	sym, st := "", s.Dim
	switch m.tone {
	case toneOK:
		sym, st = s.OK.Render("✓ "), s.FG
	case toneWarn:
		sym, st = s.Warn.Render("! "), s.FG
	case toneErr:
		sym, st = s.Err.Render("✗ "), s.FG
	case toneInfo:
		sym, st = s.Info.Render("→ "), s.FG
	}
	text := lipgloss.NewStyle().Width(m.cw() - 2).Render(m.toast)
	ls := strings.Split(text, "\n")
	if len(ls) > 3 {
		ls = ls[:3]
	}
	for i := range ls {
		ls[i] = st.Render(strings.TrimRight(ls[i], " "))
	}
	return sym + strings.Join(ls, "\n  ")
}

// frame lays body out at the top and the footer at the bottom of the
// screen, then paints the theme background.
func (m *Model) frame(body, foot string) string {
	margin := "  "
	if m.w < 60 {
		margin = " "
	}
	bl := strings.Split(body, "\n")
	fl := strings.Split(foot, "\n")
	avail := m.h - 2 - len(fl)
	if avail < 1 {
		avail = 1
	}
	if len(bl) > avail {
		bl = append(bl[:avail-1], m.st.Faint.Render("…"))
	}
	for len(bl) < avail {
		bl = append(bl, "")
	}
	lines := append([]string{""}, bl...)
	lines = append(lines, fl...)
	for i := range lines {
		lines[i] = margin + lines[i]
	}
	return m.st.Paint(strings.Join(lines, "\n"), m.w, m.h)
}

type seg struct {
	st lipgloss.Style
	s  string
}

// row renders colored segments as one line of the given width, with the
// selection background when sel is true.
func (m *Model) row(sel bool, width int, segs ...seg) string {
	var b strings.Builder
	for _, sg := range segs {
		st := sg.st
		if sel {
			st = m.st.Sel(st)
		}
		b.WriteString(st.Render(sg.s))
	}
	out := b.String()
	w := lipgloss.Width(out)
	switch {
	case w < width:
		pad := lipgloss.NewStyle()
		if sel {
			pad = m.st.Sel(pad)
		}
		out += pad.Render(strings.Repeat(" ", width-w))
	case w > width:
		out = ansi.Truncate(out, width, "…")
	}
	return out
}

func (m *Model) newInput(value, placeholder string, secret bool) textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.SetValue(value)
	in.Placeholder = placeholder
	in.CharLimit = 250
	in.Width = m.cw() - 20
	if in.Width < 16 {
		in.Width = 16
	}
	in.TextStyle = m.st.Strong
	in.PlaceholderStyle = m.st.Faint
	in.Cursor.Style = m.st.Accent
	in.Cursor.SetMode(cursor.CursorStatic)
	if secret {
		in.EchoMode = textinput.EchoPassword
		in.EchoCharacter = '•'
	}
	return in
}

func (m *Model) fieldRow(label string, in *textinput.Model) string {
	s := m.st
	mark := "  "
	if in.Focused() {
		mark = s.Accent.Render("› ")
	}
	return mark + s.Dim.Render(padRight(label, 14)) + in.View()
}

func (m *Model) pill(text string) string { return m.st.Pill.Render(" " + text + " ") }

func padRight(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return itoa(n) + " " + many
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func shortFP(fp string) string {
	if len(fp) > 19 {
		return fp[:19] + "…"
	}
	return fp
}

// midFP keeps both ends of a fingerprint, which is what people compare
// against the host's list.
func midFP(fp string) string {
	if len(fp) <= 36 {
		return fp
	}
	return fp[:20] + "…" + fp[len(fp)-10:]
}

func shortKey(line string) string {
	f := strings.Fields(line)
	if len(f) < 2 {
		return line
	}
	b := f[1]
	if len(b) > 16 {
		b = b[:10] + "…" + b[len(b)-4:]
	}
	return f[0] + " " + b
}

func (m *Model) pretty(p string) string       { return m.mgr.P.Pretty(p) }
func (m *Model) prettyFolder(f string) string { return m.mgr.P.Pretty(m.mgr.P.Expand(f)) }

// wrap breaks text to the content width, indenting continuation lines.
func (m *Model) wrap(text string, indent int, st lipgloss.Style) string {
	w := m.cw() - indent
	if w < 20 {
		w = 20
	}
	ls := strings.Split(lipgloss.NewStyle().Width(w).Render(text), "\n")
	pad := strings.Repeat(" ", indent)
	for i := range ls {
		ls[i] = pad + st.Render(strings.TrimRight(ls[i], " "))
	}
	return strings.Join(ls, "\n")
}
