// Package config reads and writes config.toml, the single source of truth
// for accounts, automatic folders, the default account and the theme.
// Every other file git-account writes is generated from it.
package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
)

// Version is the config.toml format this build writes.
const Version = 1

type Config struct {
	Version  int       `toml:"version"`
	Default  string    `toml:"default,omitempty"`
	Theme    string    `toml:"theme,omitempty"`
	SSH443   []string  `toml:"ssh_over_443,omitempty"`
	Accounts []Account `toml:"account"`
}

type Account struct {
	ID         string    `toml:"id"`                 // short name, like work
	UID        string    `toml:"uid"`                // stable id; survives renames
	Name       string    `toml:"name"`               // commit author name
	Email      string    `toml:"email"`              // commit author email
	Host       string    `toml:"host"`               // github, gitlab, bitbucket or custom
	Hostname   string    `toml:"hostname,omitempty"` // for self-hosted servers
	Port       int       `toml:"port,omitempty"`
	SSHUser    string    `toml:"ssh_user,omitempty"` // defaults to git
	Username   string    `toml:"username,omitempty"` // account name on the host
	Key        string    `toml:"key"`                // private key path, ~ allowed
	Folders    []string  `toml:"folders,omitempty"`
	Created    time.Time `toml:"created"`
	KeyCreated time.Time `toml:"key_created"`
}

// HostKinds are the values Account.Host accepts.
var HostKinds = []string{"github", "gitlab", "bitbucket", "custom"}

var (
	idRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	uidRe   = regexp.MustCompile(`^[a-f0-9]{8,32}$`)
	emailRe = regexp.MustCompile(`^[^@\s"\\]+@[^@\s"\\]+\.[^@\s"\\]+$`)
	userRe  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	hostRe  = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
)

func ValidateID(id string) error {
	if !idRe.MatchString(id) {
		return errors.New("short name: use lowercase letters, numbers and dashes, like client-x")
	}
	return nil
}

func ValidateEmail(email string) error {
	if !emailRe.MatchString(email) {
		return fmt.Errorf("%q doesn't look like an email address", email)
	}
	return nil
}

func ValidateName(name string) error {
	switch {
	case strings.TrimSpace(name) == "":
		return errors.New("add a name")
	case strings.ContainsAny(name, "\r\n"):
		return errors.New("the name can't contain line breaks")
	case len(name) > 200:
		return errors.New("the name is too long")
	}
	return nil
}

func ValidateUsername(u string) error {
	if !userRe.MatchString(u) {
		return fmt.Errorf("username %q: use letters, numbers, dots, dashes and underscores", u)
	}
	return nil
}

func ValidateHostname(h string) error {
	if !hostRe.MatchString(h) {
		return fmt.Errorf("%q isn't a host name, like git.example.com", h)
	}
	return nil
}

func ValidateFolder(f string) error {
	if strings.TrimSpace(f) == "" {
		return errors.New("the folder is empty")
	}
	if strings.ContainsAny(f, "*?[\r\n\"") {
		return fmt.Errorf("folder %q can't contain * ? [ or quotes", f)
	}
	return nil
}

func (a *Account) Validate() error {
	if err := ValidateID(a.ID); err != nil {
		return fmt.Errorf("%q: %w", a.ID, err)
	}
	wrap := func(err error) error { return fmt.Errorf("%s: %w", a.ID, err) }
	if !uidRe.MatchString(a.UID) {
		return wrap(errors.New("missing or invalid uid"))
	}
	if err := ValidateName(a.Name); err != nil {
		return wrap(err)
	}
	if err := ValidateEmail(a.Email); err != nil {
		return wrap(err)
	}
	if !contains(HostKinds, a.Host) {
		return wrap(fmt.Errorf("host must be one of %s", strings.Join(HostKinds, ", ")))
	}
	if a.Host == "custom" && a.Hostname == "" {
		return wrap(errors.New("a custom host needs a hostname"))
	}
	if a.Hostname != "" {
		if err := ValidateHostname(a.Hostname); err != nil {
			return wrap(err)
		}
	}
	if a.Port < 0 || a.Port > 65535 {
		return wrap(fmt.Errorf("port %d is out of range", a.Port))
	}
	if a.SSHUser != "" {
		if err := ValidateUsername(a.SSHUser); err != nil {
			return wrap(err)
		}
	}
	if a.Username != "" {
		if err := ValidateUsername(a.Username); err != nil {
			return wrap(err)
		}
	}
	if strings.TrimSpace(a.Key) == "" || strings.ContainsAny(a.Key, "\r\n") {
		return wrap(errors.New("no SSH key set"))
	}
	for _, f := range a.Folders {
		if err := ValidateFolder(f); err != nil {
			return wrap(err)
		}
	}
	return nil
}

func (c *Config) Validate() error {
	ids := map[string]bool{}
	uids := map[string]bool{}
	folders := map[string]string{}
	for i := range c.Accounts {
		a := &c.Accounts[i]
		if err := a.Validate(); err != nil {
			return err
		}
		if ids[a.ID] {
			return fmt.Errorf("two accounts are called %s", a.ID)
		}
		ids[a.ID] = true
		if uids[a.UID] {
			return fmt.Errorf("two accounts share the uid %s", a.UID)
		}
		uids[a.UID] = true
		for _, f := range a.Folders {
			k := strings.ToLower(strings.TrimSuffix(f, "/"))
			if other, ok := folders[k]; ok {
				return fmt.Errorf("%s is set for both %s and %s", f, other, a.ID)
			}
			folders[k] = a.ID
		}
	}
	if c.Default != "" && !ids[c.Default] {
		return fmt.Errorf("the default account %s doesn't exist", c.Default)
	}
	for _, d := range c.SSH443 {
		if err := ValidateHostname(d); err != nil {
			return err
		}
	}
	return nil
}

// Find returns the account called id and its index, or nil and -1.
func (c *Config) Find(id string) (*Account, int) {
	for i := range c.Accounts {
		if c.Accounts[i].ID == id {
			return &c.Accounts[i], i
		}
	}
	return nil, -1
}

// FindUID returns the account with this uid, or nil.
func (c *Config) FindUID(uid string) *Account {
	for i := range c.Accounts {
		if c.Accounts[i].UID == uid {
			return &c.Accounts[i]
		}
	}
	return nil
}

// Clone returns a deep copy.
func (c *Config) Clone() *Config {
	n := *c
	n.SSH443 = append([]string(nil), c.SSH443...)
	n.Accounts = make([]Account, len(c.Accounts))
	for i, a := range c.Accounts {
		a.Folders = append([]string(nil), a.Folders...)
		n.Accounts[i] = a
	}
	return &n
}

// Load reads config.toml. A missing file is an empty config.
func Load(path string) (*Config, error) {
	data, ok, err := fsx.ReadIfExists(path)
	if err != nil {
		return nil, err
	}
	c := &Config{Version: Version}
	if !ok {
		return c, nil
	}
	if _, err := toml.Decode(string(data), c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if c.Version > Version {
		return nil, fmt.Errorf("%s was written by a newer git-account (format %d); update git-account", path, c.Version)
	}
	if c.Version == 0 {
		c.Version = Version
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// Encode renders the config as TOML.
func (c *Config) Encode() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("# git-account settings. You can edit this file; run `git-account sync` afterwards.\n\n")
	enc := toml.NewEncoder(&b)
	enc.Indent = ""
	if err := enc.Encode(c); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// NewUID returns a random stable id for a new account.
func NewUID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
