package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// timePreset C-t 时间快捷片：高频时间条件一键插入（追加而非替换，AND 组合流不断）。
// 右列渲染规范语法——UI 即教程，用几次后自然记住 time: 写法。
type timePreset struct {
	label string
	expr  func(now time.Time) string
}

var timePresets = []timePreset{
	{label: "最近5分钟", expr: func(time.Time) string { return "time:>-5m" }},
	{label: "最近15分钟", expr: func(time.Time) string { return "time:>-15m" }},
	{label: "最近1小时", expr: func(time.Time) string { return "time:>-1h" }},
	{label: "最近6小时", expr: func(time.Time) string { return "time:>-6h" }},
	{label: "今天", expr: func(now time.Time) string {
		d := now.UTC().Format("2006-01-02")
		next := now.UTC().AddDate(0, 0, 1).Format("2006-01-02")
		return fmt.Sprintf("time:%sT00:00..%sT00:00", d, next)
	}},
	{label: "昨天", expr: func(now time.Time) string {
		d := now.UTC().AddDate(0, 0, -1).Format("2006-01-02")
		next := now.UTC().Format("2006-01-02")
		return fmt.Sprintf("time:%sT00:00..%sT00:00", d, next)
	}},
	{label: "自定义…", expr: func(now time.Time) string {
		return "time:>" + now.UTC().Format("2006-01-02T15:04")
	}},
}

// handleTimePresetKeys 快捷片按键（与 C-r 历史列表同构：导航/插入/关闭/续输）。
func (a *App) handleTimePresetKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(timePresets)
	switch msg.Type {
	case tea.KeyEscape:
		a.timePresetMode = false
	case tea.KeyEnter:
		a.insertTimePreset(a.timePresetCursor)
	case tea.KeyUp:
		if a.timePresetCursor > 0 {
			a.timePresetCursor--
		}
	case tea.KeyDown:
		if a.timePresetCursor < n-1 {
			a.timePresetCursor++
		}
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "k":
			if a.timePresetCursor > 0 {
				a.timePresetCursor--
			}
		case "j":
			if a.timePresetCursor < n-1 {
				a.timePresetCursor++
			}
		default:
			a.timePresetMode = false
			return a.handleSearchKeys(msg)
		}
	case tea.KeyCtrlT:
		// 列表内再按 C-t：无操作（避免误关）
	default:
		a.timePresetMode = false
		return a.handleSearchKeys(msg)
	}
	return a, nil
}

// insertTimePreset 在光标处追加插入所选 time: 表达式（前导空格分隔，光标移到末尾）。
func (a *App) insertTimePreset(idx int) {
	a.timePresetMode = false
	if idx < 0 || idx >= len(timePresets) {
		return
	}
	expr := timePresets[idx].expr(time.Now())
	input := a.activeSearchInput()
	if *input.text != "" && !strings.HasSuffix(*input.text, " ") {
		expr = " " + expr
	}
	input.insert(expr)
	if a.searchTab == 0 {
		a.recomputeView()
	}
}

// renderTimePresetList 渲染快捷片列表（左功能名，右规范语法）。
func (a *App) renderTimePresetList(content *strings.Builder) {
	content.WriteString(PopupActiveTabStyle.Render(" 时间范围 ") + "\n\n")
	now := time.Now()
	for i, p := range timePresets {
		prefix := "  "
		labelStyle := DetailValueStyle
		if i == a.timePresetCursor {
			prefix = SelArrowStyle.Render("▶ ")
			labelStyle = SelArrowStyle
		}
		content.WriteString(prefix + " " + labelStyle.Render(p.label) +
			strings.Repeat(" ", max(2, 16-displayWidth(p.label))) +
			PopupTabStyle.Render(p.expr(now)) + "\n")
	}
}
