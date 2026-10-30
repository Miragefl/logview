package logcore

import "testing"

// TestResolveThemeKnownNames 每个内置主题名可解析且非零值、IsLightTheme 不 panic；
// 未知名回落 DarkTheme（槽位完整性断言由随迁的 TestBuiltinThemesResolve 覆盖）。
func TestResolveThemeKnownNames(t *testing.T) {
	for _, name := range ThemeNames() {
		cfg := ResolveTheme(name, nil)
		if cfg == (ThemeConfig{}) {
			t.Fatalf("主题 %q 解析为零值", name)
		}
		_ = cfg.IsLightTheme() // 不 panic 即可（明暗语义由内置表自带）
	}
	fallback := ResolveTheme("不存在的主题", nil)
	if fallback.Bg != DarkTheme.Bg || fallback.Fg != DarkTheme.Fg {
		t.Errorf("未知名应回落 DarkTheme，实际 Bg=%s Fg=%s", fallback.Bg, fallback.Fg)
	}
}
