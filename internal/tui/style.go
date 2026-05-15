package tui

import "github.com/charmbracelet/lipgloss"

var (
	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62"))

	LevelDebug = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	LevelInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	LevelWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	LevelError = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	SearchStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62"))

	HighlightStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("227")).
		Foreground(lipgloss.Color("0"))

	PanelStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false)

	SelectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("15"))

	HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	HelpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))

	CheckboxChecked   = "☑"
	CheckboxUnchecked = "☐"

	FoldedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
	NewLogStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
)

func LevelStyle(level string) lipgloss.Style {
	switch level {
	case "DEBUG", "DBG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR", "ERR", "FATAL":
		return LevelError
	}
	return lipgloss.NewStyle()
}