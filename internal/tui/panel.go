package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/model"
)

func (a *App) renderFieldsPanel() string {
	var items []string
	for i, f := range model.AllFields {
		cb := CheckboxUnchecked
		if a.fieldMask.IsVisible(f) {
			cb = CheckboxChecked
		}
		item := fmt.Sprintf("%s %s", cb, f)
		if a.panelFocus && i == a.fieldCursor {
			item = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("15")).
				Render(item)
		}
		items = append(items, item)
	}
	return strings.Join(items, "\n")
}
