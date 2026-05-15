package tui

import (
	"fmt"
	"strings"

	"github.com/justfun/logview/internal/model"
)

func (a *App) renderPanel() string {
	switch a.activePanel {
	case 0:
		return a.renderFieldsPanel()
	case 1:
		return a.renderLevelsPanel()
	case 2:
		return a.renderFilterPanel()
	}
	return ""
}

func (a *App) renderFieldsPanel() string {
	var items []string
	for _, f := range model.AllFields {
		cb := CheckboxUnchecked
		if a.fieldMask.IsVisible(f) {
			cb = CheckboxChecked
		}
		items = append(items, fmt.Sprintf("%s %s", cb, f))
	}
	return PanelStyle.Render("字段: " + strings.Join(items, "  "))
}

func (a *App) renderLevelsPanel() string {
	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	var items []string
	for _, l := range levels {
		cb := CheckboxUnchecked
		if a.levelMask[l] {
			cb = CheckboxChecked
		}
		items = append(items, fmt.Sprintf("%s %s", cb, LevelStyle(l).Render(l)))
	}
	return PanelStyle.Render("级别: " + strings.Join(items, "  "))
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
	return PanelStyle.Render(fmt.Sprintf("筛选: traceId=%s  thread=%s", td, thd))
}