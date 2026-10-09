package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
)

// formModel is a small screen of fields: automatic folders, edit, rename.
type formModel struct {
	kind     string // folder, edit, rename
	id       string
	title    string
	question string
	hint     string
	labels   []string
	fields   []textinput.Model
	focus    int
	err      string
	back     screen
	onList   bool // folder form: the list of existing folders has focus
	listSel  int
}

func (f *formModel) setFocus(i int) {
	for j := range f.fields {
		f.fields[j].Blur()
	}
	f.focus = (i%len(f.fields) + len(f.fields)) % len(f.fields)
	if !f.onList {
		f.fields[f.focus].Focus()
	}
}

func (m *Model) startFolder(a *config.Account, back screen) {
	f := &formModel{kind: "folder", id: a.ID, back: back, title: "Automatic folders · " + a.ID,
		question: "Use " + a.ID + " automatically in…",
		hint:     "Every repo inside this folder, now and later. It works in your IDE too, because git itself picks the account.",
		labels:   []string{"Folder"}, fields: []textinput.Model{m.newInput(m.suggestFolder(), "~/work", false)}}
	f.setFocus(0)
	m.form = f
	m.scr = scrForm
	m.clearToast()
}

func (m *Model) startEdit(a *config.Account) {
	f := &formModel{kind: "edit", id: a.ID, back: scrAccount, title: "Edit · " + a.ID,
		question: "Name and email for new commits", hint: "Commits you already made keep their author.",
		labels: []string{"Your name", "Email", "Username"},
		fields: []textinput.Model{m.newInput(a.Name, "Your Name", false), m.newInput(a.Email, "you@example.com", false), m.newInput(a.Username, "username on the host", false)}}
	f.setFocus(0)
	m.form = f
	m.scr = scrForm
	m.clearToast()
}

func (m *Model) startRename(a *config.Account) {
	f := &formModel{kind: "rename", id: a.ID, back: scrAccount, title: "Rename · " + a.ID,
		question: "New short name", hint: "Folders, the default and repos using it all follow the new name.",
		labels: []string{"Short name"}, fields: []textinput.Model{m.newInput(a.ID, "work", false)}}
	f.setFocus(0)
	m.form = f
	m.scr = scrForm
	m.clearToast()
}

func (m *Model) closeForm() {
	m.scr = m.form.back
	m.form = nil
}

func (m *Model) keyForm(msg tea.KeyMsg) tea.Cmd {
	f := m.form
	a, _ := m.mgr.Cfg.Find(f.id)
	if a == nil {
		m.form = nil
		m.scr = scrHome
		return nil
	}
	k := msg.String()
	if k == "esc" {
		m.closeForm()
		return nil
	}
	if f.kind == "folder" && k == "tab" && len(a.Folders) > 0 {
		f.onList = !f.onList
		f.listSel = 0
		f.setFocus(0)
		return nil
	}
	if f.onList {
		switch k {
		case "up", "k":
			f.listSel = (f.listSel - 1 + len(a.Folders)) % len(a.Folders)
		case "down", "j":
			f.listSel = (f.listSel + 1) % len(a.Folders)
		case "x", "delete", "backspace":
			folder := a.Folders[f.listSel]
			if err := m.mgr.RemoveFolder(a.ID, folder); err != nil {
				f.err = capitalize(err.Error())
				return nil
			}
			m.say(toneOK, "Repos in "+m.prettyFolder(folder)+" no longer choose "+a.ID+".")
			if a2, _ := m.mgr.Cfg.Find(f.id); a2 == nil || len(a2.Folders) == 0 {
				f.onList = false
				f.setFocus(0)
			} else if f.listSel >= len(a2.Folders) {
				f.listSel = len(a2.Folders) - 1
			}
			return m.refresh()
		}
		return nil
	}
	switch k {
	case "tab", "down":
		f.setFocus(f.focus + 1)
		return nil
	case "shift+tab", "up":
		f.setFocus(f.focus - 1)
		return nil
	case "enter":
		if f.focus < len(f.fields)-1 {
			f.setFocus(f.focus + 1)
			return nil
		}
		return m.submitForm(a)
	}
	var cmd tea.Cmd
	f.fields[f.focus], cmd = f.fields[f.focus].Update(msg)
	return cmd
}

func (m *Model) submitForm(a *config.Account) tea.Cmd {
	f := m.form
	f.err = ""
	val := func(i int) string { return strings.TrimSpace(f.fields[i].Value()) }
	switch f.kind {
	case "folder":
		if val(0) == "" {
			m.closeForm()
			return nil
		}
		stored, err := m.mgr.AddFolder(a.ID, val(0))
		if err != nil {
			f.err = capitalize(err.Error())
			return nil
		}
		m.say(toneOK, "Repos in "+m.prettyFolder(stored)+" now use "+a.ID+" automatically.")
	case "edit":
		if err := config.ValidateName(val(0)); err != nil {
			f.err = capitalize(err.Error()) + "."
			return nil
		}
		if err := config.ValidateEmail(val(1)); err != nil {
			f.err = capitalize(err.Error()) + "."
			return nil
		}
		user := strings.TrimPrefix(val(2), "@")
		if user != "" {
			if err := config.ValidateUsername(user); err != nil {
				f.err = capitalize(err.Error()) + "."
				return nil
			}
		}
		if err := m.mgr.Edit(a.ID, val(0), val(1), user); err != nil {
			f.err = capitalize(err.Error())
			return nil
		}
		m.say(toneOK, "New commits with "+a.ID+" use "+val(0)+" <"+val(1)+">.")
	case "rename":
		if err := m.mgr.Rename(a.ID, val(0)); err != nil {
			f.err = capitalize(err.Error())
			return nil
		}
		m.say(toneOK, "Renamed "+f.id+" to "+val(0)+".")
	}
	m.closeForm()
	return m.refresh()
}

func (m *Model) viewForm() (string, string) {
	s := m.st
	f := m.form
	b := []string{m.header(f.title, s.Dim.Render("esc back")), "", s.Strong.Render(f.question), m.wrap(f.hint, 0, s.Dim), ""}
	for i := range f.fields {
		b = append(b, m.fieldRow(f.labels[i], &f.fields[i]))
	}
	ks := []key{{"enter", "save"}, {"tab", "next field"}, {"esc", "back"}}
	if f.kind == "folder" {
		ks = []key{{"enter", "add folder"}, {"esc", "back"}}
		if a, _ := m.mgr.Cfg.Find(f.id); a != nil && len(a.Folders) > 0 {
			b = append(b, "", s.Label.Render("ALREADY SET"))
			for i, folder := range a.Folders {
				sel := f.onList && i == f.listSel
				mark := "  "
				if sel {
					mark = "▸ "
				}
				b = append(b, m.row(sel, m.cw(), seg{s.Accent, mark}, seg{s.Info, m.prettyFolder(folder)}))
			}
			if f.onList {
				ks = []key{{"↑↓", "choose"}, {"x", "remove folder"}, {"tab", "back to adding"}, {"esc", "back"}}
			} else {
				ks = []key{{"enter", "add folder"}, {"tab", "manage folders"}, {"esc", "back"}}
			}
		}
	}
	if f.err != "" {
		b = append(b, "", s.Err.Render("✗ ")+s.FG.Render(f.err))
	}
	return strings.Join(b, "\n"), m.footer(ks...)
}
