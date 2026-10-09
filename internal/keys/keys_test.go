package keys

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kartik-dhawan-dev/git-account/internal/fsx"
)

func TestGenerateAndLoad(t *testing.T) {
	for _, tc := range []struct {
		typ  string
		pass string
	}{{"ed25519", ""}, {"ed25519", "correct horse"}, {"rsa", ""}} {
		path := filepath.Join(t.TempDir(), "k", "id_"+tc.typ)
		info, priv, err := Generate(path, tc.typ, "me@example.com", []byte(tc.pass))
		if err != nil {
			t.Fatalf("%s: %v", tc.typ, err)
		}
		if priv == nil || !strings.HasPrefix(info.Fingerprint, "SHA256:") {
			t.Fatalf("%s: bad info %+v", tc.typ, info)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Fingerprint != info.Fingerprint || got.Encrypted != (tc.pass != "") || got.Comment != "me@example.com" {
			t.Errorf("%s: loaded %+v, generated %+v", tc.typ, got, info)
		}
		if ok, err := fsx.IsPrivate(path); err != nil || !ok {
			t.Errorf("%s: private key must be private (ok=%v err=%v)", tc.typ, ok, err)
		}
		if _, _, err := Generate(path, tc.typ, "x", nil); err == nil {
			t.Errorf("%s: Generate must refuse to overwrite a key", tc.typ)
		}
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"id_ed25519", "work_key"} {
		if _, _, err := Generate(filepath.Join(dir, n), "ed25519", n, nil); err != nil {
			t.Fatal(err)
		}
	}
	if got := Find(dir); len(got) != 2 {
		t.Errorf("Find found %d keys, want 2", len(got))
	}
}
