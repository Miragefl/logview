package tui

import (
	"fmt"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
// 直写 entry 钉死相同 LastUsed(手法同 TestSortedUsageWordsTieLastUsed):
// 真实 BumpUsage 逐词写盘,循环跨秒边界时各词 LastUsed 出现秒差,
// 衰减分不再全同,词序断言会跨秒 flake;钉死后断言与真实时钟解耦。
func TestSortedUsageWordsLimit(t *testing.T) {
	resetUsage(t)
	usageMu.Lock()
	for i := 0; i < 25; i++ {
		loadUsage()[fmt.Sprintf("hide::w%02d", i)] = usageEntry{Count: 1, LastUsed: 1000}
	}
	usageMu.Unlock()
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

// C-r 高亮历史:频次降序快照,首行高频词,Enter 填入首行。
func TestKeywordHistFreqOrderAndFill(t *testing.T) {
	resetUsage(t)
	BumpUsage("hl::err")
	BumpUsage("hl::err")
	BumpUsage("hl::timeout")
	app := newTestApp()
	app.searchTab = 1
	app.searchMode = true // ctrl+r 仅在搜索弹窗内处理
	app.Update(fakeKey("ctrl+r"))
	if !app.searchHistMode {
		t.Fatal("C-r 应打开高亮历史列表")
	}
	if got := app.histRowAt(app.keywordHist, 0); got != "err" {
		t.Fatalf("首行应为高频词 err, got %q", got)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.highlightInput != "err" {
		t.Fatalf("Enter 应填入首行 err, got %q", app.highlightInput)
	}
}

// scope 隔离:frp2 的列表只含 frp2 域词条。
func TestKeywordHistScopeIsolation(t *testing.T) {
	resetUsage(t)
	BumpUsage("hl:frp1:pay")
	BumpUsage("hl:frp2:gw")
	app := newTestApp()
	app.currentScope = "frp2"
	app.searchTab = 1
	app.searchMode = true
	app.Update(fakeKey("ctrl+r"))
	if len(app.keywordHist) != 1 || app.keywordHist[0] != "gw" {
		t.Fatalf("frp2 列表应仅 [gw], got %v", app.keywordHist)
	}
}

// 跨会话:丢内存缓存(模拟重启,从盘重读)后列表仍在。
func TestKeywordHistPersistAcrossRestart(t *testing.T) {
	resetUsage(t)
	BumpUsage("hl::err")
	usageMu.Lock()
	usageData = nil
	usageMu.Unlock()
	app := newTestApp()
	app.searchTab = 1
	app.searchMode = true
	app.Update(fakeKey("ctrl+r"))
	if len(app.keywordHist) != 1 || app.keywordHist[0] != "err" {
		t.Fatalf("重启后列表应仍含 err, got %v", app.keywordHist)
	}
}

// 搜索 tab 零改动:时序倒序(最新在首行)。
func TestSearchHistUnchanged(t *testing.T) {
	app := newTestApp()
	app.searchHistory = []string{"a", "b"} // append 序,b 最新
	app.searchTab = 0
	app.searchMode = true
	app.Update(fakeKey("ctrl+r"))
	if got := app.histRowAt(app.currentTabHistory(), 0); got != "b" {
		t.Fatalf("搜索首行应为最新 b, got %q", got)
	}
}

// 高亮/隐藏历史选词追加进输入框(逗号拼接、已含则不重复);搜索 tab 仍整框替换。
func TestApplySearchHistoryAppend(t *testing.T) {
	resetUsage(t)
	app := newTestApp()

	// 追加:框已有内容,选中新词拼接
	app.searchTab = 1
	app.highlightInput = "error,timeout"
	app.highlightCursor = len([]rune("error,timeout"))
	app.applySearchHistory("fail")
	if app.highlightInput != "error,timeout,fail" {
		t.Fatalf("追加后 = %q, want error,timeout,fail", app.highlightInput)
	}
	if app.highlightCursor != len([]rune("error,timeout,fail")) {
		t.Fatalf("光标应在末尾, got %d", app.highlightCursor)
	}

	// 去重:已含的词不重复追加,框保持原样
	app.applySearchHistory("error")
	if app.highlightInput != "error,timeout,fail" {
		t.Fatalf("重复词不应追加, got %q", app.highlightInput)
	}

	// 空框:直接填入
	app.searchTab = 2
	app.hideInput = ""
	app.applySearchHistory("health")
	if app.hideInput != "health" || app.hideCursor != len([]rune("health")) {
		t.Fatalf("空框应直接填入, got %q cursor=%d", app.hideInput, app.hideCursor)
	}

	// 搜索 tab:整框替换不变
	app.searchTab = 0
	app.searchInput = "old query"
	app.applySearchHistory("field:value")
	if app.searchInput != "field:value" {
		t.Fatalf("搜索 tab 应整框替换, got %q", app.searchInput)
	}
}

// 纯函数:拼接与去重。
func TestAppendKeywordToInput(t *testing.T) {
	cases := []struct{ existing, q, want string }{
		{"", "err", "err"},
		{"err", "fail", "err,fail"},
		{"err,fail", "err", "err,fail"}, // 已含不重复
		{"err,", "fail", "err,,fail"},  // 尾悬逗号原样拼接(splitKeywords 容忍)
	}
	for _, c := range cases {
		if got := appendKeywordToInput(c.existing, c.q); got != c.want {
			t.Errorf("appendKeywordToInput(%q,%q) = %q, want %q", c.existing, c.q, got, c.want)
		}
	}
}
