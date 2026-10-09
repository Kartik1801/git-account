// Command man writes manual pages for every git-account command:
//
//	go run ./tools/man ./man
//
// Installed where man looks (for example /usr/local/share/man/man1), they
// make `man git-account` and `man git-account-use` work.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/kartik-dhawan-dev/git-account/internal/cli"
)

func main() {
	dir := "man"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root := cli.NewRoot(os.Getenv("VERSION"))
	root.DisableAutoGenTag = true
	placeholders(root)
	header := &doc.GenManHeader{Title: "GIT-ACCOUNT", Section: "1", Source: "git-account", Manual: "git-account manual"}
	if err := doc.GenManTree(root, header, dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var angle = regexp.MustCompile(`<[a-z-]+>`)

// placeholders writes <account> as ACCOUNT, the manual page convention.
// The markdown step would otherwise drop it as an HTML tag.
func placeholders(c *cobra.Command) {
	up := func(s string) string {
		return angle.ReplaceAllStringFunc(s, func(m string) string { return strings.ToUpper(m[1 : len(m)-1]) })
	}
	c.Use, c.Long, c.Example = up(c.Use), up(c.Long), up(c.Example)
	for _, sub := range c.Commands() {
		placeholders(sub)
	}
}
