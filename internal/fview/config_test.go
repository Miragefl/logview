package fview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justfun/logview/internal/parser"
)

// TestUpdateHidesAndFields 写回共享 rules.yaml：目标键替换、其余键保留（spec 4.5）。
func TestUpdateHidesAndFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	seed := `rules:
  - name: java-logback
    pattern: '.*'
theme: dracula
hides:
  - old-hide
`
	if err := os.WriteFile(path, []byte(seed), 0644); err != nil {
		t.Fatalf("写入种子: %v", err)
	}

	if err := UpdateHides(path, []string{"health", "ping"}); err != nil {
		t.Fatalf("UpdateHides: %v", err)
	}
	res, err := parser.LoadRules(path)
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	if len(res.Hides) != 2 || res.Hides[0] != "health" || res.Hides[1] != "ping" {
		t.Errorf("写回后 hides = %v, 期望 [health ping]", res.Hides)
	}
	if res.Theme != "dracula" {
		t.Errorf("其余键被破坏：theme = %q, 期望 dracula", res.Theme)
	}
	if len(res.Rules) != 1 || res.Rules[0].Name != "java-logback" {
		t.Errorf("其余键被破坏：rules = %v", res.Rules)
	}

	fields := []parser.FieldConfig{{Name: "thread", Visible: false}, {Name: "traceId", Visible: true}}
	if err := UpdateFields(path, fields); err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}
	res, err = parser.LoadRules(path)
	if err != nil {
		t.Fatalf("LoadRules 二次: %v", err)
	}
	if len(res.Fields) != 2 || res.Fields[0].Name != "thread" || !res.Fields[1].Visible {
		t.Errorf("写回后 fields = %v", res.Fields)
	}
	if res.Theme != "dracula" {
		t.Errorf("fields 写回破坏 theme: %q", res.Theme)
	}
}

// TestUpdateHidesCreatesMissingFile rules.yaml 不存在时保存应自动落盘默认（Final Review #11）。
func TestUpdateHidesCreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml") // 不存在
	if err := UpdateHides(path, []string{"kw"}); err != nil {
		t.Fatalf("文件不存在时 UpdateHides 应自动创建: %v", err)
	}
	res, err := parser.LoadRules(path)
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	if len(res.Hides) != 1 || res.Hides[0] != "kw" {
		t.Errorf("落盘后 hides = %v", res.Hides)
	}
	if len(res.Rules) == 0 {
		t.Error("落盘文件应含默认解析规则（纯 fview 新用户开箱即用）")
	}
}
