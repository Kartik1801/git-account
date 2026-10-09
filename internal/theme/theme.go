// Package theme holds the color themes and turns one into lipgloss styles.
package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Theme is 14 colors. Status never relies on color alone: every state also
// has a symbol (✓ ! ✗), so the No color theme and NO_COLOR stay readable.
type Theme struct {
	ID     string `toml:"-"`
	Name   string `toml:"name"`
	Kind   string `toml:"kind"` // dark or light
	BG     string `toml:"bg"`
	Bar    string `toml:"bar"`
	Edge   string `toml:"edge"`
	FG     string `toml:"fg"`
	Dim    string `toml:"dim"`
	Faint  string `toml:"faint"`
	Accent string `toml:"accent"`
	OK     string `toml:"ok"`
	Info   string `toml:"info"`
	Warn   string `toml:"warn"`
	Err    string `toml:"err"`
	Row    string `toml:"row"`
	RowSel string `toml:"row_sel"`
	Strong string `toml:"strong"`
	Custom bool   `toml:"-"`
}

func t(id, name, kind string, c ...string) Theme {
	return Theme{ID: id, Name: name, Kind: kind, BG: c[0], Bar: c[1], Edge: c[2], FG: c[3], Dim: c[4], Faint: c[5],
		Accent: c[6], OK: c[7], Info: c[8], Warn: c[9], Err: c[10], Row: c[11], RowSel: c[12], Strong: c[13]}
}

// Builtin themes. Order: bg bar edge fg dim faint accent ok info warn err row rowsel strong.
var Builtin = []Theme{
	t("git-account", "git-account", "dark", "#12171e", "#1a2129", "#29333e", "#d9dfe6", "#76838f", "#3a4550", "#f2b35b", "#82d9a8", "#93b5ff", "#f2b35b", "#f28b81", "#19212a", "#1f2a35", "#ffffff"),
	t("catppuccin", "Catppuccin Mocha", "dark", "#1e1e2e", "#181825", "#313244", "#cdd6f4", "#7f849c", "#45475a", "#cba6f7", "#a6e3a1", "#89b4fa", "#f9e2af", "#f38ba8", "#252536", "#313244", "#ffffff"),
	t("dracula", "Dracula", "dark", "#282a36", "#21222c", "#44475a", "#f8f8f2", "#7a86b8", "#4d5066", "#ff79c6", "#50fa7b", "#8be9fd", "#f1fa8c", "#ff5555", "#2f3241", "#3a3d4e", "#ffffff"),
	t("nord", "Nord", "dark", "#2e3440", "#3b4252", "#434c5e", "#d8dee9", "#8a94a8", "#4c566a", "#88c0d0", "#a3be8c", "#81a1c1", "#ebcb8b", "#bf616a", "#353c4a", "#3b4252", "#eceff4"),
	t("gruvbox", "Gruvbox", "dark", "#282828", "#1d2021", "#3c3836", "#ebdbb2", "#928374", "#504945", "#fe8019", "#b8bb26", "#83a598", "#fabd2f", "#fb4934", "#302f2d", "#3c3836", "#fbf1c7"),
	t("tokyo", "Tokyo Night", "dark", "#1a1b26", "#16161e", "#292e42", "#c0caf5", "#6b73a0", "#3b4261", "#ff9e64", "#9ece6a", "#7aa2f7", "#e0af68", "#f7768e", "#1f2130", "#292e42", "#ffffff"),
	t("one-dark", "One Dark", "dark", "#282c34", "#21252b", "#3e4451", "#abb2bf", "#7f848e", "#4b5263", "#c678dd", "#98c379", "#61afef", "#e5c07b", "#e06c75", "#2c313a", "#3e4451", "#ffffff"),
	t("monokai", "Monokai", "dark", "#272822", "#1e1f1c", "#3e3d32", "#f8f8f2", "#908a78", "#49483e", "#fd971f", "#a6e22e", "#66d9ef", "#e6db74", "#f92672", "#2d2e27", "#3e3d32", "#ffffff"),
	t("solarized-dark", "Solarized Dark", "dark", "#002b36", "#073642", "#0e4552", "#93a1a1", "#6c8286", "#365b65", "#2aa198", "#859900", "#268bd2", "#b58900", "#dc322f", "#04313d", "#073642", "#fdf6e3"),
	t("everforest", "Everforest", "dark", "#2d353b", "#232a2e", "#475258", "#d3c6aa", "#859289", "#4f585e", "#e69875", "#a7c080", "#7fbbb3", "#dbbc7f", "#e67e80", "#343f44", "#3d484d", "#fdf6e3"),
	t("kanagawa", "Kanagawa", "dark", "#1f1f28", "#16161d", "#2a2a37", "#dcd7ba", "#727169", "#363646", "#957fb8", "#98bb6c", "#7e9cd8", "#e6c384", "#ff5d62", "#2a2a37", "#363646", "#f2ecbc"),
	t("rosepine", "Rosé Pine", "dark", "#191724", "#1f1d2e", "#26233a", "#e0def4", "#908caa", "#6e6a86", "#ebbcba", "#9ccfd8", "#c4a7e7", "#f6c177", "#eb6f92", "#1f1d2e", "#26233a", "#ffffff"),
	t("ayu-dark", "Ayu Dark", "dark", "#0b0e14", "#0f131a", "#232834", "#bfbdb6", "#6c7380", "#3d424d", "#e6b450", "#aad94c", "#59c2ff", "#ffb454", "#f07178", "#11151c", "#1a1f29", "#ffffff"),
	t("github-dark", "GitHub Dark", "dark", "#0d1117", "#161b22", "#30363d", "#c9d1d9", "#8b949e", "#484f58", "#f78166", "#3fb950", "#58a6ff", "#d29922", "#f85149", "#161b22", "#21262d", "#ffffff"),
	t("synthwave", "Synthwave '84", "dark", "#262335", "#1e1a2e", "#34294f", "#e8e6f0", "#848bbd", "#495495", "#ff7edb", "#72f1b8", "#36f9f6", "#fede5d", "#fe4450", "#2a2139", "#34294f", "#ffffff"),
	// Light themes. A few official colors are a shade darker here so they
	// stay readable as text on the light background.
	t("rosepine-dawn", "Rosé Pine Dawn", "light", "#faf4ed", "#f2e9e1", "#dfdad9", "#575279", "#797593", "#cecacd", "#bd6863", "#286983", "#56949f", "#b87a1e", "#b4637a", "#fffaf3", "#f2e9e1", "#26233a"),
	t("solarized-light", "Solarized Light", "light", "#fdf6e3", "#eee8d5", "#e0d9c3", "#073642", "#657b83", "#d3cbb4", "#a57c00", "#738a00", "#268bd2", "#cb4b16", "#dc322f", "#f5efdc", "#eee8d5", "#002b36"),
	t("github-light", "GitHub Light", "light", "#ffffff", "#f6f8fa", "#d0d7de", "#1f2328", "#59636e", "#afb8c1", "#bc4c00", "#1a7f37", "#0969da", "#9a6700", "#cf222e", "#f6f8fa", "#ddf4ff", "#0b0d0f"),
	t("catppuccin-latte", "Catppuccin Latte", "light", "#eff1f5", "#e6e9ef", "#ccd0da", "#4c4f69", "#6c6f85", "#9ca0b0", "#8839ef", "#3a8f26", "#1e66f5", "#b86e0b", "#d20f39", "#e6e9ef", "#dce0e8", "#11111b"),
	t("gruvbox-light", "Gruvbox Light", "light", "#fbf1c7", "#ebdbb2", "#d5c4a1", "#3c3836", "#7c6f64", "#bdae93", "#af3a03", "#79740e", "#076678", "#b57614", "#9d0006", "#f2e5bc", "#ebdbb2", "#282828"),
	t("one-light", "One Light", "light", "#fafafa", "#f0f0f1", "#e5e5e6", "#383a42", "#696c77", "#a0a1a7", "#a626a4", "#50a14f", "#4078f2", "#c18401", "#e45649", "#f0f0f1", "#e5e5e6", "#1b1c20"),
	// Accessibility.
	t("high-contrast", "High contrast", "dark", "#000000", "#0a0a0a", "#808080", "#ffffff", "#c8c8c8", "#6e6e6e", "#ffd700", "#3dff8b", "#5cc8ff", "#ffd700", "#ff6b6b", "#0d0d0d", "#1f3a5f", "#ffffff"),
	t("mono", "No color", "dark", "#111111", "#1a1a1a", "#3a3a3a", "#e6e6e6", "#9a9a9a", "#4a4a4a", "#ffffff", "#e6e6e6", "#e6e6e6", "#ffffff", "#ffffff", "#161616", "#2a2a2a", "#ffffff"),
}

var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// All returns the built-in themes plus every *.toml file in dir. Broken
// files are reported and skipped.
func All(dir string) ([]Theme, []error) {
	all := append([]Theme(nil), Builtin...)
	var errs []error
	entries, _ := os.ReadDir(dir)
	var custom []Theme
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		th, err := load(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		custom = append(custom, th)
	}
	sort.Slice(custom, func(i, j int) bool { return custom[i].Name < custom[j].Name })
	return append(all, custom...), errs
}

func load(path string) (Theme, error) {
	var th Theme
	if _, err := toml.DecodeFile(path, &th); err != nil {
		return th, fmt.Errorf("theme %s: %w", filepath.Base(path), err)
	}
	th.ID = "custom:" + strings.TrimSuffix(filepath.Base(path), ".toml")
	th.Custom = true
	if th.Name == "" {
		th.Name = strings.TrimPrefix(th.ID, "custom:")
	}
	if th.Kind != "light" {
		th.Kind = "dark"
	}
	base := Builtin[0]
	if th.Kind == "light" {
		base, _ = Find(Builtin, "rosepine-dawn")
	}
	for _, f := range []struct {
		v   *string
		def string
	}{{&th.BG, base.BG}, {&th.Bar, base.Bar}, {&th.Edge, base.Edge}, {&th.FG, base.FG}, {&th.Dim, base.Dim},
		{&th.Faint, base.Faint}, {&th.Accent, base.Accent}, {&th.OK, base.OK}, {&th.Info, base.Info}, {&th.Warn, base.Warn},
		{&th.Err, base.Err}, {&th.Row, base.Row}, {&th.RowSel, base.RowSel}, {&th.Strong, base.Strong}} {
		if *f.v == "" {
			*f.v = f.def
		} else if !hexRe.MatchString(*f.v) {
			return th, fmt.Errorf("theme %s: %q isn't a color like #1e1e2e", filepath.Base(path), *f.v)
		}
	}
	return th, nil
}

// Find returns the theme with id.
func Find(all []Theme, id string) (Theme, bool) {
	for _, th := range all {
		if th.ID == id {
			return th, true
		}
	}
	return Theme{}, false
}

// Pick resolves a theme setting. "auto" (or empty, or unknown) follows the
// terminal: it picks a dark or light palette and keeps the terminal's own
// background. A named theme also paints its background.
func Pick(all []Theme, id string, dark bool) (th Theme, paint bool) {
	if id != "" && id != "auto" {
		if th, ok := Find(all, id); ok {
			return th, true
		}
	}
	if dark {
		return Builtin[0], false
	}
	th, _ = Find(Builtin, "rosepine-dawn")
	return th, false
}

// Styles are the lipgloss styles for one theme.
type Styles struct {
	T                                                   Theme
	FG, Dim, Faint, Strong, Accent, OK, Info, Warn, Err lipgloss.Style
	Key, Label, Pill, PillWarn, Box                     lipgloss.Style
	bg                                                  string
}

func New(th Theme, paint bool) *Styles {
	c := func(h string) lipgloss.Color { return lipgloss.Color(h) }
	fg := func(h string) lipgloss.Style { return lipgloss.NewStyle().Foreground(c(h)) }
	s := &Styles{
		T: th, FG: fg(th.FG), Dim: fg(th.Dim), Faint: fg(th.Faint), Strong: fg(th.Strong).Bold(true),
		Accent: fg(th.Accent), OK: fg(th.OK), Info: fg(th.Info), Warn: fg(th.Warn), Err: fg(th.Err),
		Key:      fg(th.Accent).Bold(true),
		Label:    fg(th.Dim).Bold(true),
		Pill:     lipgloss.NewStyle().Foreground(c(th.BG)).Background(c(th.OK)).Bold(true),
		PillWarn: lipgloss.NewStyle().Foreground(c(th.BG)).Background(c(th.Warn)).Bold(true),
		Box:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c(th.Edge)).Padding(0, 1),
	}
	if paint {
		if seq := lipgloss.ColorProfile().Color(th.BG).Sequence(true); seq != "" {
			s.bg = termenv.CSI + seq + "m"
		}
	}
	return s
}

// Sel gives a style the selected-row background.
func (s *Styles) Sel(st lipgloss.Style) lipgloss.Style {
	return st.Background(lipgloss.Color(s.T.RowSel))
}

// Swatch draws a theme's main colors as small blocks.
func Swatch(th Theme) string {
	var b strings.Builder
	for _, h := range []string{th.Accent, th.OK, th.Info, th.Warn, th.Err} {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(h)).Render("■"))
	}
	return b.String()
}

// Paint fills the whole screen with the theme's background. Styled text
// ends with a reset, which would also reset the background, so the
// background is switched back on after every reset and each line is padded
// to the full width.
func (s *Styles) Paint(frame string, width, height int) string {
	if s.bg == "" {
		return frame
	}
	lines := strings.Split(frame, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	for i, l := range lines {
		l = strings.ReplaceAll(l, "\x1b[0m", "\x1b[0m"+s.bg)
		l = strings.ReplaceAll(l, "\x1b[m", "\x1b[m"+s.bg)
		if pad := width - lipgloss.Width(l); pad > 0 {
			l += strings.Repeat(" ", pad)
		}
		lines[i] = s.bg + l + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}
