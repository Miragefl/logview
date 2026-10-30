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
