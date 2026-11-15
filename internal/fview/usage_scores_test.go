// UsageScores 批量分数接口测试(panel-hot-sort):HOME 隔离 + ResetForTest
// 后 Touch 若干 key,断言分数单调、缺失 key 不出现在结果、惰性衰减生效。
package fview

import (
	"testing"

	"github.com/Miragefl/logview/internal/usage"
)

func TestUsageScores(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	usage.ResetForTest()

	UsageTouch("ssh:alpha")
	UsageTouch("ssh:alpha")
	UsageTouch("ssh:beta")

	scores := UsageScores([]string{"ssh:alpha", "ssh:beta", "ssh:none"})
	if len(scores) != 2 {
		t.Fatalf("缺失 key 不应出现, got %v", scores)
	}
	if scores["ssh:alpha"] <= scores["ssh:beta"] {
		t.Fatalf("两次 Touch 应高于一次: alpha=%v beta=%v", scores["ssh:alpha"], scores["ssh:beta"])
	}
	if scores["ssh:beta"] <= 0 {
		t.Fatalf("刚 Touch 的 key 分数应为正, got %v", scores["ssh:beta"])
	}

	// 惰性衰减:Bump 落盘 LastUsed 截整秒,读取带亚秒 → 刚用的分略 < Touch 次数;
	// 再 Touch 后 alpha 分应继续增长(≥ 原值)
	before := scores["ssh:alpha"]
	UsageTouch("ssh:alpha")
	after := UsageScores([]string{"ssh:alpha"})["ssh:alpha"]
	if after < before {
		t.Fatalf("再 Touch 分数不得回退: before=%v after=%v", before, after)
	}
}
