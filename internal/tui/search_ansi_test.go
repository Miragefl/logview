package tui

import (
	"testing"

	"github.com/Miragefl/logview/internal/model"
)

// TestSearchIgnoresANSIColorCodes 搜索命中判定须忽略行内 ANSI 颜色码:
// 输入 "31" 不得命中 "\x1b[31m"(日志自身颜色)——显示文本无 31 即不命中。
func TestSearchIgnoresANSIColorCodes(t *testing.T) {
	app := newTestApp()
	app.processLine(model.RawLine{
		Text:   "\x1b[31m2026-05-15 09:27:01.130 [t] ERROR red\x1b[0m",
		Source: "test",
	})
	app.searchInput = "31"
	app.updateSearchStats()
	if app.searchMatchCount != 0 {
		t.Fatalf("搜索 31 命中 %d 行,应 0(ANSI 颜色码不得参与匹配)", app.searchMatchCount)
	}
}
