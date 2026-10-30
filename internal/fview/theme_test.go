package fview

import (
	"strings"
	"testing"

	"github.com/justfun/logview/internal/logcore"
)

// TestThemeToCSSKeys 每个内置主题产出含 bg/fg/级别色键且值为 #hex（计划 Task 12 Step 1）。
func TestThemeToCSSKeys(t *testing.T) {
	for _, name := range logcore.ThemeNames() {
		css := ThemeToCSS(logcore.ResolveTheme(name, nil))
		for _, key := range []string{"--bg", "--fg", "--level-error", "--level-warn", "--level-info", "--level-debug"} {
			v, ok := css[key]
			if !ok || v == "" {
				t.Errorf("主题 %s: 缺 CSS 键 %s", name, key)
				continue
			}
			if !strings.HasPrefix(v, "#") {
				t.Errorf("主题 %s: %s = %q 非 #hex", name, key, v)
			}
		}
	}
}

// TestThemeToCSSFieldColors 字段色全量导出：Source/TraceID/Thread/Selected/Visual
// 必须进 CSS 变量（验收反馈⑤——治"日志文字全白"）。
func TestThemeToCSSFieldColors(t *testing.T) {
	css := ThemeToCSS(logcore.DarkTheme)
	want := map[string]string{
		"--source":   "#D7AFFF",
		"--trace":    "#87FFFF",
		"--thread":   "#9E9E9E",
		"--selected": "#30354F",
		"--visual":   "#008700",
	}
	for key, w := range want {
		v, ok := css[key]
		if !ok {
			t.Errorf("缺 CSS 键 %s（DarkTheme 应导出全部字段色）", key)
			continue
		}
		if v != w {
			t.Errorf("%s = %q, 期望 %q", key, v, w)
		}
	}
}
