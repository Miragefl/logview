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

	SelectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("15"))

	HelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	HelpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))

	CheckboxChecked   = "☑"
	CheckboxUnchecked = "☐"

	FieldSeparator = " │ "

	FoldedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
	NewLogStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	DetailLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	DetailValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	DetailDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	PopupBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(0, 2).
			Background(lipgloss.Color("235"))

	PopupActiveTabStyle = lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("15"))

	PopupTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	HorizontalLine = "─"
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
