package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 热点排序：频次降序、同频名称序、常用项带 ★ 标记。
func TestSortCandidatesHot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetUsageForTest()
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
	resetUsageForTest()
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
	resetUsageForTest()
	BumpUsage(usageK8sRes + "deployment/batch")
	resetUsageForTest() // 清内存缓存模拟新会话
	if usageScore(usageK8sRes+"deployment/batch") < 0.99 {
		t.Fatal("频次应从磁盘恢复")
	}
}

// 时间衰减：旧使用先打折再累计；长期不用读数衰减。
func TestUsageDecay(t *testing.T) {
	resetUsageForTest()
	m := loadUsage()
	// 30 天前用过 8 次（直接构造，避免 sleep）
	m[usageSSHHost+"old"] = usageEntry{Count: 8, LastUsed: time.Now().Add(-30 * 24 * time.Hour).Unix()}
	usageData = m

	// 衰减读数：8 × 0.5^(30/7) ≈ 8 × 0.0514 ≈ 0.41
	score := usageScore(usageSSHHost + "old")
	if score > 0.6 || score < 0.2 {
		t.Fatalf("30 天前 8 次应衰减到 ~0.41，实际 %.2f", score)
	}

	// 再 Bump 一次：0.41 + 1 ≈ 1.41，LastUsed 刷新
	BumpUsage(usageSSHHost + "old")
	entry := loadUsage()[usageSSHHost+"old"]
	if entry.Count < 1.3 || entry.Count > 1.6 {
		t.Fatalf("衰减后累计应为 ~1.41，实际 %.2f", entry.Count)
	}
}

// 旧格式（纯数字累计计数）迁移为 {count,lastUsed}。
func TestUsageLegacyMigration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	resetUsageForTest()
	dir := filepath.Join(home, ".local", "state", "logview")
	os.MkdirAll(dir, 0755)
	legacy := map[string]float64{"ssh:oldhost": 5}
	data, _ := json.Marshal(legacy)
	os.WriteFile(filepath.Join(dir, "usage.json"), data, 0644)

	m := loadUsage()
	e, ok := m["ssh:oldhost"]
	if !ok || e.Count != 5 {
		t.Fatalf("旧格式应迁移 count=5: %+v", e)
	}
	if e.LastUsed == 0 {
		t.Fatal("迁移应补 LastUsed")
	}
}

// resetUsageForTest 清内存缓存（磁盘数据按 HOME 重新加载）。
func resetUsageForTest() {
	usageMu.Lock()
	usageData = nil
	usageDirty = false
	usageMu.Unlock()
}
