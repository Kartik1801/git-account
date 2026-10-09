package hosts

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		out, kind, user string
	}{
		{"Hi kartik-acme! You've successfully authenticated, but GitHub does not provide shell access.", "ok", "kartik-acme"},
		{"Welcome to GitLab, @kartik.cx!", "ok", "kartik.cx"},
		{"authenticated via ssh key.\n\nYou can use git to connect to Bitbucket. Shell access is disabled\n", "ok", ""},
		{"logged in as kdhawan.\n\nYou can use git or hg to connect to Bitbucket.", "ok", "kdhawan"},
		{"Hi there, forge-user! You've successfully authenticated with the key named x, but Forgejo does not provide shell access.", "ok", "forge-user"},
		{"git@github.com: Permission denied (publickey).", "denied", ""},
		{"ssh: connect to host github.com port 22: Connection timed out", "network", ""},
		{"ssh: Could not resolve hostname gitlab.acme: Name or service not known", "network", ""},
		{"Host key verification failed.", "hostkey", ""},
	}
	for _, c := range cases {
		r := Parse(c.out)
		if r.Kind != c.kind || r.Username != c.user {
			t.Errorf("Parse(%q) = kind %q user %q, want %q %q", c.out, r.Kind, r.Username, c.kind, c.user)
		}
	}
}

func TestFor(t *testing.T) {
	if h := For("github", ""); h.KeysURL() != "https://github.com/settings/ssh/new" || h.Alt443 != "ssh.github.com" {
		t.Errorf("github: %+v", h)
	}
	if h := For("github", "github.acme.com"); h.Domain != "github.acme.com" || h.Alt443 != "" {
		t.Errorf("enterprise: %+v", h)
	}
	if h := For("custom", "gitlab.acme.io"); h.KeysPath != "/-/user_settings/ssh_keys" {
		t.Errorf("custom gitlab: %+v", h)
	}
}
