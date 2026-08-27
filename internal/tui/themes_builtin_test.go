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
}
