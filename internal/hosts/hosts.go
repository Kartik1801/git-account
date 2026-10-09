// Package hosts knows the Git hosts git-account supports: where their SSH
// key settings live, which server answers on port 443, and how to read the
// "Hi @user" reply to tell which account a key belongs to.
package hosts

import (
	"regexp"
	"strings"
)

type Host struct {
	Kind     string // github, gitlab, bitbucket, custom
	Name     string // GitHub
	Domain   string // github.com
	KeysPath string // path of the "add SSH key" page
	Alt443   string // server that speaks SSH on port 443, or ""
}

// KeysURL is the page where you add an SSH key.
func (h Host) KeysURL() string { return "https://" + h.Domain + h.KeysPath }

var known = []Host{
	{Kind: "github", Name: "GitHub", Domain: "github.com", KeysPath: "/settings/ssh/new", Alt443: "ssh.github.com"},
	{Kind: "gitlab", Name: "GitLab", Domain: "gitlab.com", KeysPath: "/-/user_settings/ssh_keys", Alt443: "altssh.gitlab.com"},
	{Kind: "bitbucket", Name: "Bitbucket", Domain: "bitbucket.org", KeysPath: "/account/settings/ssh-keys/", Alt443: "altssh.bitbucket.org"},
}

// Known returns the built-in hosts.
func Known() []Host { return append([]Host(nil), known...) }

// For returns the host for an account. A hostname turns a built-in kind
// into its self-hosted version (GitHub Enterprise, self-managed GitLab).
func For(kind, hostname string) Host {
	for _, h := range known {
		if h.Kind != kind {
			continue
		}
		if hostname == "" || strings.EqualFold(hostname, h.Domain) {
			return h
		}
		h.Domain, h.Alt443 = hostname, ""
		if kind == "bitbucket" {
			h.KeysPath = "/plugins/servlet/ssh/account/keys"
		}
		return h
	}
	h := Host{Kind: "custom", Name: hostname, Domain: hostname, KeysPath: "/"}
	switch l := strings.ToLower(hostname); {
	case strings.Contains(l, "gitlab"):
		h.KeysPath = "/-/user_settings/ssh_keys"
	case strings.Contains(l, "github"):
		h.KeysPath = "/settings/ssh/new"
	case strings.Contains(l, "gitea"), strings.Contains(l, "forgejo"), strings.Contains(l, "codeberg"):
		h.KeysPath = "/user/settings/keys"
	}
	return h
}

// Alt443 returns the port-443 server for a domain, or "".
func Alt443(domain string) string {
	for _, h := range known {
		if strings.EqualFold(h.Domain, domain) {
			return h.Alt443
		}
	}
	return ""
}

// Result is what a connection test found.
type Result struct {
	OK       bool
	Username string // who the host says you are, when it says
	Kind     string // ok, denied, network, hostkey, passphrase, error
	Message  string // one plain sentence
	Raw      string
}

var userPatterns = []*regexp.Regexp{
	regexp.MustCompile(`Hi there, ([^!\s]+)!`),           // Gitea, Forgejo
	regexp.MustCompile(`Hi ([^!\s]+)!`),                  // GitHub
	regexp.MustCompile(`Welcome to GitLab, @([^!\s]+)!`), // GitLab
	regexp.MustCompile(`logged in as ([A-Za-z0-9._-]+)`), // Bitbucket
	regexp.MustCompile(`(?i)authenticated as ([A-Za-z0-9._-]+)`),
}

var okPhrases = []string{"successfully authenticated", "Welcome to GitLab", "authenticated via ssh key", "You can use git to connect"}

var networkPhrases = []string{
	"Connection timed out", "Operation timed out", "Connection refused", "Network is unreachable",
	"No route to host", "Connection closed by", "Connection reset", "kex_exchange_identification",
	"connect to host", "Could not resolve hostname", "Name or service not known", "nodename nor servname",
}

// Parse reads the output of `ssh -T git@host`.
func Parse(out string) Result {
	r := Result{Raw: out}
	for _, re := range userPatterns {
		if m := re.FindStringSubmatch(out); m != nil {
			r.OK, r.Kind, r.Username = true, "ok", strings.TrimRight(strings.TrimPrefix(m[1], "@"), ".")
			return r
		}
	}
	for _, p := range okPhrases {
		if strings.Contains(out, p) {
			r.OK, r.Kind = true, "ok"
			return r
		}
	}
	switch {
	case strings.Contains(out, "Permission denied"):
		r.Kind, r.Message = "denied", "the host didn't accept this key"
	case strings.Contains(out, "Host key verification failed"), strings.Contains(out, "REMOTE HOST IDENTIFICATION HAS CHANGED"):
		r.Kind, r.Message = "hostkey", "the server's identity changed or couldn't be checked"
	case strings.Contains(out, "passphrase"):
		r.Kind, r.Message = "passphrase", "the key needs its passphrase"
	case containsAny(out, networkPhrases):
		r.Kind, r.Message = "network", "can't reach the server"
		if strings.Contains(out, "resolve hostname") || strings.Contains(out, "not known") || strings.Contains(out, "servname") {
			r.Message = "can't find the server; check the host name and your connection"
		}
	default:
		r.Kind = "error"
		r.Message = firstLine(out)
		if r.Message == "" {
			r.Message = "ssh exited without saying why"
		}
	}
	return r
}

func containsAny(s string, list []string) bool {
	for _, p := range list {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}
