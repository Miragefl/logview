package fview

import (
	"fmt"
	"testing"

	"github.com/Miragefl/logview/internal/usage"
)

// resetUsage HOME 指到临时目录并清 usage 内存缓存(词频落盘/加载隔离)。
func resetUsage(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	usage.ResetForTest()
}

// TestBumpKeywordAndSort 词频累计与排序:高频在前,kind/scope 双隔离。
func TestBumpKeywordAndSort(t *testing.T) {
	resetUsage(t)
	for i := 0; i < 3; i++ {
		BumpKeyword("host1", "hl", "alpha")
	}
	BumpKeyword("host1", "hl", "beta")

	got := Keywords("host1", "hl")
	if len(got) != 2 {
		t.Fatalf("Keywords(hl) = %v, 期望 2 词", got)
	}
	if got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("排序错: %v,期望 alpha(3次) 在前", got)
	}
	// kind 隔离:hide 域不见 hl 词
	if ks := Keywords("host1", "hide"); len(ks) != 0 {
		t.Errorf("hide 域不应看到 hl 词: %v", ks)
	}
	// scope 隔离:其他主机不见
	if ks := Keywords("other", "hl"); len(ks) != 0 {
		t.Errorf("scope 隔离失效: %v", ks)
	}
	// scope 空 = 全局域(本地文件 tab)
	BumpKeyword("", "hl", "global-word")
	if ks := Keywords("", "hl"); len(ks) != 1 || ks[0] != "global-word" {
		t.Errorf("全局域读取失败: %v", ks)
	}
}

// TestKeywordsTopLimit top20 截断。
func TestKeywordsTopLimit(t *testing.T) {
	resetUsage(t)
	for i := 0; i < 25; i++ {
		BumpKeyword("", "hide", fmt.Sprintf("w%02d", i))
	}
	got := Keywords("", "hide")
	if len(got) != 20 {
		t.Fatalf("Keywords = %d 词,期望截断 20", len(got))
	}
}

// TestBumpKeywordBadKind 非法 kind 静默忽略(不产生脏 key)。
func TestBumpKeywordBadKind(t *testing.T) {
	resetUsage(t)
	BumpKeyword("", "bogus", "x")
	BumpKeyword("", "hl", "")
	if ks := Keywords("", "hl"); len(ks) != 0 {
		t.Errorf("非法输入不应产生词条: %v", ks)
	}
}
