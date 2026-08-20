package tui

import (
	"strings"
	"testing"
)

// 每个内置主题名可解析，且全部语义槽位非空。
func TestBuiltinThemesResolve(t *testing.T) {
	for _, name := range ThemeNames() {
		cfg := ResolveTheme(name, nil)
		if cfg.TitleBG == "" || cfg.LevelError == "" || cfg.Accent == "" {
			t.Errorf("theme %s: key slots empty", name)
		}
		check := map[string]string{
			"TitleFG": cfg.TitleFG, "TitleBG": cfg.TitleBG,
			"LevelDebug": cfg.LevelDebug, "LevelInfo": cfg.LevelInfo,
			"LevelWarn": cfg.LevelWarn, "LevelError": cfg.LevelError,
			"Time": cfg.Time, "Source": cfg.Source, "TraceID": cfg.TraceID,
			"Thread": cfg.Thread, "Highlight": cfg.Highlight,
			"Selected": cfg.Selected, "Visual": cfg.Visual,
			"PopupBorder": cfg.PopupBorder, "PopupBg": cfg.PopupBg,
			"Dim": cfg.Dim, "Accent": cfg.Accent,
		}
		for slot, v := range check {
			if v == "" {
				t.Errorf("theme %s: slot %s empty", name, slot)
			}
			if !strings.HasPrefix(v, "#") {
				t.Errorf("theme %s: slot %s not hex: %q", name, slot, v)
			}
		}
		// light 主题必须设置 Bg/Fg（全屏底色）；dark 系可为空（透终端）
		if cfg.IsLightTheme() && (cfg.Bg == "" || cfg.Fg == "") {
			t.Errorf("light theme %s missing Bg/Fg", name)
		}
	}
}

// 未知主题回退 DarkTheme，不 panic。
func TestResolveThemeFallback(t *testing.T) {
	cfg := ResolveTheme("nonexistent", nil)
	if cfg.TitleBG != DarkTheme.TitleBG {
		t.Errorf("unknown theme should fall back to dark")
	}
}

// theme_colors 覆盖在任意主题上生效。
func TestThemeColorOverride(t *testing.T) {
	cfg := ResolveTheme("dracula", map[string]string{"level.error": "#FF0000"})
	if cfg.LevelError != "#FF0000" {
		t.Errorf("override not applied: %s", cfg.LevelError)
	}
	// 覆盖后色带仍派生自新色值
	if cfg.LevelErrorBg() != darken("#FF0000", 0.45) {
		t.Errorf("error band should derive from overridden red")
	}
}

// ERROR 色带前景/背景亮度差：暗主题白字深底，亮主题深字浅底。
func TestErrorBandContrast(t *testing.T) {
	cases := []struct {
		name  string
		light bool
	}{
		{"dark", false}, {"dracula", false}, {"nord", false},
		{"gruvbox-light", true}, {"solarized-light", true}, {"catppuccin-latte", true},
	}
	for _, tc := range cases {
		cfg := ResolveTheme(tc.name, nil)
		bgR, bgG, bgB := hexToRGB(cfg.LevelErrorBg())
		fgR, fgG, fgB := hexToRGB(cfg.LevelErrorFg())
		bgLuma := 0.299*float64(bgR) + 0.587*float64(bgG) + 0.114*float64(bgB)
		fgLuma := 0.299*float64(fgR) + 0.587*float64(fgG) + 0.114*float64(fgB)
		if diff := fgLuma - bgLuma; tc.light && diff > -60 {
			t.Errorf("%s (light): fg/bg luma diff %.0f too low (dark fg on pale bg expected)", tc.name, diff)
		} else if !tc.light && diff < 60 {
			t.Errorf("%s (dark): fg/bg luma diff %.0f insufficient (white fg on deep red expected)", tc.name, diff)
		}
	}
}
