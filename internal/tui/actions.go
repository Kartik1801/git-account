package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/hosts"
	"github.com/kartik-dhawan-dev/git-account/internal/store"
	"github.com/kartik-dhawan-dev/git-account/internal/sysx"
)

func (m *Model) useHere(a *config.Account) tea.Cmd {
	removed, err := m.mgr.UseHere(m.cwd, a.ID)
	if errors.Is(err, store.ErrNotRepo) {
		m.say(toneWarn, "This folder isn't a git repo. Press d to make "+a.ID+" the default instead.")
		return nil
	}
	if err != nil {
		m.fail(err)
		return nil
	}
	msg := repoName(m.here.Repo) + " now uses " + a.ID + ". Only this repo changed."
	if len(removed) > 0 {
		msg += " Removed the repo's own " + strings.Join(removed, ", ") + "."
	}
	m.say(toneOK, msg)
	return m.refresh()
}

func (m *Model) makeDefault(a *config.Account) tea.Cmd {
	if err := m.mgr.SetDefault(a.ID); err != nil {
		m.fail(err)
		return nil
	}
	m.say(toneOK, a.ID+" is now the default everywhere no folder or repo says otherwise.")
	return m.refresh()
}

func (m *Model) copyKey(a *config.Account) tea.Cmd {
	info := m.infos[a.ID]
	if info == nil {
		var err error
		if info, err = m.mgr.KeyInfo(a); err != nil {
			m.fail(err)
			return nil
		}
	}
	line, id := info.Line(), a.ID
	return func() tea.Msg {
		if err := sysx.Copy(line); err != nil {
			return toastMsg{toneErr, "Couldn't reach the clipboard (" + err.Error() + "). Run `git-account show-key " + id + "` to print the key."}
		}
		return toastMsg{toneOK, "Copied " + id + "'s public key: " + shortKey(line)}
	}
}

func (m *Model) openPage(a *config.Account) tea.Cmd {
	h := m.mgr.Host(a)
	if err := sysx.OpenURL(h.KeysURL()); err != nil {
		m.say(toneWarn, "Couldn't open a browser. The page is "+h.KeysURL())
		return nil
	}
	msg := "Opened " + strings.TrimPrefix(h.KeysURL(), "https://") + "."
	if info := m.infos[a.ID]; info != nil {
		msg += " Look for the fingerprint " + shortFP(info.Fingerprint)
	}
	m.say(toneInfo, msg)
	return nil
}

func (m *Model) loadKey(a *config.Account) tea.Cmd {
	if m.mgr.SSH == nil {
		m.say(toneErr, "OpenSSH isn't installed, so there's no agent to load the key into.")
		return nil
	}
	id := a.ID
	return tea.ExecProcess(m.mgr.SSH.LoadCmd(m.mgr.KeyPath(a)), func(err error) tea.Msg {
		return execDoneMsg{kind: "load", id: id, err: err}
	})
}

func (m *Model) removeSelected() tea.Cmd {
	a := m.selected()
	if a == nil {
		return nil
	}
	id := a.ID
	if err := m.mgr.Remove(id); err != nil {
		m.fail(err)
		return nil
	}
	m.scr = scrHome
	m.say(toneOK, "Removed "+id+". Its key is in the trash for 30 days. Press u to undo.")
	return m.refresh()
}

func (m *Model) undo() tea.Cmd {
	label, err := m.mgr.Undo()
	if err != nil {
		m.say(toneDim, capitalize(err.Error())+".")
		return nil
	}
	m.say(toneOK, "Undid: "+label)
	return m.refresh()
}

func (m *Model) sayTest(id string, r hosts.Result) {
	a, _ := m.mgr.Cfg.Find(id)
	if a == nil {
		return
	}
	h := m.mgr.Host(a)
	switch {
	case r.OK && a.Username != "" && r.Username != "" && !strings.EqualFold(a.Username, r.Username):
		m.say(toneErr, h.Domain+" says this key belongs to @"+r.Username+", not @"+a.Username+". Press c for the fix.")
	case r.OK && r.Username != "":
		m.say(toneOK, h.Domain+" says Hi @"+r.Username+"!")
	case r.OK:
		m.say(toneOK, h.Domain+" accepted the key.")
	case r.Kind == "denied":
		m.say(toneErr, h.Domain+" didn't accept the key. Press y to copy it and o to open the SSH keys page.")
	case r.Kind == "passphrase":
		m.say(toneWarn, "The key has a passphrase and isn't loaded in the agent. Press → then s to load it.")
	case r.Kind == "network":
		m.say(toneErr, capitalize(r.Message)+". Press c to check whether port 443 works instead.")
	default:
		m.say(toneErr, capitalize(r.Message))
	}
}
