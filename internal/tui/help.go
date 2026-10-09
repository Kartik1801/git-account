package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) viewHelp() (string, string) {
	s := m.st
	group := func(title string, rows ...[2]string) string {
		out := []string{s.Label.Render(strings.ToUpper(title))}
		for _, r := range rows {
			out = append(out, s.Key.Render(padRight(r[0], 9))+s.FG.Render(r[1]))
		}
		return strings.Join(out, "\n")
	}
	home := group("Home",
		[2]string{"↑↓ j k", "move"}, [2]string{"enter", "use in this repo"}, [2]string{"→ l", "account actions"},
		[2]string{"y", "copy public key"}, [2]string{"o", "open the host's key page"}, [2]string{"t", "test connection"},
		[2]string{"f", "automatic folder"}, [2]string{"d", "make default"}, [2]string{"x", "remove"},
		[2]string{"a", "add account"}, [2]string{"c", "check and fix"}, [2]string{"s", "theme"},
		[2]string{"u", "undo the last change"}, [2]string{"q", "quit"})
	acct := group("Account screen",
		[2]string{"letter", "run that action"}, [2]string{"s", "load key into the agent"}, [2]string{"h", "use in this repo"},
		[2]string{"e", "edit name and email"}, [2]string{"n", "rename"}, [2]string{"r", "replace key"}, [2]string{"esc ←", "back"})
	forms := group("Forms",
		[2]string{"enter", "next or save"}, [2]string{"tab", "next field"}, [2]string{"esc", "back or cancel"})
	cmds := group("Same things as commands",
		[2]string{"use", "git-account use work"}, [2]string{"copy", "git-account copy-key work"}, [2]string{"check", "git-account check --fix"},
		[2]string{"all", "git-account help"})
	right := acct + "\n\n" + forms + "\n\n" + cmds
	body := m.header("Keys", s.Dim.Render("esc back")) + "\n\n"
	if m.cw() >= 84 {
		body += lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(40).Render(home), right)
	} else {
		body += home + "\n\n" + right
	}
	return body, m.footer(key{"esc", "back"})
}
