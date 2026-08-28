package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62"))

	LevelDebug = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	LevelInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("61"))
	LevelWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	LevelError = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	TimeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	SourceStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
	TraceIDStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("123"))
	ThreadStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))

	HighlightStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("227")).
			Foreground(lipgloss.Color("0"))

	SelectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252"))

	SelArrowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	KeyCapStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("81")).
			Bold(true)

	FrameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))

	TitleBarStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("114"))

	BreadcrumbStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117"))

	VisualStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("28")).
			Foreground(lipgloss.Color("15"))

	HelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	HelpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))

	CheckboxChecked   = "☑"
	CheckboxUnchecked = "☐"

	FoldedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
	NewLogStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	DetailLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	DetailValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	DetailDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	PopupBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("73")).
			Padding(0, 1)

	PopupActiveTabStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("81"))

	PopupTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	HorizontalLine = "─"

	SourceColors = []lipgloss.Color{
		lipgloss.Color("183"), // purple (default)
		lipgloss.Color("117"), // cyan
		lipgloss.Color("221"), // yellow
		lipgloss.Color("156"), // green
		lipgloss.Color("215"), // orange
		lipgloss.Color("217"), // pink
	}

	HighlightColors = []lipgloss.Color{
		lipgloss.Color("227"), // yellow
		lipgloss.Color("123"), // cyan
		lipgloss.Color("201"), // magenta
		lipgloss.Color("82"),  // green
		lipgloss.Color("214"), // orange
		lipgloss.Color("69"),  // blue
		lipgloss.Color("183"), // purple
		lipgloss.Color("196"), // red
	}
)

var VisualBgColor lipgloss.Color = lipgloss.Color("28")
var VisualFgColor lipgloss.Color = lipgloss.Color("15")

var SelectedBgColor lipgloss.Color = lipgloss.Color("62")
var SelectedFgColor lipgloss.Color = lipgloss.Color("15")

var HideMarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
var BookmarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

var AppBgColor lipgloss.Color = ""
var AppFgColor lipgloss.Color = ""
var AppBgSeq string = ""

func hexToRGB(hex string) (r, g, b int) {
	if len(hex) == 7 && hex[0] == '#' {
		fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	}
	return
}

func SetTerminalBg(hex string) string {
	if hex == "" {
		return ""
	}
	r, g, b := hexToRGB(hex)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

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

// padLevel 级别徽章定宽对齐（最长 ERROR/DEBUG=5 字符，短的右侧补空格），
// 使 message 列起点稳定；超长级别（如 WARNING）保留原长不截断。
func padLevel(v string) string {
	for len(v) < 5 {
		v += " "
	}
	return v
}
