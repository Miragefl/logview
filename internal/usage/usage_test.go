package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 频次持久化：Bump 后清缓存重载仍在（跨会话热点记忆）。
func TestUsagePersisted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ResetForTest()
	Bump(KeyK8sRes + "deployment/batch")
	ResetForTest() // 清内存缓存模拟新会话
	if Score(KeyK8sRes+"deployment/batch") < 0.99 {
		t.Fatal("频次应从磁盘恢复")
	}
}

// 时间衰减：旧使用先打折再累计；长期不用读数衰减。
func TestUsageDecay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ResetForTest()
	mu.Lock()
	m := loadLocked()
	m[KeySSHHost+"old"] = Entry{Count: 8, LastUsed: time.Now().Add(-30 * 24 * time.Hour).Unix()}
	mu.Unlock()

	// 衰减读数：8 × 0.5^(30/7) ≈ 8 × 0.0514 ≈ 0.41
	score := Score(KeySSHHost + "old")
	if score > 0.6 || score < 0.2 {
		t.Fatalf("30 天前 8 次应衰减到 ~0.41，实际 %.2f", score)
	}

	// 再 Bump 一次：0.41 + 1 ≈ 1.41，LastUsed 刷新
	Bump(KeySSHHost + "old")
	mu.Lock()
	entry := data[KeySSHHost+"old"]
	mu.Unlock()
	if entry.Count < 1.3 || entry.Count > 1.6 {
		t.Fatalf("衰减后累计应为 ~1.41，实际 %.2f", entry.Count)
	}
}

// 旧格式（纯数字累计计数）迁移为 {count,lastUsed}。
func TestUsageLegacyMigration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ResetForTest()
	dir := filepath.Join(home, ".local", "state", "logview")
	os.MkdirAll(dir, 0755)
	legacy := map[string]float64{"ssh:oldhost": 5}
	dataBytes, _ := json.Marshal(legacy)
	os.WriteFile(filepath.Join(dir, "usage.json"), dataBytes, 0644)

	mu.Lock()
	m := loadLocked()
	mu.Unlock()
	e, ok := m["ssh:oldhost"]
	if !ok || e.Count != 5 {
		t.Fatalf("旧格式应迁移 count=5: %+v", e)
	}
	if e.LastUsed == 0 {
		t.Fatal("迁移应补 LastUsed")
	}
}

// RecentSourceKeys 只返回源类 key（排除高亮/隐藏词条），衰减分降序、同分按 LastUsed 新→旧。
func TestRecentSourceKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ResetForTest()
	Bump(KeyFile + "/tmp/app.log")
	Bump(KeyFile + "/tmp/app.log")
	Bump(KeyFile + "/tmp/error.log")
	Bump(KeySSHHost + "web1")
	Bump(KeyHighlight + "timeout") // 词条不应出现在源列表
	Bump(KeyHide + "health")       // 同上

	got := RecentSourceKeys(10)
	if len(got) != 3 {
		t.Fatalf("应只返回 3 个源 key（排除词条），实际 %d: %v", len(got), got)
	}
	if got[0] != KeyFile+"/tmp/app.log" {
		t.Fatalf("两次使用的 app.log 应排第一: %v", got)
	}
	if got[1] != KeyFile+"/tmp/error.log" || got[2] != KeySSHHost+"web1" {
		t.Fatalf("同分项应按 LastUsed 新→旧: %v", got)
	}

	// limit 截断
	if len(RecentSourceKeys(1)) != 1 {
		t.Fatal("limit=1 应只返回 1 条")
	}
}
