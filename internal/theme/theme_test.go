package theme

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// luminance is the WCAG relative luminance of a #rrggbb color.
func luminance(hex string) float64 {
	ch := func(i int) float64 {
		v, _ := strconv.ParseUint(hex[i:i+2], 16, 8)
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(1) + 0.7152*ch(3) + 0.0722*ch(5)
}

func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Every built-in theme must be readable: body text at WCAG AA (4.5:1),
// secondary text and status colors at 3:1, including on a selected row.
func TestThemesAreReadable(t *testing.T) {
	ids := map[string]bool{}
	for _, th := range Builtin {
		if ids[th.ID] {
			t.Errorf("two themes have the id %s", th.ID)
		}
		ids[th.ID] = true
		for _, c := range []string{th.BG, th.Bar, th.Edge, th.FG, th.Dim, th.Faint, th.Accent, th.OK, th.Info, th.Warn, th.Err, th.Row, th.RowSel, th.Strong} {
			if !hexRe.MatchString(c) {
				t.Errorf("%s: %q isn't a #rrggbb color", th.ID, c)
			}
		}
		checks := []struct {
			what   string
			fg, bg string
			min    float64
		}{
			{"text", th.FG, th.BG, 4.5},
			{"bold text", th.Strong, th.BG, 4.5},
			{"text on a selected row", th.FG, th.RowSel, 4.5},
			{"secondary text", th.Dim, th.BG, 3},
			{"accent", th.Accent, th.BG, 3},
			{"selected name", th.Accent, th.RowSel, 3},
			{"ok", th.OK, th.BG, 3},
			{"info", th.Info, th.BG, 3},
			{"warning", th.Warn, th.BG, 3},
			{"error", th.Err, th.BG, 3},
			{"the here pill", th.BG, th.OK, 3},
			{"the default pill", th.BG, th.Warn, 3},
		}
		for _, c := range checks {
			if got := contrast(c.fg, c.bg); got < c.min {
				t.Errorf("%s: %s contrast is %.2f:1 (%s on %s), want at least %.1f:1", th.ID, c.what, got, c.fg, c.bg, c.min)
			}
		}
		dark := luminance(th.BG) < luminance(th.FG)
		if dark != (th.Kind == "dark") {
			t.Errorf("%s: kind is %q but the background is %s", th.ID, th.Kind, map[bool]string{true: "dark", false: "light"}[dark])
		}
	}
}

func TestCustomThemeFillsGaps(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mine.toml"), []byte("name = \"Mine\"\nkind = \"light\"\naccent = \"#123456\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.toml"), []byte("accent = \"blue\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	all, errs := All(dir)
	if len(errs) != 1 {
		t.Errorf("want one error for the broken file, got %v", errs)
	}
	mine, ok := Find(all, "custom:mine")
	if !ok {
		t.Fatal("custom theme not loaded")
	}
	light, _ := Find(Builtin, "rosepine-dawn")
	if mine.Accent != "#123456" || mine.BG != light.BG {
		t.Errorf("custom theme should keep its accent and borrow the light defaults, got %+v", mine)
	}
}
