package tui

import (
	"fmt"
	"strings"
)

func (a *App) renderHelpBarContent() string {
	var parts []string
	for _, b := range a.keymap.HelpEntries() {
		parts = append(parts, fmt.Sprintf("%s%s", HelpKeyStyle.Render(b.Help().Key), HelpStyle.Render(b.Help().Desc)))
	}
	return strings.Join(parts, "  ")
}
