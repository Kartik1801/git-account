// Package cli is the command line. Running it with no command opens the
// app; every action in the app also has a plain command for scripts.
package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kartik-dhawan-dev/git-account/internal/store"
	"github.com/kartik-dhawan-dev/git-account/internal/tui"
)

var (
	okSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#82d9a8"))
	warnSt = lipgloss.NewStyle().Foreground(lipgloss.Color("#f2b35b"))
	errSt  = lipgloss.NewStyle().Foreground(lipgloss.Color("#f28b81"))
	dimSt  = lipgloss.NewStyle().Faint(true)
	boldSt = lipgloss.NewStyle().Bold(true)
)

func ok(format string, a ...any)   { fmt.Println(okSt.Render("✓") + " " + fmt.Sprintf(format, a...)) }
func warn(format string, a ...any) { fmt.Println(warnSt.Render("!") + " " + fmt.Sprintf(format, a...)) }
func info(format string, a ...any) { fmt.Println(dimSt.Render(fmt.Sprintf(format, a...))) }

func interactive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// stdin is shared so prompts and --passphrase-stdin never lose buffered input.
var stdin = bufio.NewReader(os.Stdin)

func readLine() string {
	line, _ := stdin.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

// ask prompts for a value, offering def when there is one.
func ask(label, def string) string {
	if def != "" {
		fmt.Printf("%s %s ", label, dimSt.Render("["+def+"]"))
	} else {
		fmt.Printf("%s ", label)
	}
	if v := strings.TrimSpace(readLine()); v != "" {
		return v
	}
	return def
}

// askSecret prompts without echoing what is typed.
func askSecret(label string) string {
	fmt.Print(label + " ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return string(b)
}

// confirm asks a yes/no question, or requires --yes when there is no
// terminal to ask on.
func confirm(question string, yes bool) error {
	if yes {
		return nil
	}
	if !interactive() {
		return errors.New("add --yes to confirm when not running in a terminal")
	}
	fmt.Print(question + " [y/N] ")
	if a := strings.ToLower(strings.TrimSpace(readLine())); a == "y" || a == "yes" {
		return nil
	}
	return errors.New("cancelled")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Execute runs the command line and returns the exit code.
func Execute(version string) int {
	if err := NewRoot(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, errSt.Render("✗")+" "+capitalize(err.Error()))
		return 1
	}
	return 0
}

func open() (*store.Manager, error) { return store.Open() }

const rootLong = `git-account keeps several Git identities apart. Each account has its own
name, email and SSH key, and git picks the right one: by folder, by repo,
or your default.

Run "git-account" with no command to open the app. Everything the app does
is also one of the commands below.

Help for one command:  git-account <command> --help   or   git-account help <command>`

const rootExample = `  git-account                          open the app
  git-account status                   which account applies here, and why
  git-account add work --email me@acme.com --username me-acme --folder ~/work
  git-account use work                 use work in this repo
  git-account use personal --global    make personal the default
  git-account copy-key work            copy work's public key
  git-account check --fix              find and fix problems`

var groups = []struct {
	id, title string
	cmds      []string
}{
	{"daily", "Everyday:", []string{"status", "list", "use", "unpin", "default", "clone"}},
	{"accounts", "Accounts and folders:", []string{"add", "edit", "rename", "remove", "folder"}},
	{"keys", "Keys and connections:", []string{"show-key", "copy-key", "open", "test", "load-key", "replace-key"}},
	{"setup", "Safety and setup:", []string{"check", "undo", "history", "sync", "theme", "uninstall", "version"}},
}

// NewRoot builds the whole command tree.
func NewRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "git-account",
		Short:         "Use several Git accounts on one computer without mixing them up",
		Long:          rootLong,
		Example:       rootExample,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := open()
			if err != nil {
				return err
			}
			if !interactive() {
				return status(m, false)
			}
			return tui.Run(m)
		},
	}
	root.SetVersionTemplate("git-account {{.Version}}\n")
	root.AddCommand(
		listCmd(), statusCmd(), useCmd(), unpinCmd(), defaultCmd(), addCmd(), removeCmd(), editCmd(), renameCmd(),
		folderCmd(), showKeyCmd(), copyKeyCmd(), openCmd(), testCmd(), checkCmd(), loadKeyCmd(), replaceKeyCmd(),
		cloneCmd(), undoCmd(), historyCmd(), themeCmd(), syncCmd(), uninstallCmd(), versionCmd(version),
	)
	for _, g := range groups {
		root.AddGroup(&cobra.Group{ID: g.id, Title: g.title})
		for _, name := range g.cmds {
			if c, _, err := root.Find([]string{name}); err == nil && c != root {
				c.GroupID = g.id
			}
		}
	}
	root.SetHelpCommandGroupID("setup")
	root.SetCompletionCommandGroupID("setup")
	return root
}

func versionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print the version", Args: cobra.NoArgs,
		Example: "  git-account version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("git-account %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		},
	}
}

func accountNames() []string {
	m, err := open()
	if err != nil {
		return nil
	}
	var ids []string
	for _, a := range m.Cfg.Accounts {
		ids = append(ids, a.ID)
	}
	return ids
}

// accountArgs completes account names.
func accountArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	var ids []string
	for _, id := range accountNames() {
		if strings.HasPrefix(id, toComplete) {
			ids = append(ids, id)
		}
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}

// needArgs checks the number of arguments and explains what's missing,
// listing your accounts when the first argument is one.
func needArgs(min, max int, account bool, example string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) >= min && (max < 0 || len(args) <= max) {
			return nil
		}
		msg := "usage: " + strings.TrimSuffix(cmd.UseLine(), " [flags]")
		if example != "" {
			msg += "\n  for example: " + example
		}
		if account && len(args) == 0 {
			if names := accountNames(); len(names) > 0 {
				msg += "\n  your accounts: " + strings.Join(names, ", ")
			} else {
				msg += "\n  you don't have any accounts yet; add one with: git-account add <name>"
			}
		}
		return errors.New(msg)
	}
}

func cwd() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	return d
}
