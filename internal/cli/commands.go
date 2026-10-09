package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kartik-dhawan-dev/git-account/internal/config"
	"github.com/kartik-dhawan-dev/git-account/internal/doctor"
	"github.com/kartik-dhawan-dev/git-account/internal/hosts"
	"github.com/kartik-dhawan-dev/git-account/internal/keys"
	"github.com/kartik-dhawan-dev/git-account/internal/store"
	"github.com/kartik-dhawan-dev/git-account/internal/sysx"
	"github.com/kartik-dhawan-dev/git-account/internal/theme"
)

type accountJSON struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Host     string   `json:"host"`
	Domain   string   `json:"domain"`
	Username string   `json:"username,omitempty"`
	Key      string   `json:"key"`
	Folders  []string `json:"folders"`
	Default  bool     `json:"default"`
}

func listCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "list", Aliases: []string{"ls"}, Short: "List your accounts", Args: cobra.NoArgs,
		Long: "List your accounts. A dot marks the one that applies in the current folder.",
		Example: `  git-account list
  git-account list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			if asJSON {
				out := []accountJSON{}
				for i := range m.Cfg.Accounts {
					a := &m.Cfg.Accounts[i]
					out = append(out, accountJSON{a.ID, a.Name, a.Email, a.Host, m.Host(a).Domain, a.Username, m.KeyPath(a),
						append([]string{}, a.Folders...), a.ID == m.Cfg.Default})
				}
				return printJSON(out)
			}
			if len(m.Cfg.Accounts) == 0 {
				info("No accounts yet. Add one with `git-account add <name>`, or run `git-account` and press a.")
				return nil
			}
			here := m.Resolve(cwd())
			idW, emailW := 0, 0
			for _, a := range m.Cfg.Accounts {
				idW, emailW = max(idW, len(a.ID)), max(emailW, len(a.Email))
			}
			for i := range m.Cfg.Accounts {
				a := &m.Cfg.Accounts[i]
				mark := "  "
				if a.ID == here.Account {
					mark = okSt.Render("● ")
				}
				tags := []string{m.Host(a).Name}
				if a.ID == m.Cfg.Default {
					tags = append(tags, warnSt.Render("default"))
				}
				for _, f := range a.Folders {
					tags = append(tags, "auto "+f)
				}
				fmt.Printf("%s%s  %-*s  %s\n", mark, boldSt.Render(fmt.Sprintf("%-*s", idW, a.ID)), emailW, a.Email, dimSt.Render(strings.Join(tags, " · ")))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func status(m *store.Manager, asJSON bool) error {
	h := m.Resolve(cwd())
	if asJSON {
		return printJSON(map[string]string{"repo": h.Repo, "account": h.Account, "kind": h.Kind, "folder": h.Folder, "file": h.File, "email": h.Email})
	}
	var why string
	switch h.Kind {
	case "repo":
		why = "set for this repo"
	case "folder":
		why = "automatic in " + h.Folder
	case "default":
		why = "your default"
		if h.Repo == "" {
			why += " (not in a git repo)"
		}
	case "other":
		fmt.Printf("%s comes from %s, which git-account doesn't manage\n", h.Email, m.P.Pretty(h.File))
		return nil
	default:
		info("No account applies here. Add one with `git-account add <name>`.")
		return nil
	}
	a, _ := m.Cfg.Find(h.Account)
	fmt.Printf("%s  %s\n", boldSt.Render(h.Account), dimSt.Render(why))
	if a != nil {
		fmt.Printf("  %s <%s> · %s\n", a.Name, a.Email, m.Host(a).Domain)
	}
	return nil
}

func statusCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "status", Aliases: []string{"current", "whoami"}, Short: "Show which account applies here and why", Args: cobra.NoArgs,
		Long: `Show which account git uses in the current folder and why: pinned to this
repo, automatic because of a folder, or your default.`,
		Example: `  git-account status
  git-account status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			return status(m, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func useCmd() *cobra.Command {
	var global bool
	cmd := &cobra.Command{
		Use: "use <account>", Aliases: []string{"switch"}, Short: "Use an account in this repo, or everywhere with --global",
		Long: `Pin an account to the repo you're in. Only this repo changes. If the repo
set its own user.name, user.email or core.sshCommand, those are removed,
because they would win over the account.

With --global the account becomes your default instead: it's used wherever
no folder or pinned repo says otherwise.`,
		Example: `  git-account use work
  git-account use personal --global`,
		Args: needArgs(1, 1, true, "git-account use work"), ValidArgsFunction: accountArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			if global {
				if err := m.SetDefault(args[0]); err != nil {
					return err
				}
				ok("%s is now the default everywhere no folder or repo says otherwise", args[0])
				return nil
			}
			removed, err := m.UseHere(cwd(), args[0])
			if errors.Is(err, store.ErrNotRepo) {
				return errors.New("this folder isn't a git repo; add --global to make it your default")
			}
			if err != nil {
				return err
			}
			ok("This repo now uses %s", args[0])
			if len(removed) > 0 {
				info("  Removed the repo's own %s", strings.Join(removed, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&global, "global", "g", false, "make it the default account instead")
	return cmd
}

func unpinCmd() *cobra.Command {
	return &cobra.Command{
		Use: "unpin", Short: "Stop pinning an account to this repo", Args: cobra.NoArgs,
		Long:    "Remove the account pinned to this repo with `git-account use`. The repo then follows your folders and your default again.",
		Example: "  git-account unpin",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			if err := m.Unpin(cwd()); err != nil {
				return err
			}
			ok("This repo follows folders and your default again")
			return nil
		},
	}
}

func defaultCmd() *cobra.Command {
	return &cobra.Command{
		Use: "default <account>", Short: "Make an account the default", ValidArgsFunction: accountArgs,
		Long:    "Make an account your default. It's the same as `git-account use <account> --global`.",
		Example: "  git-account default personal",
		Args:    needArgs(1, 1, true, "git-account default personal"),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			if err := m.SetDefault(args[0]); err != nil {
				return err
			}
			ok("%s is now the default", args[0])
			return nil
		},
	}
}

func globalGit(m *store.Manager, key string) string {
	v, _, _ := m.Git.Get(os.TempDir(), "--global", "--get", key)
	return v
}

func addCmd() *cobra.Command {
	var a config.Account
	var keyPath, newKey string
	var passStdin, noVerify, noBrowser bool
	var wait time.Duration
	cmd := &cobra.Command{
		Use:   "add <account>",
		Short: "Add an account",
		Long: `Add an account from the command line. <account> is a short name, like work.

Without --key a new ed25519 key is created. Its public key is printed and
copied, the host's SSH key page opens, and the command waits until the host
accepts the key. If you gave --username, it also checks the key signs in as
that user, which catches keys added to the wrong account.

In a terminal, anything you leave out is asked for. In scripts, pass
--name and --email (and usually --username).

For the guided version, run "git-account" and press a.`,
		Example: `  # Asks for the rest
  git-account add work

  # Everything as flags
  git-account add work --name "Kartik Dhawan" --email kartik@acme.com \
    --username kartik-acme --folder ~/work

  # GitLab, or a self-hosted server
  git-account add oss --host gitlab --email me@example.com --username me
  git-account add corp --host custom --hostname git.corp.example --port 2222 --email me@corp.example

  # A key you already have, and no waiting for the host
  git-account add old --key ~/.ssh/id_ed25519 --email me@example.com --no-verify`,
		Args: needArgs(1, 1, false, "git-account add work"),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			a.ID = args[0]
			a.UID = config.NewUID()
			if err := config.ValidateID(a.ID); err != nil {
				return err
			}
			if x, _ := m.Cfg.Find(a.ID); x != nil {
				return fmt.Errorf("you already have an account called %s", a.ID)
			}
			h := hosts.For(a.Host, a.Hostname)
			if interactive() {
				if a.Name == "" {
					a.Name = ask("Your name for commits:", globalGit(m, "user.name"))
				}
				if a.Email == "" {
					a.Email = ask("Email for commits:", globalGit(m, "user.email"))
				}
				if a.Username == "" && !cmd.Flags().Changed("username") {
					a.Username = strings.TrimPrefix(ask("Your username on "+h.Name+" (Enter to skip):", ""), "@")
				}
			}
			var missing []string
			if a.Name == "" {
				missing = append(missing, "--name")
			}
			if a.Email == "" {
				missing = append(missing, "--email")
			}
			if len(missing) > 0 {
				return fmt.Errorf("missing %s\n  for example: git-account add %s --name \"Your Name\" --email you@example.com --username you",
					strings.Join(missing, " and "), a.ID)
			}
			a.Key = "pending" // validated for real once the key exists
			if err := a.Validate(); err != nil {
				return err
			}

			created := false
			if keyPath != "" {
				a.Key = m.P.Expand(keyPath)
				if _, err := keys.Load(a.Key); err != nil {
					return err
				}
			} else {
				var pass []byte
				switch {
				case passStdin:
					pass = []byte(readLine())
				case interactive():
					p1 := askSecret("Passphrase for the new key (Enter for none):")
					if p1 != "" && askSecret("Type it again:") != p1 {
						return errors.New("the passphrases don't match")
					}
					pass = []byte(p1)
				}
				a.Key = m.NewKeyPath(a.ID, newKey)
				if _, _, err := keys.Generate(a.Key, newKey, a.Email, pass); err != nil {
					return err
				}
				created = true
				ok("Created %s", m.P.Pretty(a.Key))
			}
			if !noVerify {
				if err := handOffAndWait(m, &a, a.Key, !noBrowser, wait); err != nil {
					if created {
						keys.Delete(a.Key)
					}
					return err
				}
			}
			if err := m.Add(a); err != nil {
				if created {
					keys.Delete(a.Key)
				}
				return err
			}
			ok("Added %s", a.ID)
			if len(a.Folders) > 0 {
				info("  Repos in %s use it automatically.", strings.Join(a.Folders, ", "))
			} else if len(m.Cfg.Accounts) > 1 {
				info("  Use it in a repo with `git-account use %s`, or for a folder with `git-account folder add %s <folder>`.", a.ID, a.ID)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&a.Name, "name", "", "your name for commits")
	f.StringVar(&a.Email, "email", "", "your email for commits")
	f.StringVar(&a.Host, "host", "github", "github, gitlab, bitbucket or custom")
	f.StringVar(&a.Hostname, "hostname", "", "server name, for GitHub Enterprise, self-hosted GitLab or custom")
	f.IntVar(&a.Port, "port", 0, "SSH port, if it isn't 22")
	f.StringVar(&a.Username, "username", "", "your username on the host, used to check the key")
	f.StringSliceVar(&a.Folders, "folder", nil, "use the account automatically in this folder (repeatable)")
	f.StringVar(&keyPath, "key", "", "use this existing private key instead of creating one")
	f.StringVar(&newKey, "key-type", "ed25519", "type of the new key: ed25519 or rsa")
	f.BoolVar(&passStdin, "passphrase-stdin", false, "read the new key's passphrase from standard input")
	f.BoolVar(&noVerify, "no-verify", false, "save without waiting for the host to accept the key")
	f.BoolVar(&noBrowser, "no-browser", false, "don't open the host's SSH key page")
	f.DurationVar(&wait, "wait", 10*time.Minute, "how long to wait for the host to accept the key")
	return cmd
}

// handOffAndWait prints and copies the public key, opens the host's page
// and polls until the host accepts the key as the right user.
func handOffAndWait(m *store.Manager, a *config.Account, key string, browser bool, wait time.Duration) error {
	k, err := keys.Load(key)
	if err != nil {
		return err
	}
	h := m.Host(a)
	fmt.Println()
	fmt.Println(k.Line())
	fmt.Println()
	if err := sysx.Copy(k.Line()); err == nil {
		ok("Public key copied to the clipboard")
	}
	opened := false
	if browser {
		opened = sysx.OpenURL(h.KeysURL()) == nil
	}
	if opened {
		ok("Opened %s", h.KeysURL())
	} else {
		info("Add it at %s", h.KeysURL())
	}
	fmt.Printf("Waiting for %s to accept the key (Ctrl-C to stop)…\n", h.Name)
	deadline := time.Now().Add(wait)
	for {
		r := m.Test(context.Background(), a, key)
		switch {
		case r.OK && a.Username != "" && r.Username != "" && !strings.EqualFold(r.Username, a.Username):
			return fmt.Errorf("%s says this key belongs to @%s, not @%s; remove it there and add it to @%s", h.Name, r.Username, a.Username, a.Username)
		case r.OK:
			if a.Username == "" && r.Username != "" {
				a.Username = r.Username
			}
			ok("%s accepted the key%s", h.Domain, whoSuffix(r))
			return nil
		case r.Kind != "denied" && r.Kind != "network":
			return errors.New(r.Message)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s didn't accept the key within %s; add it and run again, or use --no-verify", h.Name, wait)
		}
		time.Sleep(3 * time.Second)
	}
}

func whoSuffix(r hosts.Result) string {
	if r.Username == "" {
		return ""
	}
	return " (Hi @" + r.Username + "!)"
}

func removeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use: "remove <account>", Aliases: []string{"rm"}, Short: "Remove an account",
		Long: `Remove an account. A key git-account created goes to the trash for 30 days;
a key you brought yourself is left where it is. "git-account undo" brings
the account back.`,
		Example: `  git-account remove old-client
  git-account remove old-client --yes`,
		Args: needArgs(1, 1, true, "git-account remove old-client"), ValidArgsFunction: accountArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			if _, err := m.Account(args[0]); err != nil {
				return err
			}
			if err := confirm("Remove "+args[0]+"?", yes); err != nil {
				return err
			}
			if err := m.Remove(args[0]); err != nil {
				return err
			}
			ok("Removed %s. Run `git-account undo` to bring it back.", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask for confirmation")
	return cmd
}

func editCmd() *cobra.Command {
	var name, email, username string
	cmd := &cobra.Command{
		Use: "edit <account>", Short: "Change an account's name, email or username",
		Long: "Change the name and email new commits use, or your username on the host. Commits you already made keep their author.",
		Example: `  git-account edit work --email kartik@acme-corp.com
  git-account edit work --name "K. Dhawan" --username kdhawan`,
		Args: needArgs(1, 1, true, "git-account edit work --email new@acme.com"), ValidArgsFunction: accountArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			a, err := m.Account(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("email") && !cmd.Flags().Changed("username") {
				return errors.New("nothing to change; add --name, --email or --username")
			}
			if !cmd.Flags().Changed("name") {
				name = a.Name
			}
			if !cmd.Flags().Changed("email") {
				email = a.Email
			}
			if !cmd.Flags().Changed("username") {
				username = a.Username
			}
			if err := m.Edit(a.ID, name, email, strings.TrimPrefix(username, "@")); err != nil {
				return err
			}
			ok("Updated %s", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "your name for commits")
	cmd.Flags().StringVar(&email, "email", "", "your email for commits")
	cmd.Flags().StringVar(&username, "username", "", "your username on the host")
	return cmd
}

func renameCmd() *cobra.Command {
	return &cobra.Command{
		Use: "rename <account> <new-name>", Short: "Rename an account", ValidArgsFunction: accountArgs,
		Long:    "Change an account's short name. Folders, the default and repos pinned to it follow the new name.",
		Example: "  git-account rename work acme",
		Args:    needArgs(2, 2, true, "git-account rename work acme"),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			if err := m.Rename(args[0], args[1]); err != nil {
				return err
			}
			ok("Renamed %s to %s", args[0], args[1])
			return nil
		},
	}
}

func folderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "folder", Short: "Choose accounts automatically by folder",
		Long: `Make every repo inside a folder use an account, now and later. git itself
picks the account, so it works in the terminal, in IDEs and in GUI clients.
A deeper folder wins over the folder around it.`,
		Example: `  git-account folder add work ~/work
  git-account folder list
  git-account folder remove work ~/work`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use: "add <account> <folder>", Short: "Use an account for every repo inside a folder", ValidArgsFunction: accountArgs,
		Example: `  git-account folder add work ~/work
  git-account folder add client-x ~/clients/x`,
		Args: needArgs(2, 2, true, "git-account folder add work ~/work"),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			stored, err := m.AddFolder(args[0], args[1])
			if err != nil {
				return err
			}
			ok("Repos in %s now use %s", stored, args[0])
			return nil
		},
	}, &cobra.Command{
		Use: "remove <account> <folder>", Aliases: []string{"rm"}, Short: "Stop a folder from choosing an account", ValidArgsFunction: accountArgs,
		Example: "  git-account folder remove work ~/work",
		Args:    needArgs(2, 2, true, "git-account folder remove work ~/work"),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			if err := m.RemoveFolder(args[0], args[1]); err != nil {
				return err
			}
			ok("Removed %s from %s", args[1], args[0])
			return nil
		},
	}, &cobra.Command{
		Use: "list", Short: "List automatic folders", Args: cobra.NoArgs,
		Example: "  git-account folder list",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			n := 0
			for _, a := range m.Cfg.Accounts {
				for _, f := range a.Folders {
					fmt.Printf("%-30s %s\n", f, a.ID)
					n++
				}
			}
			if n == 0 {
				info("No automatic folders yet. Add one with: git-account folder add <account> <folder>")
			}
			return nil
		},
	})
	return cmd
}

func keyOf(id string) (*store.Manager, *config.Account, *keys.Info, error) {
	m, err := open()
	if err != nil {
		return nil, nil, nil, err
	}
	a, err := m.Account(id)
	if err != nil {
		return nil, nil, nil, err
	}
	k, err := m.KeyInfo(a)
	return m, a, k, err
}

func showKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use: "show-key <account>", Short: "Print an account's public key", ValidArgsFunction: accountArgs,
		Long: "Print the public key, the line you paste into the host's SSH key settings. In a terminal the fingerprint and file are shown too.",
		Example: `  git-account show-key work
  git-account show-key work > work.pub`,
		Args: needArgs(1, 1, true, "git-account show-key work"),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, k, err := keyOf(args[0])
			if err != nil {
				return err
			}
			fmt.Println(k.Line())
			if interactive() {
				info("%s · %s", k.Fingerprint, k.Path)
			}
			return nil
		},
	}
}

func copyKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use: "copy-key <account>", Short: "Copy an account's public key to the clipboard", ValidArgsFunction: accountArgs,
		Long:    "Copy the public key to the clipboard. On Linux this needs xclip, xsel or wl-clipboard; without one the key is printed instead.",
		Example: "  git-account copy-key work",
		Args:    needArgs(1, 1, true, "git-account copy-key work"),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, k, err := keyOf(args[0])
			if err != nil {
				return err
			}
			if err := sysx.Copy(k.Line()); err != nil {
				fmt.Println(k.Line())
				return fmt.Errorf("couldn't reach the clipboard (%v); the key is printed above", err)
			}
			ok("Copied %s's public key (%s)", args[0], k.Fingerprint)
			return nil
		},
	}
}

func openCmd() *cobra.Command {
	return &cobra.Command{
		Use: "open <account>", Short: "Open the host's SSH key page for an account", ValidArgsFunction: accountArgs,
		Long:    "Open the page where you add or delete SSH keys on the account's host, and print the fingerprint to look for.",
		Example: "  git-account open work",
		Args:    needArgs(1, 1, true, "git-account open work"),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, a, k, err := keyOf(args[0])
			if err != nil {
				return err
			}
			url := m.Host(a).KeysURL()
			if err := sysx.OpenURL(url); err != nil {
				return fmt.Errorf("couldn't open a browser; go to %s", url)
			}
			ok("Opened %s. Look for %s", url, k.Fingerprint)
			return nil
		},
	}
}

func printResult(r doctor.Result) {
	sym := okSt.Render("✓")
	switch r.Level {
	case doctor.Info:
		sym = dimSt.Render("·")
	case doctor.Warn:
		sym = warnSt.Render("!")
	case doctor.Problem:
		sym = errSt.Render("✗")
	}
	fmt.Println(sym + " " + r.Title)
	if r.Detail != "" && r.Level > doctor.OK {
		fmt.Println("  " + dimSt.Render(r.Detail))
	}
	if r.Fix != nil && r.Level > doctor.OK {
		fmt.Println("  " + dimSt.Render("fix: "+r.Fix.Label))
	}
}

func testCmd() *cobra.Command {
	return &cobra.Command{
		Use: "test [account...]", Short: "Check that each account's key signs in as the right user", ValidArgsFunction: accountArgs,
		Long: `Connect to each account's host with only that account's key and check who
the host says you are. With no names, every account is tested. Exits with
status 1 if any account needs attention.`,
		Example: `  git-account test
  git-account test work personal`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			var list []config.Account
			if len(args) == 0 {
				list = m.Cfg.Accounts
			}
			for _, id := range args {
				a, err := m.Account(id)
				if err != nil {
					return err
				}
				list = append(list, *a)
			}
			if len(list) == 0 {
				info("No accounts to test.")
				return nil
			}
			failed := 0
			for _, a := range list {
				r := doctor.Connection(context.Background(), m, a)
				printResult(r)
				if r.Level >= doctor.Warn {
					failed++
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d accounts need attention; `git-account check --fix` can fix most problems", failed, len(list))
			}
			return nil
		},
	}
}

func runChecks(m *store.Manager) []doctor.Result {
	rs := doctor.Local(m)
	type res struct {
		i int
		r doctor.Result
	}
	ch := make(chan res)
	for i, a := range m.Cfg.Accounts {
		go func(i int, a config.Account) { ch <- res{i, doctor.Connection(context.Background(), m, a)} }(i, a)
	}
	conn := make([]doctor.Result, len(m.Cfg.Accounts))
	for range m.Cfg.Accounts {
		r := <-ch
		conn[r.i] = r.r
	}
	rs = append(rs, conn...)
	doctor.Sort(rs)
	return rs
}

func checkCmd() *cobra.Command {
	var fix, asJSON bool
	cmd := &cobra.Command{
		Use: "check", Aliases: []string{"doctor"}, Short: "Find problems and optionally fix them", Args: cobra.NoArgs,
		Long: `Check git, OpenSSH and ssh-agent; settings that override your accounts
(a global core.sshCommand, GIT_SSH_COMMAND, a Host block with its own key);
each key's file, permissions and age; and each account's connection,
including whether port 22 is blocked and port 443 works instead.

With --fix, every problem that has a fix is fixed. Some fixes need a
terminal: loading a key asks for its passphrase, and starting the Windows
ssh-agent service asks for administrator rights. Exits with status 1 if
anything still needs attention.`,
		Example: `  git-account check
  git-account check --fix
  git-account check --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			rs := runChecks(m)
			if asJSON {
				type out struct {
					Level   string `json:"level"`
					Title   string `json:"title"`
					Detail  string `json:"detail,omitempty"`
					Account string `json:"account,omitempty"`
					Fix     string `json:"fix,omitempty"`
				}
				levels := []string{"ok", "info", "warn", "problem"}
				list := []out{}
				for _, r := range rs {
					o := out{Level: levels[r.Level], Title: r.Title, Detail: r.Detail, Account: r.Account}
					if r.Fix != nil {
						o.Fix = r.Fix.Label
					}
					list = append(list, o)
				}
				if err := printJSON(list); err != nil {
					return err
				}
				if doctor.Attention(rs) > 0 {
					return errors.New("some things need attention")
				}
				return nil
			}
			for _, r := range rs {
				printResult(r)
			}
			n := doctor.Attention(rs)
			if n == 0 {
				fmt.Println()
				ok("All good")
				return nil
			}
			if !fix {
				fmt.Println()
				return fmt.Errorf("%d things need attention; run `git-account check --fix` or open `git-account` and press c", n)
			}
			fmt.Println()
			for _, r := range rs {
				if r.Fix == nil || r.Level < doctor.Warn {
					continue
				}
				fmt.Println(boldSt.Render("→ " + r.Fix.Label))
				var ferr error
				if r.Fix.Cmd != nil {
					if !interactive() {
						warn("skipped: needs a terminal")
						continue
					}
					c := r.Fix.Cmd()
					c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
					ferr = c.Run()
				} else {
					ferr = r.Fix.Run()
				}
				if ferr != nil {
					warn("didn't work: %v", ferr)
				}
				_ = m.Reload()
			}
			fmt.Println()
			left := doctor.Attention(runChecks(m))
			if left > 0 {
				return fmt.Errorf("%d things still need attention; see `git-account check`", left)
			}
			ok("All good now")
			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "apply the fixes")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func loadKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use: "load-key <account>", Short: "Load an account's key into ssh-agent", ValidArgsFunction: accountArgs,
		Long: `Load a passphrase-protected key into ssh-agent, so pushes stop asking for
the passphrase. On macOS the passphrase is also saved in Keychain.`,
		Example: "  git-account load-key work",
		Args:    needArgs(1, 1, true, "git-account load-key work"),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, a, _, err := keyOf(args[0])
			if err != nil {
				return err
			}
			if m.SSH == nil {
				return errors.New("OpenSSH isn't installed")
			}
			c := m.SSH.LoadCmd(m.KeyPath(a))
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			return c.Run()
		},
	}
}

func replaceKeyCmd() *cobra.Command {
	var passStdin, noBrowser bool
	var wait time.Duration
	cmd := &cobra.Command{
		Use: "replace-key <account>", Short: "Create a new key and switch to it once the host accepts it", ValidArgsFunction: accountArgs,
		Long: `Create a new key for an account. The public key is printed and copied and
the host's page opens. Once the host accepts the new key, the account
switches to it and the old key goes to the trash. Until then nothing
changes, so the old key keeps working. Delete the old key on the host
afterwards; its fingerprint is printed.`,
		Example: `  git-account replace-key work
  echo "new passphrase" | git-account replace-key work --passphrase-stdin`,
		Args: needArgs(1, 1, true, "git-account replace-key work"),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			a, err := m.Account(args[0])
			if err != nil {
				return err
			}
			// A missing or unreadable old key is a fine reason to replace it.
			old, _ := m.KeyInfo(a)
			typ := "ed25519"
			if old != nil && old.ShortType() == "rsa" {
				typ = "rsa"
			}
			var pass []byte
			switch {
			case passStdin:
				pass = []byte(readLine())
			case interactive():
				p1 := askSecret("Passphrase for the new key (Enter for none):")
				if p1 != "" && askSecret("Type it again:") != p1 {
					return errors.New("the passphrases don't match")
				}
				pass = []byte(p1)
			}
			path := m.NewKeyPath(a.ID, typ)
			if _, _, err := keys.Generate(path, typ, a.Email, pass); err != nil {
				return err
			}
			ok("Created %s", m.P.Pretty(path))
			if err := handOffAndWait(m, a, path, !noBrowser, wait); err != nil {
				keys.Delete(path)
				return err
			}
			if err := m.ReplaceKey(a.ID, path); err != nil {
				keys.Delete(path)
				return err
			}
			ok("%s now uses the new key. The old one is in the trash for 30 days.", a.ID)
			if old != nil {
				warn("Delete the old key on %s too: %s", m.Host(a).Name, old.Fingerprint)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&passStdin, "passphrase-stdin", false, "read the new key's passphrase from standard input")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "don't open the host's SSH key page")
	cmd.Flags().DurationVar(&wait, "wait", 10*time.Minute, "how long to wait for the host to accept the key")
	return cmd
}

// repoDir is the folder git clone creates for url.
func repoDir(url string) string {
	u := strings.TrimSuffix(strings.TrimRight(url, "/"), ".git")
	if i := strings.LastIndexAny(u, "/:"); i >= 0 {
		u = u[i+1:]
	}
	return u
}

func cloneCmd() *cobra.Command {
	return &cobra.Command{
		Use: "clone <account> <url> [folder]", Short: "Clone with an account's key and pin the new repo to it", ValidArgsFunction: accountArgs,
		Long: `Clone a repository using an account's key, then pin the new repo to that
account. Use this for private repos outside your automatic folders, where
no rule can apply yet because the repo doesn't exist.`,
		Example: `  git-account clone work git@github.com:acme/api.git
  git-account clone work git@github.com:acme/api.git ~/code/api`,
		Args: needArgs(2, 3, true, "git-account clone work git@github.com:acme/api.git"),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			a, err := m.Account(args[0])
			if err != nil {
				return err
			}
			dir := repoDir(args[1])
			if len(args) == 3 {
				dir = args[2]
			}
			c := exec.Command(m.Git.Bin, "-c", "core.sshCommand="+m.SSHCommand(m.KeyPath(a)), "clone", args[1], dir)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("git clone failed")
			}
			abs, _ := filepath.Abs(dir)
			if _, err := m.UseHere(abs, a.ID); err != nil {
				return err
			}
			ok("Cloned into %s and pinned it to %s", dir, a.ID)
			return nil
		},
	}
}

func undoCmd() *cobra.Command {
	return &cobra.Command{
		Use: "undo", Short: "Undo the last change git-account made", Args: cobra.NoArgs,
		Long:    "Put back every file the last change touched. Run it again to go further back; `git-account history` lists what can be undone.",
		Example: "  git-account undo",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			label, err := m.Undo()
			if err != nil {
				return err
			}
			ok("Undid: %s", label)
			return nil
		},
	}
}

func historyCmd() *cobra.Command {
	return &cobra.Command{
		Use: "history", Short: "List recent changes you can undo", Args: cobra.NoArgs,
		Long:    "List the last 30 changes, newest first. `git-account undo` reverts the top one.",
		Example: "  git-account history",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			h := m.History()
			if len(h) == 0 {
				info("No changes yet.")
				return nil
			}
			for i, s := range h {
				n := ""
				if i == 0 {
					n = dimSt.Render("  ← git-account undo")
				}
				fmt.Printf("%s  %s%s\n", dimSt.Render(s.Created.Format("2 Jan 15:04")), s.Label, n)
			}
			return nil
		},
	}
}

func themeCmd() *cobra.Command {
	return &cobra.Command{
		Use: "theme [name]", Short: "List themes or pick one", Args: needArgs(0, 1, false, "git-account theme dracula"),
		Long: `With no name, list the themes and mark the current one. With a name, use it.
"auto" follows your terminal's light or dark background. Your own themes
are .toml files in the themes folder next to config.toml.`,
		Example: `  git-account theme
  git-account theme dracula
  git-account theme auto`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			all, errs := theme.All(m.P.ThemesDir())
			for _, e := range errs {
				warn("%v", e)
			}
			if len(args) == 0 {
				cur := m.Cfg.Theme
				if cur == "" {
					cur = "auto"
				}
				ids := []string{"auto"}
				for _, t := range all {
					ids = append(ids, t.ID)
				}
				for _, id := range ids {
					mark := "  "
					if id == cur {
						mark = okSt.Render("● ")
					}
					name := "Auto (matches your terminal)"
					if t, found := theme.Find(all, id); found {
						name = t.Name + dimSt.Render(" · "+t.Kind) + " " + theme.Swatch(t)
					}
					fmt.Printf("%s%-18s %s\n", mark, id, name)
				}
				info("\nYour own themes go in %s", m.P.Pretty(m.P.ThemesDir()))
				return nil
			}
			id := args[0]
			if _, found := theme.Find(all, id); !found && id != "auto" {
				return fmt.Errorf("there's no theme called %s; run `git-account theme` to list them", id)
			}
			if err := m.SetTheme(id); err != nil {
				return err
			}
			ok("Theme: %s", id)
			return nil
		},
	}
}

func syncCmd() *cobra.Command {
	return &cobra.Command{
		Use: "sync", Short: "Rebuild git-account's files from config.toml", Args: cobra.NoArgs,
		Long:    "Regenerate every file git-account manages from config.toml. Run it after editing config.toml by hand.",
		Example: "  git-account sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			if err := m.Sync(); err != nil {
				return err
			}
			ok("Files match config.toml")
			return nil
		},
	}
}

func uninstallCmd() *cobra.Command {
	var purge, yes bool
	cmd := &cobra.Command{
		Use: "uninstall", Short: "Remove git-account's lines and files from this computer", Args: cobra.NoArgs,
		Long: `Remove git-account's block from your global git config and ~/.ssh/config
and delete the files it generated. Your accounts stay in config.toml and
your keys stay where they are, unless you add --purge. "git-account undo"
reverses it. Delete the program itself afterwards.`,
		Example: `  git-account uninstall
  git-account uninstall --purge --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			q := "Remove git-account's settings from this computer?"
			if purge {
				q = "Remove git-account's settings, config.toml and the keys it created (" + strconv.Itoa(len(m.Cfg.Accounts)) + " accounts)?"
			}
			if err := confirm(q, yes); err != nil {
				return err
			}
			if err := m.Uninstall(purge); err != nil {
				return err
			}
			ok("Removed. `git-account undo` puts it all back.")
			info("Delete the git-account program itself the way you installed it.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete config.toml and move the keys git-account created to the trash")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask for confirmation")
	return cmd
}
