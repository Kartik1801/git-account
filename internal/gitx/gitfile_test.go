package gitx

import (
	"os"
	"path/filepath"
	"testing"
)

// Values must come back from git exactly as written, whatever they contain.
func TestRenderRoundTripsThroughGit(t *testing.T) {
	g, err := Find()
	if err != nil {
		t.Skip("git not installed")
	}
	values := map[string]string{
		"name":  `Kartik "KD" Dhawan \ o/`,
		"email": "me+tag@example.com",
		"cmd":   `'C:/Program Files/OpenSSH/ssh.exe' -i 'C:/Users/me/.ssh/git-account/work/id_ed25519' -o IdentitiesOnly=yes`,
	}
	file := filepath.Join(t.TempDir(), "cfg")
	data := Render([]string{"test"}, []Section{
		{Name: "user", Entries: []Entry{{"name", values["name"]}, {"email", values["email"]}}},
		{Name: "core", Entries: []Entry{{"sshCommand", values["cmd"]}}},
		{Name: "includeIf", Sub: `gitdir/i:C:/Users/me/work "x"/`, Entries: []Entry{{"path", "p"}}},
	})
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"user.name": values["name"], "user.email": values["email"], "core.sshcommand": values["cmd"]} {
		got, ok, err := g.Get("", "--file", file, "--get", key)
		if err != nil || !ok || got != want {
			t.Errorf("%s = %q (ok=%v err=%v), want %q", key, got, ok, err, want)
		}
	}
	got, _, _ := g.Get("", "--file", file, "--get", `includeIf.gitdir/i:C:/Users/me/work "x"/.path`)
	if got != "p" {
		t.Errorf("subsection with quotes didn't round-trip: %q", got)
	}
}
