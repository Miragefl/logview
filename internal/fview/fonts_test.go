package fview

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFontsInDirs 扫描目录树收集字体文件名（去扩展、去重、升序；忽略非字体）。
func TestFontsInDirs(t *testing.T) {
	root := t.TempDir()
	// 目录结构：根 2 个字体 + 1 个非字体；子目录 Supplemental 1 个 + 与根重名 1 个
	mustWrite(t, filepath.Join(root, "SFMono.ttf"), "")
	mustWrite(t, filepath.Join(root, "PingFang.otf"), "")
	mustWrite(t, filepath.Join(root, "readme.txt"), "")
	sub := filepath.Join(root, "Supplemental")
	mustWrite(t, filepath.Join(sub, "Songti.ttc"), "")
	mustWrite(t, filepath.Join(sub, "SFMono.ttf"), "") // 与根重名 → 去重

	got := fontsInDirs([]string{root})
	want := []string{"PingFang", "SFMono", "Songti"}
	if len(got) != len(want) {
		t.Fatalf("应收集 3 个字体名，实际 %v", got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("应升序去重: want %v got %v", want, got)
		}
	}
}

// TestFontsInDirsMissingDir 目录不存在时静默跳过（首跑机器字体目录缺失）。
func TestFontsInDirsMissingDir(t *testing.T) {
	got := fontsInDirs([]string{filepath.Join(t.TempDir(), "nope")})
	if len(got) != 0 {
		t.Fatalf("缺失目录应返回空，实际 %v", got)
	}
}

// TestSystemFontsNotEmpty 当前机器至少能枚举出字体（macOS 系统字体目录必在）。
func TestSystemFontsNotEmpty(t *testing.T) {
	got := SystemFonts()
	if len(got) == 0 {
		t.Fatal("SystemFonts 应返回非空列表")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
