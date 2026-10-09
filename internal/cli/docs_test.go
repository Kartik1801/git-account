package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Every command needs a summary and an example, belongs to a help group,
// and is described with all its flags in docs/commands.md. This keeps the
// help and the docs from drifting when commands change.
func TestCommandsAreDocumented(t *testing.T) {
	data, err := os.ReadFile("../../docs/commands.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	root := NewRoot("test")
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Hidden {
				continue
			}
			path := sub.CommandPath()
			if sub.Short == "" {
				t.Errorf("%s has no short description", path)
			}
			if sub.Example == "" {
				t.Errorf("%s has no example", path)
			}
			if c == root && sub.GroupID == "" {
				t.Errorf("%s isn't in a help group; add it to groups in cli.go", path)
			}
			if !strings.Contains(doc, "`"+path) {
				t.Errorf("docs/commands.md doesn't mention `%s`", path)
			}
			sub.LocalFlags().VisitAll(func(f *pflag.Flag) {
				if f.Name != "help" && !strings.Contains(doc, "--"+f.Name) {
					t.Errorf("docs/commands.md doesn't mention %s --%s", path, f.Name)
				}
			})
			walk(sub)
		}
	}
	walk(root)
}

func TestHelpShowsGitAccount(t *testing.T) {
	root := NewRoot("test")
	use, _, err := root.Find([]string{"use"})
	if err != nil {
		t.Fatal(err)
	}
	if got := use.UseLine(); !strings.HasPrefix(got, "git-account use") {
		t.Errorf("usage line is %q, want it to start with \"git-account use\"", got)
	}
}

func TestRepoDir(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:acme/api.git":        "api",
		"https://github.com/acme/api.git":    "api",
		"https://gitlab.com/acme/tools/cli":  "cli",
		"ssh://git@host:2222/team/repo.git/": "repo",
	} {
		if got := repoDir(url); got != want {
			t.Errorf("repoDir(%q) = %q, want %q", url, got, want)
		}
	}
}
