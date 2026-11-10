package logcore

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

// 被删主题名不再注册，且回落 DarkTheme(老配置 theme: nord 静默降级,不 panic)。
func TestRemovedThemesFallBack(t *testing.T) {
	for _, gone := range []string{"nord", "catppuccin", "one-dark", "catppuccin-latte", "nord-light"} {
		for _, n := range ThemeNames() {
			if n == gone {
				t.Errorf("removed theme %s still registered", gone)
			}
		}
		if cfg := ResolveTheme(gone, nil); cfg.TitleBG != DarkTheme.TitleBG {
			t.Errorf("removed theme %s should fall back to dark", gone)
		}
	}
}

// theme_colors 覆盖在任意主题上生效。
func TestThemeColorOverride(t *testing.T) {
	cfg := ResolveTheme("kanagawa", map[string]string{"level.error": "#FF0000"})
	if cfg.LevelError != "#FF0000" {
		t.Errorf("override not applied: %s", cfg.LevelError)
	}
}

// 新增 9 款签名槽位抽查(全槽位值见 spec 色值表,防复制错主题)。
func TestNewBuiltinThemesSignatureSlots(t *testing.T) {
	want := map[string][4]string{ // slug: {TitleBG, LevelError, LevelInfo, Bg}
		"solarized-dark": {"#268BD2", "#DC322F", "#A6B34D", ""},
		"kanagawa":       {"#938AA9", "#E82424", "#97B473", ""},
		"everforest":     {"#DF69BA", "#F85552", "#6F8746", ""},
		"rose-pine-moon": {"#C4A7E7", "#EB6F92", "#53869B", ""},
		"monokai-pro":    {"#AB9DF2", "#FF6188", "#A9C38F", ""},
		"synthwave-84":   {"#FF7EDB", "#F841A0", "#50965D", ""},
		"one-half-light": {"#A626A4", "#E45649", "#2F9E44", "#FAFAFA"},
		"rose-pine-dawn": {"#907AA9", "#B4637A", "#266A85", "#FAF4ED"},
		"ayu-light":      {"#9E75C7", "#EA6C6D", "#55B03F", "#F8F9FA"},
	}
	registered := map[string]bool{}
	for _, n := range ThemeNames() {
		registered[n] = true
	}
	for slug, w := range want {
		if !registered[slug] {
			t.Errorf("new theme %s not in ThemeNames", slug)
			continue
		}
		cfg := ResolveTheme(slug, nil)
		if cfg.TitleBG != w[0] || cfg.LevelError != w[1] || cfg.LevelInfo != w[2] || cfg.Bg != w[3] {
			t.Errorf("theme %s signature slots mismatch: got TitleBG=%s Err=%s Info=%s Bg=%s", slug, cfg.TitleBG, cfg.LevelError, cfg.LevelInfo, cfg.Bg)
		}
	}
}
