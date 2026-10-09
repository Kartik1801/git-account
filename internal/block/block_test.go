package block

import (
	"strings"
	"testing"
)

func TestAppendReplacesAndKeepsRest(t *testing.T) {
	in := "[user]\n\tname = Me\n"
	out, err := Append(in, []string{"[include]", "\tpath = a"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, in) || !IsLast(out) {
		t.Fatalf("unexpected:\n%s", out)
	}
	again, _ := Append(out, []string{"[include]", "\tpath = a"})
	if again != out {
		t.Errorf("appending the same block twice changed the file:\n%s", again)
	}
	back, found, err := Remove(out)
	if err != nil || !found || back != in {
		t.Errorf("remove gave %q (found=%v err=%v), want the original", back, found, err)
	}
}

func TestMovesBlockToEnd(t *testing.T) {
	in := Start + "\n[include]\n" + End + "\n[user]\n\temail = later@example.com\n"
	if IsLast(in) {
		t.Fatal("block isn't last here")
	}
	out, _ := Append(in, []string{"[include]"})
	if !IsLast(out) || strings.Count(out, Start) != 1 {
		t.Errorf("block should move to the end once:\n%s", out)
	}
}

func TestKeepsCRLF(t *testing.T) {
	out, _ := Append("[user]\r\n\tname = Me\r\n", []string{"x"})
	if strings.Count(out, "\r\n") != strings.Count(out, "\n") {
		t.Errorf("line endings mixed:\n%q", out)
	}
}

func TestUnclosedBlockIsNotTouched(t *testing.T) {
	in := "a\n" + Start + "\nb\nc\n"
	if _, err := Append(in, []string{"x"}); err != ErrUnclosed {
		t.Errorf("err = %v, want ErrUnclosed", err)
	}
}

func TestPrepend(t *testing.T) {
	out, _ := Prepend("Host *\n  AddKeysToAgent yes\n", []string{"Host github.com", "Host *"})
	if !strings.HasPrefix(out, Start+"\n") || !strings.HasSuffix(out, "Host *\n  AddKeysToAgent yes\n") {
		t.Errorf("unexpected:\n%s", out)
	}
}
