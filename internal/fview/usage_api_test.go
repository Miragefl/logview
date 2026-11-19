package fview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Miragefl/logview/internal/usage"
)

// TestUsageRecentAndTouch Touch 计频后 Recent 返回该键且居首（热点面板数据源）。
// file: 键须指向真实存在的文件（2026-11-19 起存在性过滤,不存在的剔除不显示）。
func TestUsageRecentAndTouch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	usage.ResetForTest()

	dir := t.TempDir()
	app := filepath.Join(dir, "app.log")
	other := filepath.Join(dir, "other.log")
	for _, p := range []string{app, other} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	UsageTouch("file:" + app)
	UsageTouch("file:" + other)
	UsageTouch("file:" + app) // app.log 两次，居首

	got := UsageRecent()
	if len(got) != 2 {
		t.Fatalf("应返回 2 个源 key，实际 %v", got)
	}
	if got[0] != "file:"+app {
		t.Fatalf("两次使用的 app.log 应居首: %v", got)
	}
}

// TestUsageRecentEmpty usage.json 不存在时空列表不 panic（首跑机器）。
func TestUsageRecentEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	usage.ResetForTest()

	if got := UsageRecent(); len(got) != 0 {
		t.Fatalf("无数据应返回空列表，实际 %v", got)
	}
}

// TestMemStatsBytes 基本口径：运行中进程必然 > 0（Sys-HeapReleased）。
func TestMemStatsBytes(t *testing.T) {
	if got := MemStatsBytes(); got == 0 {
		t.Fatal("MemStatsBytes 应 > 0")
	}
}
