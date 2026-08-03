package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// 回归保护 #1：搜索 popup 曾从日志区顶部逐行 overlay，当 popup 行数 >= 匹配结果
// 行数时把日志全盖成空白（用户看到 popup 下面一片空）。改为 inline 布局后 popup
// 紧贴搜索栏、日志下方独立显示。此测试锁死"搜索时日志区必须能看见匹配行"。
func TestSearchPopupDoesNotBlankLog(t *testing.T) {
	app := NewApp(&mockStream{}, nil, 1000, nil)
	app.width = 120
	app.height = 40
	// 只喂 5 行 ERROR：匹配结果(5) < popup 高度，最易触发空白
	for i := 0; i < 5; i++ {
		app.processLine(model.RawLine{Text: "2026-05-15 09:27:01.130 [t] [abc] ERROR App - boom failure", Source: "p"})
	}

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 进搜索
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if !app.searchMode {
		t.Fatalf("应处于搜索模式")
	}
	if len(app.filteredView) != 5 {
		t.Fatalf("ERROR 应匹配 5 行，实际 %d", len(app.filteredView))
	}

	// View() 日志区（跳过 title/sep/bar/sep 共 4 行）必须至少一行含原始日志内容
	view := app.View()
	lines := strings.Split(view, "\n")
	vl := app.visibleLines()
	hasLog := false
	for i := 4; i < 4+vl && i < len(lines); i++ {
		if strings.Contains(stripAnsi(lines[i]), "boom") {
			hasLog = true
			break
		}
	}
	if !hasLog {
		t.Fatalf("搜索 popup 不应把日志盖成空白：日志区 %d 行找不到匹配日志", vl)
	}
}

// 回归保护 #2：搜 "service" 输入到 "ser" 时匹配骤减（"se" 匹配多、含 "ser" 的少），
// v0.12.15 的居中 overlay 在小终端（popup 行数 >= 可视行数）退化为顶部覆盖，把仅剩
// 的 service 行全盖，必须输完回车关 popup 才看到。inline 布局后无论终端大小，
// 匹配行始终在 popup 下方可见。覆盖正常终端(40)和小终端(20)。
func TestSearchPopupKeepsMatchesVisibleWhenFew(t *testing.T) {
	for _, h := range []int{20, 40} {
		t.Run(fmt.Sprintf("height=%d", h), func(t *testing.T) {
			app := NewApp(&mockStream{}, nil, 1000, nil)
			app.width = 120
			app.height = h
			// 12 行含 "se" 不含 "ser"（response）+ 3 行含 "ser"（service）
			for i := 0; i < 12; i++ {
				app.processLine(model.RawLine{Text: "2026 request response upstream", Source: "k"})
			}
			for i := 0; i < 3; i++ {
				app.processLine(model.RawLine{Text: "2026 service started here", Source: "k"})
			}

			app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
			for _, r := range "ser" {
				app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			}
			if !app.searchMode {
				t.Fatalf("应处于搜索模式")
			}
			// ser 阶段只剩 service 行匹配
			if len(app.filteredView) != 3 {
				t.Fatalf("ser 应只匹配 3 行 service，实际 %d", len(app.filteredView))
			}

			// 与 View() 相同的 popup 高度算法
			vl := app.visibleLines()
			logReserve := vl / 3
			if logReserve < 3 {
				logReserve = 3
			}
			popupMaxH := vl - logReserve
			if popupMaxH < 1 {
				popupMaxH = 1
			}
			ph := len(app.buildSearchPopup(popupMaxH))

			// popup 之后的日志区必须有 service 行可见
			view := app.View()
			lines := strings.Split(view, "\n")
			hasVisible := false
			for i := 4 + ph; i < 4+vl && i < len(lines); i++ {
				if strings.Contains(stripAnsi(lines[i]), "service") {
					hasVisible = true
					break
				}
			}
			if !hasVisible {
				t.Fatalf("height=%d: ser 阶段 service 行应在 popup 下方可见，ph=%d vl=%d", h, ph, vl)
			}
		})
	}
}
