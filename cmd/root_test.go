package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandTailArgs(t *testing.T) {
	tests := []struct {
		in, want []string
	}{
		{[]string{"-100f", "app.log"}, []string{"--tail", "100", "-f", "app.log"}},
		{[]string{"-200f"}, []string{"--tail", "200", "-f"}},
		{[]string{"tail", "-f", "a.log"}, []string{"tail", "-f", "a.log"}},
		{[]string{"-f"}, []string{"-f"}},
		{[]string{}, nil},
	}
	for _, tt := range tests {
		got := expandTailArgs(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("expandTailArgs(%v) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("expandTailArgs(%v) = %v, want %v", tt.in, got, tt.want)
				break
			}
		}
	}
}

func TestIsSubcommand(t *testing.T) {
	for _, name := range []string{"tail", "file", "pipe", "k8s", "version"} {
		if !isSubcommand(name) {
			t.Errorf("isSubcommand(%q) = false, want true", name)
		}
	}
	// cobra 动态补全协议命令必须豁免 picker/pipe 注入（shell 补全依赖）
	for _, name := range []string{"__complete", "__completeNoDesc"} {
		if !isSubcommand(name) {
			t.Errorf("isSubcommand(%q) = false, want true (shell completion broken)", name)
		}
	}
	if isSubcommand("notacommand") {
		t.Error("isSubcommand(notacommand) = true, want false")
	}
}

// withTestConfigDir 把 --config 指到临时目录，返回该目录。
func withTestConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	configDir = dir
	t.Cleanup(func() { configDir = "" })
	return dir
}

func TestLoadConfigCreatesDefaultFile(t *testing.T) {
	dir := withTestConfigDir(t)
	bufferSize = 1000

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	// 首次运行应生成默认 rules.yaml 并加载成功
	if _, err := os.Stat(filepath.Join(dir, "rules.yaml")); err != nil {
		t.Errorf("default rules.yaml not created: %v", err)
	}
	if cfg.parsers == nil {
		t.Error("cfg.parsers should not be nil")
	}
	if cfg.history != 5000 {
		t.Errorf("history = %d, want 5000", cfg.history)
	}
	if cfg.rulesPath != filepath.Join(dir, "rules.yaml") {
		t.Errorf("rulesPath = %q", cfg.rulesPath)
	}
}

func TestLoadConfigInvalidBufferSize(t *testing.T) {
	withTestConfigDir(t)
	bufferSize = 0
	t.Cleanup(func() { bufferSize = 100000 })

	if _, err := loadConfig(); err == nil {
		t.Fatal("loadConfig() should reject --buffer-size < 1")
	}
}

func TestLoadConfigInvalidYAMLWarnsAndFallsBack(t *testing.T) {
	dir := withTestConfigDir(t)
	bufferSize = 1000
	if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), []byte("rules: [broken"), 0644); err != nil {
		t.Fatal(err)
	}

	// 坏 YAML：stderr 警告 + 默认规则兜底，不返回错误
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() should not fail on invalid yaml, got %v", err)
	}
	if cfg.parsers == nil {
		t.Error("cfg.parsers should fall back to defaults")
	}
}

func TestLoadConfigInvalidRegexFails(t *testing.T) {
	dir := withTestConfigDir(t)
	bufferSize = 1000
	yaml := "rules:\n  - name: bad\n    pattern: '(?P<x>['\n"
	if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig() should fail on invalid regex rule")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("error should mention rule name, got: %v", err)
	}
}

func TestArgsHasPositional(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"-f"}, false},
		{[]string{"--rule", "json-log"}, false},
		{[]string{"--buffer-size", "5000", "-f"}, false},
		{[]string{"--tail", "200"}, false},
		{[]string{"app.log"}, true},
		{[]string{"-f", "app.log"}, true},
		{[]string{"--config", "/tmp/x", "a.log", "-f"}, true},
	}
	for _, c := range cases {
		if got := argsHasPositional(c.args); got != c.want {
			t.Errorf("argsHasPositional(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}
