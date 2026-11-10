package fview

import (
	"testing"

	"github.com/Miragefl/logview/internal/usage"
)

// TestUsageRecentAndTouch Touch 计频后 Recent 返回该键且居首（热点面板数据源）。
func TestUsageRecentAndTouch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	usage.ResetForTest()

	UsageTouch("file:/tmp/app.log")
	UsageTouch("file:/tmp/other.log")
	UsageTouch("file:/tmp/app.log") // app.log 两次，居首

	got := UsageRecent()
	if len(got) != 2 {
		t.Fatalf("应返回 2 个源 key，实际 %v", got)
	}
	if got[0] != "file:/tmp/app.log" {
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
