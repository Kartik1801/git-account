// Package tui is the terminal app you get by running `git-account`.
package tui

import (
	"context"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/doctor"
	"github.com/kartik-dhawan-dev/git-account/internal/hosts"
	"github.com/kartik-dhawan-dev/git-account/internal/keys"
	"github.com/kartik-dhawan-dev/git-account/internal/store"
	"github.com/kartik-dhawan-dev/git-account/internal/theme"
)

type screen int

const (
	scrHome screen = iota
	scrAccount
	scrAdd
	scrForm
	scrRotate
	scrCheck
	scrTheme
	scrHelp
)

type tone int

const (
	toneDim tone = iota
	toneOK
	toneWarn
	toneErr
	toneInfo
)

type testState struct {
	res hosts.Result
	at  time.Time
}

type Model struct {
	mgr  *store.Manager
	cwd  string
	here store.Here

	themes []theme.Theme
	dark   bool
	st     *theme.Styles

	w, h    int
	scr     screen
	sel     int // selected account
	act     int // selected action on the account screen
	confirm bool

	toast string
	tone  tone
	busy  string
	spin  spinner.Model

	issues  int
	infos   map[string]*keys.Info // by account id
	agent   map[string]bool       // fingerprints loaded in ssh-agent
	agentOK bool
	tests   map[string]testState

	add   *addModel
	form  *formModel
	rot   *rotateModel
	chk   *checkModel
	thSel int

	gen int
}

// New builds the app for the folder cwd. dark says whether the terminal
// background is dark, which the Auto theme follows.
func New(mgr *store.Manager, cwd string, dark bool) *Model {
	m := &Model{mgr: mgr, cwd: cwd, dark: dark, infos: map[string]*keys.Info{}, agent: map[string]bool{}, tests: map[string]testState{}}
	var errs []error
	m.themes, errs = theme.All(mgr.P.ThemesDir())
	m.spin = spinner.New(spinner.WithSpinner(spinner.MiniDot))
	m.applyTheme(mgr.Cfg.Theme)
	if len(errs) > 0 {
		m.say(toneWarn, errs[0].Error())
	}
	m.here = mgr.Resolve(cwd)
	for i, a := range mgr.Cfg.Accounts {
		if a.ID == m.here.Account {
			m.sel = i
		}
	}
	return m
}

// Run starts the app in the current terminal.
func Run(mgr *store.Manager) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dark := lipgloss.HasDarkBackground()
	_, err = tea.NewProgram(New(mgr, cwd, dark), tea.WithAltScreen()).Run()
	return err
}

func (m *Model) Init() tea.Cmd { return tea.Batch(m.spin.Tick, m.loadCmd()) }

func (m *Model) nextGen() int {
	m.gen++
	return m.gen
}

func (m *Model) applyTheme(id string) {
	th, paint := theme.Pick(m.themes, id, m.dark)
	m.st = theme.New(th, paint)
	m.spin.Style = m.st.Accent
}

func (m *Model) selected() *config.Account {
	if m.sel >= 0 && m.sel < len(m.mgr.Cfg.Accounts) {
		return &m.mgr.Cfg.Accounts[m.sel]
	}
	return nil
}

func (m *Model) say(t tone, text string) { m.toast, m.tone = text, t }
func (m *Model) fail(err error)          { m.say(toneErr, capitalize(err.Error())) }
func (m *Model) clearToast()             { m.toast = "" }

// refresh rereads what the screens show after a change.
func (m *Model) refresh() tea.Cmd {
	m.here = m.mgr.Resolve(m.cwd)
	if n := len(m.mgr.Cfg.Accounts); m.sel >= n {
		m.sel = n - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
	return m.loadCmd()
}

type loadedMsg struct {
	infos   map[string]*keys.Info
	agent   map[string]bool
	agentOK bool
	issues  int
}

// loadCmd reads keys, the agent and the quick checks in the background.
func (m *Model) loadCmd() tea.Cmd {
	mgr := m.mgr
	accounts := append([]config.Account(nil), mgr.Cfg.Accounts...)
	return func() tea.Msg {
		msg := loadedMsg{infos: map[string]*keys.Info{}, agent: map[string]bool{}}
		for i := range accounts {
			if info, err := mgr.KeyInfo(&accounts[i]); err == nil {
				msg.infos[accounts[i].ID] = info
			}
		}
		if mgr.SSH != nil {
			if fps, ok := mgr.SSH.AgentKeys(); ok {
				msg.agent, msg.agentOK = fps, true
			}
		}
		msg.issues = doctor.Attention(doctor.Local(mgr))
		return msg
	}
}

type toastMsg struct {
	tone tone
	text string
}

type testMsg struct {
	id  string
	res hosts.Result
}

type execDoneMsg struct {
	kind string
	id   string
	gen  int
	err  error
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case loadedMsg:
		m.infos, m.agent, m.agentOK, m.issues = msg.infos, msg.agent, msg.agentOK, msg.issues
		return m, nil
	case toastMsg:
		m.say(msg.tone, msg.text)
		return m, nil
	case testMsg:
		m.busy = ""
		m.tests[msg.id] = testState{res: msg.res, at: time.Now()}
		m.sayTest(msg.id, msg.res)
		return m, nil
	case execDoneMsg:
		return m, m.onExecDone(msg)
	case keyMadeMsg:
		return m, m.onKeyMade(msg)
	case pollMsg:
		return m, m.onPoll(msg)
	case pollResultMsg:
		return m, m.onPollResult(msg)
	case localMsg:
		return m, m.onLocal(msg)
	case connMsg:
		m.onConn(msg)
		return m, nil
	case fixDoneMsg:
		return m, m.onFixDone(msg)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, m.onKey(msg)
	}
	return m, nil
}

func (m *Model) onKey(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()
	switch m.scr {
	case scrHome:
		return m.keyHome(k)
	case scrAccount:
		return m.keyAccount(k)
	case scrAdd:
		return m.keyAdd(msg)
	case scrForm:
		return m.keyForm(msg)
	case scrRotate:
		return m.keyRotate(msg)
	case scrCheck:
		return m.keyCheck(k)
	case scrTheme:
		return m.keyTheme(k)
	case scrHelp:
		if k == "esc" || k == "q" || k == "?" || k == "enter" {
			m.scr = scrHome
		}
	}
	return nil
}

func (m *Model) onExecDone(msg execDoneMsg) tea.Cmd {
	switch msg.kind {
	case "load":
		if msg.err != nil {
			m.say(toneErr, "The key wasn't loaded. ssh-add said: "+msg.err.Error())
		} else {
			m.say(toneOK, "Loaded "+msg.id+"'s key into the agent.")
		}
		return m.refresh()
	case "wizard-load":
		if m.add == nil || m.add.f.gen != msg.gen {
			return nil
		}
		if msg.err != nil {
			m.add.f.log(toneWarn, "The key wasn't loaded into the agent, so the test can't use it yet.")
		} else {
			m.add.f.log(toneOK, "Loaded into the agent")
		}
		return tea.Batch(m.handOff(m.add.f), m.refresh())
	case "fix":
		if msg.err != nil {
			m.say(toneErr, "That didn't work: "+msg.err.Error())
		} else {
			m.say(toneOK, "Done.")
		}
		return tea.Batch(m.startCheck(), m.refresh())
	}
	return nil
}

// testCmd runs a connection test in the background.
func (m *Model) testCmd(a *config.Account) tea.Cmd {
	acc := *a
	mgr := m.mgr
	m.busy = "Connecting to " + mgr.Host(a).Domain + " as " + a.ID + "…"
	return func() tea.Msg {
		return testMsg{id: acc.ID, res: mgr.Test(context.Background(), &acc, mgr.KeyPath(&acc))}
	}
}

func (m *Model) View() string {
	if m.w == 0 {
		return ""
	}
	var body, foot string
	switch m.scr {
	case scrHome:
		body, foot = m.viewHome()
	case scrAccount:
		body, foot = m.viewAccount()
	case scrAdd:
		body, foot = m.viewAdd()
	case scrForm:
		body, foot = m.viewForm()
	case scrRotate:
		body, foot = m.viewRotate()
	case scrCheck:
		body, foot = m.viewCheck()
	case scrTheme:
		body, foot = m.viewTheme()
	case scrHelp:
		body, foot = m.viewHelp()
	}
	return m.frame(body, foot)
}
