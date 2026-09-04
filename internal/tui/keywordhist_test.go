package tui

import (
	"fmt"
	"reflect"
	"testing"
)

// resetUsage 清空 usage 全局缓存(隔离跨测试污染,惯例同 frppicker_test)。
// HOME 切到临时目录:BumpUsage 会整map写盘,若写全局 usage.json 会抹掉
// ssh:lane 等跨运行累计频次,使 TestSSHCandidatesHotSorted 丢 ★ 标记(flake)。
func resetUsage(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	usageMu.Lock()
	usageData = map[string]usageEntry{}
	usageDirty = false
	usageMu.Unlock()
	t.Cleanup(func() {
		usageMu.Lock()
		usageData = nil
		usageMu.Unlock()
	})
}

// scopedMock 带自定义 scope 的测试流(复用 mockStream 的其余方法)。
type scopedMock struct {
	mockStream
	scope string
}

func (m *scopedMock) Scope() string { return m.scope }

// NewApp 取初始源 scope;ReplaceStream 换流后刷新。
func TestCurrentScopeTracksStream(t *testing.T) {
	app := newTestApp()
	if app.currentScope != "" {
		t.Fatalf("mockStream(全局)初始 scope 应为空, got %q", app.currentScope)
	}
	app.ReplaceStream(&scopedMock{scope: "frp1"})
	if app.currentScope != "frp1" {
		t.Fatalf("ReplaceStream 后 scope 应刷新为 frp1, got %q", app.currentScope)
	}
}

// sortedUsageWords 按域捞词:频次降序、同分 LastUsed 新→旧、再同词序升序、截 limit。
func TestSortedUsageWords(t *testing.T) {
	resetUsage(t)
	BumpUsage("hl::err")
	BumpUsage("hl::err")
	BumpUsage("hl::timeout")
	BumpUsage("hl:frp1:secret") // 其他 scope 不混入
	BumpUsage("hide::err")      // 其他 kind 不混入
	got := sortedUsageWords(usageHighlight, "", 20)
	if !reflect.DeepEqual(got, []string{"err", "timeout"}) {
		t.Fatalf("频次降序 = %v, want [err timeout]", got)
	}
}

// 同分时 LastUsed 新的在前(直写 entry 伪造时间,衰减后同分)。
func TestSortedUsageWordsTieLastUsed(t *testing.T) {
	resetUsage(t)
	usageMu.Lock()
	loadUsage()["hl::a"] = usageEntry{Count: 1, LastUsed: 100}
	loadUsage()["hl::b"] = usageEntry{Count: 1, LastUsed: 200}
	usageMu.Unlock()
	got := sortedUsageWords(usageHighlight, "", 20)
	if !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Fatalf("同分 LastUsed 新在前 = %v, want [b a]", got)
	}
}

// limit 截断:25 个同分词按词序取前 20。
func TestSortedUsageWordsLimit(t *testing.T) {
	resetUsage(t)
	for i := 0; i < 25; i++ {
		BumpUsage(fmt.Sprintf("hide::w%02d", i))
	}
	got := sortedUsageWords(usageHide, "", 20)
	if len(got) != 20 || got[0] != "w00" || got[19] != "w19" {
		t.Fatalf("截断/词序错误: len=%d first=%q last=%q", len(got), got[0], got[19])
	}
}

// 确认多词输入逐词计频(全局域,mockStream scope 为空)。
// 注:usageScore 返回衰减分(半衰期 7 天,LastUsed 截断到 Unix 秒),
// bump 后即时读取恒 < 1(亚秒级幻影衰减),故用容差带而非精确等值。
func TestConfirmHighlightsBumpPerWord(t *testing.T) {
	resetUsage(t)
	app := newTestApp()
	app.searchTab = 1
	app.highlightInput = "err,fail"
	app.confirmHighlights()
	for _, k := range []string{"hl::err", "hl::fail"} {
		if s := usageScore(k); s < 0.99 || s > 1.01 {
			t.Fatalf("key %s 应计 1 次, got %v", k, s)
		}
	}
	if usageScore("hl::timeout") != 0 {
		t.Fatal("未确认的词不应计数")
	}
}

// 隐藏同构 + 空输入不计数。
func TestConfirmHidesBumpAndEmpty(t *testing.T) {
	resetUsage(t)
	app := newTestApp()
	app.searchTab = 2
	app.hideInput = "health,metrics"
	app.confirmHides()
	for _, k := range []string{"hide::health", "hide::metrics"} {
		if s := usageScore(k); s < 0.99 || s > 1.01 {
			t.Fatalf("key %s 应计 1 次, got %v", k, s)
		}
	}
	app.hideInput = ""
	app.confirmHides() // 清空:不计数
	if usageScore("hide::") != 0 {
		t.Fatal("空输入不应产生计数")
	}
}

// scope 隔离计频:切到 frp1 后确认,词计入 frp1 域而非全局。
func TestConfirmScopedBump(t *testing.T) {
	resetUsage(t)
	app := newTestApp()
	app.ReplaceStream(&scopedMock{scope: "frp1"})
	app.searchTab = 1
	app.highlightInput = "pay"
	app.confirmHighlights()
	if s := usageScore("hl:frp1:pay"); s < 0.99 || s > 1.01 {
		t.Fatalf("应计入 frp1 域, got %v", s)
	}
	if usageScore("hl::pay") != 0 {
		t.Fatal("不应污染全局域")
	}
}
