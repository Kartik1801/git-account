package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/hosts"
	"github.com/kartik-dhawan-dev/git-account/internal/keys"
	"github.com/kartik-dhawan-dev/git-account/internal/paths"
)

var hostChoices = []struct{ kind, name, desc string }{
	{"github", "GitHub", "github.com"},
	{"gitlab", "GitLab", "gitlab.com"},
	{"bitbucket", "Bitbucket", "bitbucket.org"},
	{"custom", "Other", "GitHub Enterprise, self-hosted GitLab, Gitea, Forgejo… any host and port"},
}

var addSteps = []string{"You", "Host", "Key", "Connect"}

type addModel struct {
	step                 int
	id, name, email      textinput.Model
	user, hostname, port textinput.Model
	pass, pass2, folder  textinput.Model
	focus                int
	host, keyOpt         int
	existing             []*keys.Info
	err                  string
	f                    *flow
	mismatch, skipped    bool
}

func (w *addModel) all() []*textinput.Model {
	return []*textinput.Model{&w.id, &w.name, &w.email, &w.user, &w.hostname, &w.port, &w.pass, &w.pass2, &w.folder}
}

// inputs are the text fields on the current step.
func (w *addModel) inputs() []*textinput.Model {
	switch w.step {
	case 0:
		return []*textinput.Model{&w.id, &w.name, &w.email}
	case 1:
		if hostChoices[w.host].kind == "custom" {
			return []*textinput.Model{&w.user, &w.hostname, &w.port}
		}
		return []*textinput.Model{&w.user}
	case 2:
		if w.keyOpt < 2 {
			return []*textinput.Model{&w.pass, &w.pass2}
		}
	case 3:
		if w.f != nil && w.f.connected && !w.mismatch {
			return []*textinput.Model{&w.folder}
		}
	}
	return nil
}

func (w *addModel) setFocus(i int) {
	for _, in := range w.all() {
		in.Blur()
	}
	ins := w.inputs()
	if len(ins) == 0 {
		w.focus = 0
		return
	}
	w.focus = (i%len(ins) + len(ins)) % len(ins)
	ins[w.focus].Focus()
}

func (w *addModel) updateFocused(msg tea.KeyMsg) tea.Cmd {
	ins := w.inputs()
	if len(ins) == 0 {
		return nil
	}
	var cmd tea.Cmd
	*ins[w.focus], cmd = ins[w.focus].Update(msg)
	return cmd
}

func (m *Model) startAdd() tea.Cmd {
	name, email := "", ""
	tmp := os.TempDir()
	if v, ok, _ := m.mgr.Git.Get(tmp, "--global", "--get", "user.name"); ok {
		name = v
	}
	if v, ok, _ := m.mgr.Git.Get(tmp, "--global", "--get", "user.email"); ok {
		email = v
	}
	if def, _ := m.mgr.Cfg.Find(m.mgr.Cfg.Default); def != nil && name == "" {
		name = def.Name
	}
	if name == "" {
		name = os.Getenv("USER")
	}
	id := ""
	if len(m.mgr.Cfg.Accounts) == 0 {
		id = "personal"
	}
	w := &addModel{
		id:       m.newInput(id, "work", false),
		name:     m.newInput(name, "Your Name", false),
		email:    m.newInput(email, "you@example.com", false),
		user:     m.newInput("", "your username on the host", false),
		hostname: m.newInput("", "git.example.com", false),
		port:     m.newInput("", "22", false),
		pass:     m.newInput("", "optional, recommended", true),
		pass2:    m.newInput("", "type it again", true),
		folder:   m.newInput("", "~/work (optional)", false),
	}
	m.add = w
	m.scr = scrAdd
	m.clearToast()
	w.setFocus(0)
	return nil
}

func (m *Model) draftAccount(w *addModel, keyPath string) config.Account {
	a := config.Account{
		ID: strings.TrimSpace(w.id.Value()), UID: "draft0000", Name: strings.TrimSpace(w.name.Value()),
		Email: strings.TrimSpace(w.email.Value()), Host: hostChoices[w.host].kind,
		Username: strings.TrimPrefix(strings.TrimSpace(w.user.Value()), "@"), Key: keyPath,
	}
	if a.Host == "custom" {
		a.Hostname = strings.TrimSpace(w.hostname.Value())
		a.Port, _ = strconv.Atoi(strings.TrimSpace(w.port.Value()))
	}
	return a
}

func (m *Model) addNext() tea.Cmd {
	w := m.add
	w.err = ""
	switch w.step {
	case 0:
		id := strings.TrimSpace(w.id.Value())
		if err := config.ValidateID(id); err != nil {
			w.err = capitalize(err.Error())
			w.setFocus(0)
			return nil
		}
		if a, _ := m.mgr.Cfg.Find(id); a != nil {
			w.err = "You already have an account called " + id + "."
			w.setFocus(0)
			return nil
		}
		if err := config.ValidateName(w.name.Value()); err != nil {
			w.err = capitalize(err.Error()) + "."
			w.setFocus(1)
			return nil
		}
		if err := config.ValidateEmail(strings.TrimSpace(w.email.Value())); err != nil {
			w.err = capitalize(err.Error()) + "."
			w.setFocus(2)
			return nil
		}
	case 1:
		kind := hostChoices[w.host].kind
		user := strings.TrimPrefix(strings.TrimSpace(w.user.Value()), "@")
		if user == "" && kind != "custom" {
			w.err = "Add your username on " + hostChoices[w.host].name + ". It's used to check the key lands on the right account."
			w.setFocus(0)
			return nil
		}
		if user != "" {
			if err := config.ValidateUsername(user); err != nil {
				w.err = capitalize(err.Error()) + "."
				w.setFocus(0)
				return nil
			}
		}
		if kind == "custom" {
			if err := config.ValidateHostname(strings.TrimSpace(w.hostname.Value())); err != nil {
				w.err = capitalize(err.Error()) + "."
				w.setFocus(1)
				return nil
			}
			if p := strings.TrimSpace(w.port.Value()); p != "" {
				if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 {
					w.err = "The port is a number from 1 to 65535."
					w.setFocus(2)
					return nil
				}
			}
		}
		// ~/.ssh, plus the folder the old git-account.sh script used.
		w.existing = append(keys.Find(m.mgr.P.SSH), keys.Find(filepath.Join(m.mgr.P.SSH, "git-accounts"))...)
		w.keyOpt = 0
	case 2:
		if w.keyOpt < 2 && w.pass.Value() != w.pass2.Value() {
			w.err = "The passphrases don't match."
			w.setFocus(1)
			return nil
		}
		w.step = 3
		w.setFocus(0)
		return m.startConnect()
	}
	w.step++
	w.setFocus(0)
	return nil
}

func (m *Model) startConnect() tea.Cmd {
	w := m.add
	draft := m.draftAccount(w, "")
	w.mismatch, w.skipped = false, false
	w.f = &flow{who: "add", target: draft, host: hosts.For(draft.Host, draft.Hostname)}
	f := w.f
	if w.keyOpt >= 2 {
		info := w.existing[w.keyOpt-2]
		f.info, f.keyPath = info, info.Path
		f.target.Key = info.Path
		f.log(toneOK, "Using "+m.pretty(info.Path))
		if info.Encrypted && !m.agent[info.Fingerprint] && m.mgr.SSH != nil {
			f.gen = m.nextGen()
			gen := f.gen
			f.log(toneInfo, "Loading it into the agent. ssh-add asks for its passphrase.")
			return tea.ExecProcess(m.mgr.SSH.LoadCmd(info.Path), func(err error) tea.Msg {
				return execDoneMsg{kind: "wizard-load", gen: gen, err: err}
			})
		}
		return m.handOff(f)
	}
	typ := "ed25519"
	if w.keyOpt == 1 {
		typ = "rsa"
	}
	path := m.mgr.NewKeyPath(draft.ID, typ)
	f.target.Key = path
	return m.makeKey(f, path, typ, draft.Email, []byte(w.pass.Value()))
}

// addConnected runs when the host accepted the key.
func (m *Model) addConnected() {
	w := m.add
	want := strings.TrimPrefix(strings.TrimSpace(w.user.Value()), "@")
	got := w.f.gotUser
	switch {
	case got != "" && want != "" && !strings.EqualFold(got, want):
		w.mismatch = true
	case got != "" && want == "":
		w.user.SetValue(got)
	}
	if !w.mismatch && w.folder.Value() == "" {
		w.folder.SetValue(m.suggestFolder())
	}
	w.setFocus(0)
}

// suggestFolder proposes the folder around the current repo, like ~/work.
func (m *Model) suggestFolder() string {
	if m.here.Repo == "" {
		return ""
	}
	parent := paths.Parent(m.here.Repo)
	if paths.Same(parent, m.mgr.P.Home) || paths.IsRoot(parent) {
		return ""
	}
	for _, a := range m.mgr.Cfg.Accounts {
		for _, f := range a.Folders {
			if paths.Within(parent, m.mgr.P.Expand(f)) {
				return ""
			}
		}
	}
	return m.pretty(parent)
}

func (m *Model) addFinish() tea.Cmd {
	w := m.add
	a := m.draftAccount(w, w.f.keyPath)
	a.UID = ""
	if f := strings.TrimSpace(w.folder.Value()); f != "" {
		a.Folders = []string{f}
	}
	if w.f.created {
		a.KeyCreated = time.Now()
	} else if w.f.info != nil {
		a.KeyCreated = w.f.info.Created
	}
	if err := m.mgr.Add(a); err != nil {
		w.err = capitalize(err.Error())
		return nil
	}
	m.add = nil
	m.scr = scrHome
	m.sel = len(m.mgr.Cfg.Accounts) - 1
	msg := "Added " + a.ID + "."
	if len(a.Folders) > 0 {
		msg += " Repos in " + m.prettyFolder(a.Folders[0]) + " use it automatically."
	} else if len(m.mgr.Cfg.Accounts) > 1 {
		msg += " Press enter in a repo to use it there, or f for a folder."
	}
	if w.skipped {
		msg += " It wasn't tested yet: press t."
	}
	m.say(toneOK, msg)
	return m.refresh()
}

func (m *Model) addBack() tea.Cmd {
	w := m.add
	w.err = ""
	switch w.step {
	case 0:
		m.add = nil
		m.scr = scrHome
		m.say(toneDim, "Cancelled. Nothing was changed.")
		return nil
	case 3:
		if w.f != nil {
			w.f.stop(m)
			w.f.discardKey()
		}
		m.busy = ""
		w.f = nil
	}
	w.step--
	w.setFocus(0)
	return nil
}

func (m *Model) keyAdd(msg tea.KeyMsg) tea.Cmd {
	w := m.add
	k := msg.String()
	switch k {
	case "esc":
		return m.addBack()
	case "tab":
		w.setFocus(w.focus + 1)
		return nil
	case "shift+tab":
		w.setFocus(w.focus - 1)
		return nil
	}
	switch w.step {
	case 0:
		switch k {
		case "up":
			w.setFocus(w.focus - 1)
			return nil
		case "down":
			w.setFocus(w.focus + 1)
			return nil
		case "enter":
			if w.focus < 2 {
				w.setFocus(w.focus + 1)
				return nil
			}
			return m.addNext()
		}
	case 1:
		switch k {
		case "up", "down":
			n := len(hostChoices)
			if k == "up" {
				w.host = (w.host - 1 + n) % n
			} else {
				w.host = (w.host + 1) % n
			}
			w.setFocus(0)
			return nil
		case "enter":
			if w.focus < len(w.inputs())-1 {
				w.setFocus(w.focus + 1)
				return nil
			}
			return m.addNext()
		}
	case 2:
		switch k {
		case "up", "down":
			n := 2 + len(w.existing)
			if k == "up" {
				w.keyOpt = (w.keyOpt - 1 + n) % n
			} else {
				w.keyOpt = (w.keyOpt + 1) % n
			}
			w.setFocus(0)
			return nil
		case "enter":
			if w.keyOpt < 2 && w.focus == 0 && w.pass.Value() != "" {
				w.setFocus(1)
				return nil
			}
			return m.addNext()
		}
	case 3:
		return m.keyConnect(k, msg)
	}
	return w.updateFocused(msg)
}

func (m *Model) keyConnect(k string, msg tea.KeyMsg) tea.Cmd {
	w := m.add
	f := w.f
	if f == nil {
		return nil
	}
	switch {
	case f.connected && w.mismatch:
		if k == "u" {
			w.user.SetValue(f.gotUser)
			w.mismatch = false
			f.log(toneOK, "Username set to @"+f.gotUser)
			w.folder.SetValue(m.suggestFolder())
			w.setFocus(0)
		}
		return nil
	case f.connected:
		if k == "enter" {
			return m.addFinish()
		}
		return w.updateFocused(msg)
	}
	if cmd, ok := m.keyFlow(f, k); ok {
		return cmd
	}
	if k == "s" && f.info != nil {
		f.stop(m)
		f.failed = false
		f.connected = true
		w.skipped = true
		f.log(toneWarn, "Saving without checking. Test it later with t.")
		w.folder.SetValue(m.suggestFolder())
		w.setFocus(0)
	}
	return nil
}

func (m *Model) stepper(step int) string {
	s := m.st
	var parts []string
	for i, name := range addSteps {
		switch {
		case i < step:
			parts = append(parts, s.OK.Render("✓ "+name))
		case i == step:
			parts = append(parts, s.Accent.Bold(true).Render("● "+name))
		default:
			parts = append(parts, s.Dim.Render("○ "+name))
		}
	}
	return strings.Join(parts, s.Faint.Render(" ── "))
}

func (m *Model) option(sel bool, title, desc string) string {
	s := m.st
	mark := "  "
	if sel {
		mark = "▸ "
	}
	if desc == "" {
		return m.row(sel, m.cw(), seg{s.Accent, mark}, seg{s.FG, title})
	}
	if len([]rune(title))+len([]rune(desc))+6 <= m.cw() {
		return m.row(sel, m.cw(), seg{s.Accent, mark}, seg{s.FG, title}, seg{s.Dim, "  " + desc})
	}
	return m.row(sel, m.cw(), seg{s.Accent, mark}, seg{s.FG, title}) + "\n" + m.row(sel, m.cw(), seg{s.FG, "  "}, seg{s.Dim, desc})
}

func (m *Model) viewAdd() (string, string) {
	s := m.st
	w := m.add
	b := []string{m.header("New account", s.Dim.Render(itoa(w.step+1)+" / 4")), "", "  " + m.stepper(w.step), ""}
	var ks []key
	switch w.step {
	case 0:
		b = append(b, s.Strong.Render("Who are you on this account?"), s.Dim.Render("Filled in from your git config. Change what you need."), "",
			m.fieldRow("Short name", &w.id), m.fieldRow("Your name", &w.name), m.fieldRow("Email", &w.email))
		ks = []key{{"enter", "next"}, {"tab", "next field"}, {"esc", "cancel"}}
	case 1:
		b = append(b, s.Strong.Render("Where is it hosted?"), "")
		for i, h := range hostChoices {
			b = append(b, m.option(i == w.host, padRight(h.name, 10), h.desc))
		}
		b = append(b, "", m.fieldRow("Username", &w.user))
		if hostChoices[w.host].kind == "custom" {
			b = append(b, m.fieldRow("Host name", &w.hostname), m.fieldRow("SSH port", &w.port))
		}
		ks = []key{{"↑↓", "choose host"}, {"enter", "next"}, {"esc", "back"}}
	case 2:
		id := strings.TrimSpace(w.id.Value())
		b = append(b, s.Strong.Render("Which SSH key?"), "")
		b = append(b, m.option(w.keyOpt == 0, "Create a new ed25519 key (recommended)", "Saved in "+m.pretty(m.mgr.P.KeyDir(id))))
		b = append(b, m.option(w.keyOpt == 1, "Create a new RSA 4096 key", "Only for servers that don't accept ed25519"))
		for i, k := range w.existing {
			desc := k.ShortType() + " · " + shortFP(k.Fingerprint)
			if k.Encrypted {
				desc += " · has a passphrase"
			}
			b = append(b, m.option(w.keyOpt == i+2, "Use "+m.pretty(k.Path), desc))
		}
		if w.keyOpt < 2 {
			b = append(b, "", m.fieldRow("Passphrase", &w.pass), m.fieldRow("Repeat it", &w.pass2),
				s.Dim.Render("  A passphrase protects the key if someone copies it. Leave it empty to skip."))
		}
		ks = []key{{"↑↓", "choose"}, {"tab", "passphrase"}, {"enter", "create and connect"}, {"esc", "back"}}
	case 3:
		f := w.f
		b = append(b, s.Strong.Render("Connect it to "+f.host.Name), "")
		b = append(b, m.viewFlow(f)...)
		switch {
		case f.connected && w.mismatch:
			want := strings.TrimPrefix(strings.TrimSpace(w.user.Value()), "@")
			b = append(b, "", s.Err.Render("✗ ")+s.FG.Render(f.host.Name+" says this key belongs to @"+f.gotUser+", not @"+want+"."),
				m.wrap("You're probably signed in to the wrong "+f.host.Name+" account in your browser. Remove the key there, sign in as @"+want+" and add it again, or keep @"+f.gotUser+".", 2, s.Dim))
			ks = []key{{"u", "use @" + f.gotUser}, {"esc", "back"}}
		case f.connected:
			b = append(b, "", s.Strong.Render("Use it automatically in a folder?"), s.Dim.Render("Every repo inside it will use "+strings.TrimSpace(w.id.Value())+". Leave it empty to skip."), "",
				m.fieldRow("Folder", &w.folder))
			ks = []key{{"enter", "save account"}, {"esc", "back"}}
		case f.failed:
			ks = []key{{"r", "try again"}, {"c", "copy key"}, {"o", "open page"}, {"s", "save without checking"}, {"esc", "back"}}
		default:
			ks = []key{{"c", "copy key again"}, {"o", "open page again"}, {"s", "save without checking"}, {"esc", "back"}}
		}
	}
	if w.err != "" {
		b = append(b, "", s.Err.Render("✗ ")+s.FG.Render(w.err))
	}
	return strings.Join(b, "\n"), m.footer(ks...)
}
