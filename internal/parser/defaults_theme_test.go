package parser

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Miragefl/logview/internal/logcore"
)

// 默认模板注释推荐的主题名单必须与 logcore 注册表完全一致:
// 注释推荐已删主题 → 用户照配静默回落;新主题零露出 → 不可发现。
func TestDefaultTemplateThemeListMatchesRegistry(t *testing.T) {
	yaml := DefaultRulesYAML()
	re := regexp.MustCompile(`(?m)^#\s+(?:dark 系|light系):\s*(.+)$`)
	var slugs []string
	for _, m := range re.FindAllStringSubmatch(yaml, -1) {
		for s := range strings.SplitSeq(m[1], "/") {
			if s = strings.TrimSpace(s); s != "" {
				slugs = append(slugs, s)
			}
		}
	}
	if len(slugs) == 0 {
		t.Fatal("no theme list found in default template comments")
	}
	registered := map[string]bool{}
	for _, n := range logcore.ThemeNames() {
		registered[n] = true
	}
	// 模板注释里每个 slug 都必须是注册主题(照配不 404)
	for _, s := range slugs {
		if !registered[s] {
			t.Errorf("template recommends unregistered theme %q", s)
		}
	}
	// 注册表里每个主题都必须在模板注释露出(新主题可发现)
	inTemplate := map[string]bool{}
	for _, s := range slugs {
		inTemplate[s] = true
	}
	for n := range registered {
		if !inTemplate[n] {
			t.Errorf("registered theme %q not listed in default template comments", n)
		}
	}
}
