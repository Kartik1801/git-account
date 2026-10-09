// Command git-account manages several Git accounts on one computer. Put it
// on your PATH and run it as `git-account`.
package main

import (
	"os"

	"github.com/kartik-dhawan-dev/git-account/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() { os.Exit(cli.Execute(version)) }
