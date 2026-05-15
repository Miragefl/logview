package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/model"
)

func (a *App) renderPanel() string {
	sep := strings.Repeat(HorizontalLine, a.width)
	switch a.activePanel {
	case 0:
		return sep + "\n" + a.renderFieldsPanel()
	case 1:
		return sep + "\n" + a.renderLevelsPanel()
	case 2:
		return sep + "\n" + a.renderFilterPanel()
	}
	return sep
}

func (a *App) renderFieldsPanel() string {
	var items []string
	for i, f := range model.AllFields {
		cb := CheckboxUnchecked
		if a.fieldMask.IsVisible(f) {
			cb = CheckboxChecked
		}
		item := fmt.Sprintf("%s %s", cb, f)
		// 高亮当前光标位置（面板聚焦时）
		if a.panelFocus && a.activePanel == 0 && i == a.fieldCursor {
			item = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("15")).
				Render(item)
		}
		items = append(items, item)
	}
	hint := "  [↑↓选择] [空格切换] [Esc退出]"
	if !a.panelFocus {
		hint = "  [f进入编辑]"
	}
	return "字段: " + strings.Join(items, "  ") + hint
}

func (a *App) renderLevelsPanel() string {
	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	var items []string
	for i, l := range levels {
		cb := CheckboxUnchecked
		if a.levelMask[l] {
			cb = CheckboxChecked
		}
		item := fmt.Sprintf("%s %s", cb, LevelStyle(l).Render(l))
		if a.panelFocus && a.activePanel == 1 && i == a.fieldCursor {
			item = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("15")).
				Render(item)
		}
		items = append(items, item)
	}
	hint := "  [↑↓选择] [空格切换] [Esc退出]"
	if !a.panelFocus {
		hint = "  [Tab切换面板]"
	}
	return "级别: " + strings.Join(items, "  ") + hint
}

func (a *App) renderFilterPanel() string {
	td := a.filterTraceID
	if td == "" {
		td = "(空)"
	}
	thd := a.filterThread
	if thd == "" {
		thd = "(空)"
	}
	hint := "  [Esc退出]"
	if a.filterTraceID != "" || a.filterThread != "" {
		hint = "  [Esc清除筛选]"
	}
	return fmt.Sprintf("筛选: traceId=%s  thread=%s%s", td, thd, hint)
}
