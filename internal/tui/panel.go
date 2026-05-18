package tui

import (
	"fmt"
	"strings"

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
			item = SelectedStyle.Render(item)
		}
		items = append(items, item)
	}
	return strings.Join(items, "\n")
}
