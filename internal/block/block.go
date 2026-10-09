// Package block edits the one marked block git-account owns inside files
// that belong to you, such as ~/.gitconfig and ~/.ssh/config. Everything
// outside the markers is left exactly as it was.
package block

import (
	"errors"
	"strings"
)

const (
	Start = "# >>> git-account (managed, do not edit) >>>"
	End   = "# <<< git-account <<<"
)

// ErrUnclosed is returned when a start marker has no end marker. Nothing is
// changed in that case, so a hand-edited file never loses lines.
var ErrUnclosed = errors.New("the git-account block has no end line; fix the file by hand or delete the block")

func lines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func eol(s string) string {
	if strings.Contains(s, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func join(ls []string, nl string) string {
	if len(ls) == 0 {
		return ""
	}
	return strings.Join(ls, nl) + nl
}

func trimTrailingBlank(ls []string) []string {
	for len(ls) > 0 && strings.TrimSpace(ls[len(ls)-1]) == "" {
		ls = ls[:len(ls)-1]
	}
	return ls
}

func trimLeadingBlank(ls []string) []string {
	for len(ls) > 0 && strings.TrimSpace(ls[0]) == "" {
		ls = ls[1:]
	}
	return ls
}

// strip returns the lines of content without any git-account block.
func strip(content string) ([]string, bool, error) {
	ls := lines(content)
	out := make([]string, 0, len(ls))
	found := false
	for i := 0; i < len(ls); i++ {
		if strings.TrimSpace(ls[i]) != Start {
			out = append(out, ls[i])
			continue
		}
		end := -1
		for j := i + 1; j < len(ls); j++ {
			if strings.TrimSpace(ls[j]) == End {
				end = j
				break
			}
		}
		if end < 0 {
			return nil, found, ErrUnclosed
		}
		found = true
		i = end
	}
	return out, found, nil
}

// Has reports whether content contains a git-account block.
func Has(content string) bool {
	for _, l := range lines(content) {
		if strings.TrimSpace(l) == Start {
			return true
		}
	}
	return false
}

// IsLast reports whether the block is the last thing in content apart from
// blank lines and comments.
func IsLast(content string) bool {
	ls := lines(content)
	last := -1
	for i, l := range ls {
		if strings.TrimSpace(l) == End {
			last = i
		}
	}
	if last < 0 {
		return false
	}
	for _, l := range ls[last+1:] {
		t := strings.TrimSpace(l)
		if t != "" && !strings.HasPrefix(t, "#") && !strings.HasPrefix(t, ";") {
			return false
		}
	}
	return true
}

// Append puts the block (body between the markers) at the end of content,
// replacing any earlier copy.
func Append(content string, body []string) (string, error) {
	ls, _, err := strip(content)
	if err != nil {
		return content, err
	}
	ls = trimTrailingBlank(ls)
	if len(ls) > 0 {
		ls = append(ls, "")
	}
	ls = append(ls, Start)
	ls = append(ls, body...)
	ls = append(ls, End)
	return join(ls, eol(content)), nil
}

// Prepend puts the block at the start of content, replacing any earlier copy.
func Prepend(content string, body []string) (string, error) {
	ls, _, err := strip(content)
	if err != nil {
		return content, err
	}
	out := append([]string{Start}, body...)
	out = append(out, End)
	if rest := trimLeadingBlank(ls); len(rest) > 0 {
		out = append(out, "")
		out = append(out, rest...)
	}
	return join(out, eol(content)), nil
}

// Remove deletes the block. found is false when there was none.
func Remove(content string) (string, bool, error) {
	ls, found, err := strip(content)
	if err != nil || !found {
		return content, found, err
	}
	return join(trimTrailingBlank(ls), eol(content)), true, nil
}
