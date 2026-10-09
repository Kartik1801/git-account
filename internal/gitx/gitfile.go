package gitx

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Entry is one key = value line.
type Entry struct {
	Key, Value string
}

// Section is one [name "sub"] block of a git config file.
type Section struct {
	Comment string // written above the section
	Name    string
	Sub     string // subsection; empty for none
	Entries []Entry
}

// Quote returns v as a git config value that git reads back unchanged.
func Quote(v string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\t", `\t`, "\n", `\n`)
	return `"` + r.Replace(v) + `"`
}

func quoteSub(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// Render writes a complete git config file.
func Render(header []string, sections []Section) []byte {
	var b strings.Builder
	for _, h := range header {
		b.WriteString("# " + h + "\n")
	}
	for _, s := range sections {
		b.WriteString("\n")
		if s.Comment != "" {
			for _, c := range strings.Split(s.Comment, "\n") {
				b.WriteString("# " + c + "\n")
			}
		}
		if s.Sub != "" {
			fmt.Fprintf(&b, "[%s %s]\n", s.Name, quoteSub(s.Sub))
		} else {
			fmt.Fprintf(&b, "[%s]\n", s.Name)
		}
		for _, e := range s.Entries {
			fmt.Fprintf(&b, "\t%s = %s\n", e.Key, Quote(e.Value))
		}
	}
	return []byte(b.String())
}

// Slash writes a path with forward slashes, which git accepts on every OS.
func Slash(path string) string { return filepath.ToSlash(path) }
