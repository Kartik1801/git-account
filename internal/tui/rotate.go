package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
)

// rotateModel replaces an account's key: the new key must work before the
// old one is retired.
type rotateModel struct {
	id          string
	stage       int // 0 passphrase, 1 connecting
	pass, pass2 textinput.Model
	focus       int
	f           *flow
	oldFP       string
	done        bool
	err         string
}

func (m *Model) startRotate(a *config.Account) {
	r := &rotateModel{id: a.ID, pass: m.newInput("", "optional", true), pass2: m.newInput("", "type it again", true)}
	if info := m.infos[a.ID]; info != nil {
		r.oldFP = info.Fingerprint
	}
	r.setFocus(0)
	m.rot = r
	m.scr = scrRotate
	m.clearToast()
}

func (r *rotateModel) setFocus(i int) {
	r.pass.Blur()
	r.pass2.Blur()
	r.focus = (i%2 + 2) % 2
	if r.focus == 0 {
		r.pass.Focus()
	} else {
		r.pass2.Focus()
	}
}

func (m *Model) keyRotate(msg tea.KeyMsg) tea.Cmd {
	r := m.rot
	k := msg.String()
	if r.stage == 0 {
		switch k {
		case "esc":
			m.rot = nil
			m.scr = scrAccount
			return nil
		case "tab", "down":
			r.setFocus(r.focus + 1)
			return nil
		case "shift+tab", "up":
			r.setFocus(r.focus - 1)
			return nil
		case "enter":
			if r.focus == 0 && r.pass.Value() != "" {
				r.setFocus(1)
				return nil
			}
			if r.pass.Value() != r.pass2.Value() {
				r.err = "The passphrases don't match."
				return nil
			}
			return m.rotateStart()
		}
		var cmd tea.Cmd
		if r.focus == 0 {
			r.pass, cmd = r.pass.Update(msg)
		} else {
			r.pass2, cmd = r.pass2.Update(msg)
		}
		return cmd
	}
	f := r.f
	if r.done {
		switch k {
		case "enter", "esc":
			m.rot = nil
			m.scr = scrAccount
			return m.refresh()
		case "o":
			m.keyFlow(f, "o")
		}
		return nil
	}
	if k == "esc" {
		f.stop(m)
		f.discardKey()
		m.busy = ""
		m.rot = nil
		m.scr = scrAccount
		m.say(toneDim, "Cancelled. The old key is unchanged.")
		return nil
	}
	cmd, _ := m.keyFlow(f, k)
	return cmd
}

func (m *Model) rotateStart() tea.Cmd {
	r := m.rot
	a, _ := m.mgr.Cfg.Find(r.id)
	if a == nil {
		m.rot = nil
		m.scr = scrHome
		return nil
	}
	r.stage = 1
	r.err = ""
	typ := "ed25519"
	if info := m.infos[a.ID]; info != nil && info.ShortType() == "rsa" {
		typ = "rsa"
	}
	path := m.mgr.NewKeyPath(a.ID, typ)
	target := *a
	target.Key = path
	r.f = &flow{who: "rotate", target: target, host: m.mgr.Host(a)}
	return m.makeKey(r.f, path, typ, a.Email, []byte(r.pass.Value()))
}

func (m *Model) rotateConnected() tea.Cmd {
	r := m.rot
	f := r.f
	a, _ := m.mgr.Cfg.Find(r.id)
	if a == nil {
		return nil
	}
	if f.gotUser != "" && a.Username != "" && !strings.EqualFold(f.gotUser, a.Username) {
		f.connected, f.failed = false, true
		f.log(toneErr, "The new key signs in as @"+f.gotUser+", not @"+a.Username+". Remove it from that account, add it to @"+a.Username+", then press r.")
		return nil
	}
	if err := m.mgr.ReplaceKey(a.ID, f.keyPath); err != nil {
		f.failed = true
		f.log(toneErr, "Couldn't switch to the new key: "+err.Error())
		return nil
	}
	f.created = false
	r.done = true
	f.log(toneOK, "The old key is in the trash for 30 days")
	if r.oldFP != "" {
		f.log(toneWarn, "Delete the old key on "+f.host.Name+" too. Its fingerprint is "+r.oldFP)
	}
	return m.refresh()
}

func (m *Model) viewRotate() (string, string) {
	s := m.st
	r := m.rot
	b := []string{m.header("Replace key · "+r.id, s.Dim.Render("esc cancel")), ""}
	var ks []key
	if r.stage == 0 {
		b = append(b, s.Strong.Render("Replace "+r.id+"'s SSH key"),
			s.Dim.Render("The old key keeps working until the new one is confirmed on the host."), "",
			m.fieldRow("Passphrase", &r.pass), m.fieldRow("Repeat it", &r.pass2),
			s.Dim.Render("  Optional. Leave it empty for a key without a passphrase."))
		ks = []key{{"enter", "create the new key"}, {"tab", "next field"}, {"esc", "cancel"}}
	} else {
		f := r.f
		b = append(b, s.Strong.Render("Add the new key to "+f.host.Name), "")
		b = append(b, m.viewFlow(f)...)
		switch {
		case r.done:
			ks = []key{{"enter", "done"}, {"o", "open " + f.host.Name + " to delete the old key"}}
		case f.failed:
			ks = []key{{"r", "try again"}, {"c", "copy key"}, {"o", "open page"}, {"esc", "cancel"}}
		default:
			ks = []key{{"c", "copy key again"}, {"o", "open page again"}, {"esc", "cancel"}}
		}
	}
	if r.err != "" {
		b = append(b, "", s.Err.Render("✗ ")+s.FG.Render(r.err))
	}
	return strings.Join(b, "\n"), m.footer(ks...)
}
