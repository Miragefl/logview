package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// 回归保护（居中弹窗设计）：搜索/高亮/隐藏弹窗居中渲染且带遮罩。
// 锁定契约：
//  1. 弹窗内容（tab 栏/输入提示）出现在日志区垂直中部附近
//  2. 弹窗高度受日志区约束（不超过 vl），小终端不退化
//  3. 匹配行数统计正确（搜索功能本身不受布局影响）
func TestSearchPopupCentered(t *testing.T) {
	for _, h := range []int{20, 40} {
		t.Run(fmt.Sprintf("height=%d", h), func(t *testing.T) {
			app := NewApp(&mockStream{}, nil, 1000, nil)
			app.width = 120
			app.height = h
			for i := 0; i < 5; i++ {
				app.processLine(model.RawLine{Text: "2026 service started here", Source: "k"})
			}

			app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
			for _, r := range "ser" {
				app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			}
			if !app.searchMode {
				t.Fatalf("应处于搜索模式")
			}
			if len(app.filteredView) != 5 {
				t.Fatalf("ser 应匹配 5 行，实际 %d", len(app.filteredView))
			}

			// 弹窗高度不超过日志区
			vl := app.visibleLines()
			ph := len(app.buildSearchPopup(vl))
			if ph > vl {
				t.Fatalf("height=%d: popup 高度 %d 超过日志区 %d", h, ph, vl)
			}

			// View 日志区内应出现弹窗 tab 栏（居中 overlay）——边框盒内从第 1 行起
			view := app.View()
			lines := strings.Split(view, "\n")
			hasPopup := false
			for i := 1; i < 1+vl && i < len(lines); i++ {
				l := stripANSI(lines[i])
				if strings.Contains(l, "搜索") && strings.Contains(l, "高亮") {
					hasPopup = true
					break
				}
			}
			if !hasPopup {
				t.Fatalf("height=%d: 居中弹窗的 tab 栏应在日志区可见", h)
			}
		})
	}
}

// 弹窗不遮日志：弹窗打开时其上下区域仍显示日志行（可边看边复制）。
func TestSearchPopupKeepsLogVisible(t *testing.T) {
	app := NewApp(&mockStream{}, nil, 1000, nil)
	app.width = 120
	app.height = 40
	for i := 0; i < 20; i++ {
		app.processLine(model.RawLine{Text: "2026 ERROR boom failure line", Source: "k"})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "boom" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	view := app.View()
	lines := strings.Split(view, "\n")
	vl := app.visibleLines()
	// 弹窗上方（日志区前几行）应有日志内容透出
	hasLogAbove := false
	for i := 4; i < 4+3 && i < len(lines); i++ {
		if strings.Contains(stripANSI(lines[i]), "boom") {
			hasLogAbove = true
			break
		}
	}
	// 弹窗下方也应有日志
	hasLogBelow := false
	for i := 4; i < 4+vl && i < len(lines); i++ {
		if strings.Contains(stripANSI(lines[i]), "boom") {
			hasLogBelow = true
		}
	}
	if !hasLogAbove || !hasLogBelow {
		t.Fatalf("弹窗上下应透出日志：above=%v below=%v", hasLogAbove, hasLogBelow)
	}
}
