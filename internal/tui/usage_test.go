package tui

import (
	"strings"
	"testing"
)

// 热点排序：频次降序、同频名称序、常用项带 ★ 标记。
func TestSortCandidatesHot(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // usage.json 隔离
	BumpUsage(usageSSHHost + "lane")
	BumpUsage(usageSSHHost + "lane")
	BumpUsage(usageSSHHost + "ht-1")

	items := []sourceCandidate{
		{label: "alpha", value: "alpha"},
		{label: "lane", value: "lane"},
		{label: "ht-1", value: "ht-1"},
		{label: "beta", value: "beta"},
	}
	got := sortCandidatesHot(items, usageSSHHost, true)
	// lane(2次) > ht-1(1次) > alpha == beta(0次按名称)
	if got[0].value != "lane" || got[1].value != "ht-1" || got[2].value != "alpha" || got[3].value != "beta" {
		t.Fatalf("热点排序错误: %v", got)
	}
	if !strings.HasPrefix(got[0].label, "★") {
		t.Fatalf("常用项应有 ★ 标记: %q", got[0].label)
	}
	if strings.HasPrefix(got[2].label, "★") {
		t.Fatal("未用项不应有 ★ 标记")
	}
}

// sshCandidates 按频次排序（常用主机第一位）。
func TestSSHCandidatesHotSorted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := sshHostCandidates
	defer func() { sshHostCandidates = old }()
	sshHostCandidates = []string{"aaa", "lane", "zzz"}
	BumpUsage(usageSSHHost + "lane")

	got := sshCandidates()
	if got[0].value != "lane" {
		t.Fatalf("常用主机 lane 应排第一: %v", got)
	}
	if !strings.HasPrefix(got[0].label, "★") {
		t.Fatalf("lane 应有 ★ 标记: %q", got[0].label)
	}
}

// 频次持久化：Bump 后重新加载仍在（跨会话热点记忆）。
func TestUsagePersisted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	BumpUsage(usageK8sRes + "deployment/batch")
	// 清内存缓存模拟新会话
	usageMu.Lock()
	usageData = nil
	usageMu.Unlock()
	if UsageCount(usageK8sRes + "deployment/batch") != 1 {
		t.Fatal("频次应从磁盘恢复")
	}
}
