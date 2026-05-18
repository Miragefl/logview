package tui

import (
	"strings"
)

// buildSearchPopup returns the popup box lines (only the box itself, no full-area fill).
func (a *App) buildSearchPopup() []string {
	if len(a.starFields) == 0 {
		return nil
	}

	nRows := len(a.starFields)
	if nRows > 8 {
		nRows = 8
	}
	var content strings.Builder
	for i := 0; i < nRows; i++ {
		sf := a.starFields[i]
		prefix := "  "
		if i == a.starCursor {
			prefix = SelectedStyle.Render(" >")
		}
		if sf.Name == "" {
			content.WriteString(prefix + " " + HelpKeyStyle.Render("确认搜索") + "\n")
		} else {
			name := DetailLabelStyle.Render(sf.Name + ":")
			val := DetailValueStyle.Render(sf.Value)
			content.WriteString(prefix + " " + name + " " + val + "\n")
		}
	}
	hint := PopupTabStyle.Render(" Tab/S-Tab C-j/C-k Enter Esc")
	content.WriteString(hint)

	boxW := min(60, a.width-4)
	box := PopupBoxStyle.Width(boxW).Render(content.String())
	return strings.Split(box, "\n")
}
