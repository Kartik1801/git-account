package tui

import (
	"context"
	"crypto"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/hosts"
	"github.com/kartik-dhawan-dev/git-account/internal/keys"
	"github.com/kartik-dhawan-dev/git-account/internal/sshx"
	"github.com/kartik-dhawan-dev/git-account/internal/sysx"
)

// flow is the "connect a key" sequence shared by adding an account and
// replacing a key: create the key, copy it, open the host's page, then
// test every few seconds until the host accepts it.
type flow struct {
	who       string // add or rotate
	gen       int
	target    config.Account // what to test against
	host      hosts.Host
	keyPath   string
	info      *keys.Info
	created   bool // we made the key, so going back deletes it
	lines     []logLine
	waiting   bool
	connected bool
	failed    bool
	gotUser   string
	tries     int
	note      string
}

type logLine struct {
	tone tone
	text string
}

func (f *flow) log(t tone, text string) { f.lines = append(f.lines, logLine{t, text}) }

// stop cancels polling; results from older generations are ignored.
func (f *flow) stop(m *Model) {
	f.gen = m.nextGen()
	f.waiting = false
}

func (f *flow) discardKey() {
	if f.created && f.keyPath != "" {
		keys.Delete(f.keyPath)
	}
	f.created = false
}

type keyMadeMsg struct {
	who, path string
	gen       int
	info      *keys.Info
	err       error
	agentErr  error
	encrypted bool
}

type pollMsg struct {
	who string
	gen int
}

type pollResultMsg struct {
	who string
	gen int
	res hosts.Result
}

func (m *Model) flowFor(who string) *flow {
	switch {
	case who == "add" && m.add != nil:
		return m.add.f
	case who == "rotate" && m.rot != nil:
		return m.rot.f
	}
	return nil
}

// makeKey generates a key in the background. A passphrase-protected key
// is handed straight to ssh-agent so the test can use it.
func (m *Model) makeKey(f *flow, path, typ, comment string, pass []byte) tea.Cmd {
	f.gen = m.nextGen()
	who, gen := f.who, f.gen
	m.busy = "Creating the key…"
	return func() tea.Msg {
		info, priv, err := keys.Generate(path, typ, comment, pass)
		msg := keyMadeMsg{who: who, gen: gen, path: path, info: info, err: err, encrypted: len(pass) > 0}
		if err == nil && len(pass) > 0 {
			msg.agentErr = addToAgent(priv, comment)
		}
		return msg
	}
}

func addToAgent(priv crypto.PrivateKey, comment string) error { return sshx.AddToAgent(priv, comment) }

func (m *Model) onKeyMade(msg keyMadeMsg) tea.Cmd {
	f := m.flowFor(msg.who)
	if f == nil || f.gen != msg.gen {
		if msg.err == nil {
			keys.Delete(msg.path)
		}
		return nil
	}
	m.busy = ""
	if msg.err != nil {
		f.failed = true
		f.log(toneErr, "Couldn't create the key: "+msg.err.Error())
		return nil
	}
	f.info, f.keyPath, f.created = msg.info, msg.path, true
	f.log(toneOK, "Key created at "+m.pretty(msg.path))
	if msg.encrypted {
		if msg.agentErr != nil {
			f.log(toneWarn, "Couldn't load it into the agent ("+msg.agentErr.Error()+"). Pushes will ask for the passphrase.")
		} else {
			f.log(toneOK, "Loaded into the agent, so you won't retype the passphrase")
		}
	}
	return m.handOff(f)
}

// handOff copies the key, opens the host's page and starts waiting.
func (m *Model) handOff(f *flow) tea.Cmd {
	if err := sysx.Copy(f.info.Line()); err != nil {
		f.log(toneWarn, "Couldn't reach the clipboard. Copy the key from the box below.")
	} else {
		f.log(toneOK, "Public key copied to the clipboard")
	}
	url := f.host.KeysURL()
	if err := sysx.OpenURL(url); err != nil {
		f.log(toneWarn, "Couldn't open a browser. Go to "+url)
	} else {
		f.log(toneOK, "Opened "+strings.TrimPrefix(url, "https://"))
	}
	f.waiting, f.failed, f.tries, f.note = true, false, 0, ""
	f.gen = m.nextGen()
	return pollAfter(f, 2*time.Second)
}

func pollAfter(f *flow, d time.Duration) tea.Cmd {
	who, gen := f.who, f.gen
	return tea.Tick(d, func(time.Time) tea.Msg { return pollMsg{who: who, gen: gen} })
}

func (m *Model) onPoll(msg pollMsg) tea.Cmd {
	f := m.flowFor(msg.who)
	if f == nil || f.gen != msg.gen || !f.waiting {
		return nil
	}
	f.tries++
	mgr, target, key, who, gen := m.mgr, f.target, f.keyPath, f.who, f.gen
	return func() tea.Msg {
		return pollResultMsg{who: who, gen: gen, res: mgr.Test(context.Background(), &target, key)}
	}
}

func (m *Model) onPollResult(msg pollResultMsg) tea.Cmd {
	f := m.flowFor(msg.who)
	if f == nil || f.gen != msg.gen || !f.waiting {
		return nil
	}
	r := msg.res
	switch {
	case r.OK:
		f.waiting, f.connected, f.gotUser = false, true, r.Username
		who := "accepted the key"
		if r.Username != "" {
			who = "says Hi @" + r.Username
		}
		f.log(toneOK, "Connected: "+f.host.Domain+" "+who)
		if msg.who == "add" {
			m.addConnected()
			return nil
		}
		return m.rotateConnected()
	case f.tries > 200:
		f.waiting, f.failed = false, true
		f.log(toneWarn, "Stopped waiting after 10 minutes.")
		return nil
	case r.Kind == "denied":
		f.note = ""
		return pollAfter(f, 3*time.Second)
	case r.Kind == "network":
		f.note = capitalize(r.Message) + " right now; still trying."
		return pollAfter(f, 5*time.Second)
	default:
		f.waiting, f.failed = false, true
		f.log(toneErr, capitalize(r.Message))
		return nil
	}
}

// viewFlow shows what happened so far, the key and the waiting line.
func (m *Model) viewFlow(f *flow) []string {
	s := m.st
	var b []string
	for _, l := range f.lines {
		sym := s.OK.Render("✓")
		switch l.tone {
		case toneWarn:
			sym = s.Warn.Render("!")
		case toneErr:
			sym = s.Err.Render("✗")
		case toneInfo:
			sym = s.Info.Render("→")
		}
		b = append(b, "  "+sym+" "+s.FG.Render(l.text))
	}
	if f.info != nil {
		b = append(b, "", s.Box.Width(m.cw()-4).Render(s.Dim.Render(f.info.Line())))
	}
	if f.waiting {
		b = append(b, "", "  "+m.spin.View()+" "+s.FG.Render("Paste the key on that page. Waiting for "+f.host.Name+"…"))
		if f.note != "" {
			b = append(b, "    "+s.Dim.Render(f.note))
		}
	}
	return b
}

// keyFlow handles the shortcuts while waiting for the host.
func (m *Model) keyFlow(f *flow, k string) (tea.Cmd, bool) {
	if f.info == nil {
		return nil, false
	}
	switch k {
	case "c":
		if err := sysx.Copy(f.info.Line()); err != nil {
			m.say(toneWarn, "Couldn't reach the clipboard. Copy the key from the box.")
		} else {
			m.say(toneOK, "Copied the public key again.")
		}
		return nil, true
	case "o":
		if err := sysx.OpenURL(f.host.KeysURL()); err != nil {
			m.say(toneWarn, "Couldn't open a browser. Go to "+f.host.KeysURL())
		} else {
			m.say(toneInfo, "Opened "+strings.TrimPrefix(f.host.KeysURL(), "https://"))
		}
		return nil, true
	case "r":
		if f.failed {
			f.failed, f.waiting, f.tries = false, true, 0
			f.gen = m.nextGen()
			return pollAfter(f, 0), true
		}
	}
	return nil, false
}
