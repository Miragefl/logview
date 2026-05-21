package tui

import (
	"github.com/charmbracelet/lipgloss"
)

type ThemeConfig struct {
	TitleFG     string
	TitleBG     string
	LevelDebug  string
	LevelInfo   string
	LevelWarn   string
	LevelError  string
	Time        string
	Source      string
	TraceID     string
	Thread      string
	ErrorLineBg string
	WarnLineBg  string
	Highlight   string
	Selected    string
	Visual      string
	PopupBorder string
	PopupBg     string
	Dim         string
	Accent      string
	Bg          string
	Fg          string
}

var DarkTheme = ThemeConfig{
	TitleFG:     "#FFFFFF",
	TitleBG:     "#5F5FAF",
	LevelDebug:  "#767676",
	LevelInfo:   "#5FD7AF",
	LevelWarn:   "#FFAF00",
	LevelError:  "#FF005F",
	Time:        "#767676",
	Source:      "#D7AFFF",
	TraceID:     "#87FFFF",
	Thread:      "#9E9E9E",
	ErrorLineBg: "#5F0000",
	WarnLineBg:  "#5F5F00",
	Highlight:   "#FFFF00",
	Selected:    "#5F5FAF",
	Visual:      "#008700",
	PopupBorder: "#5FD7AF",
	PopupBg:     "#262626",
	Dim:         "#767676",
	Accent:      "#5FD7AF",
	Bg:          "",
	Fg:          "",
}

var LightTheme = ThemeConfig{
	TitleFG:     "#FFFFFF",
	TitleBG:     "#005FAF",
	LevelDebug:  "#9E9E9E",
	LevelInfo:   "#008700",
	LevelWarn:   "#AF5F00",
	LevelError:  "#AF0000",
	Time:        "#9E9E9E",
	Source:      "#AF00AF",
	TraceID:     "#0087AF",
	Thread:      "#6C6C6C",
	ErrorLineBg: "#FFD7D7",
	WarnLineBg:  "#FFFFD7",
	Highlight:   "#FFFF00",
	Selected:    "#005FAF",
	Visual:      "#005F00",
	PopupBorder: "#005FAF",
	PopupBg:     "#E4E4E4",
	Dim:         "#9E9E9E",
	Accent:      "#008700",
	Bg:          "#FFFFFF",
	Fg:          "#333333",
}

func ApplyTheme(cfg ThemeConfig) {
	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(cfg.TitleFG)).
		Background(lipgloss.Color(cfg.TitleBG))

	LevelDebug = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LevelDebug))
	LevelInfo = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LevelInfo))
	LevelWarn = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LevelWarn))
	LevelError = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LevelError)).Bold(true)

	TimeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Time))
	SourceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Source))
	TraceIDStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.TraceID))
	ThreadStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Thread))

	ErrorLineBg = lipgloss.Color(cfg.ErrorLineBg)
	WarnLineBg = lipgloss.Color(cfg.WarnLineBg)

	HighlightStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(cfg.Highlight)).
		Foreground(lipgloss.Color("#000000"))

	SelectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(cfg.Selected)).
		Foreground(lipgloss.Color("#FFFFFF"))
	SelectedBgColor = lipgloss.Color(cfg.Selected)
	SelectedFgColor = lipgloss.Color("#FFFFFF")
	VisualBgColor = lipgloss.Color(cfg.Visual)
	VisualFgColor = lipgloss.Color("#FFFFFF")

	VisualStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(cfg.Visual)).
		Foreground(lipgloss.Color("#FFFFFF"))

	HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Dim))
	HelpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.Accent))

	FoldedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Dim)).Italic(true)
	NewLogStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LevelWarn)).Bold(true)

	DetailLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.Accent))
	DetailDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Dim))

	PopupBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(cfg.PopupBorder)).
		Padding(0, 2).
		Background(lipgloss.Color(cfg.PopupBg))

	PopupTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Dim))
	HideMarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LevelError)).Bold(true)

	HighlightColors = []lipgloss.Color{
		lipgloss.Color(cfg.Highlight),
		lipgloss.Color(cfg.TraceID),
		lipgloss.Color("#FF87FF"),
		lipgloss.Color("#5FD75F"),
		lipgloss.Color(cfg.LevelWarn),
		lipgloss.Color("#5F87D7"),
		lipgloss.Color(cfg.Source),
		lipgloss.Color(cfg.LevelError),
	}

	AppBgColor = lipgloss.Color(cfg.Bg)
	AppFgColor = lipgloss.Color(cfg.Fg)
	AppBgSeq = SetTerminalBg(cfg.Bg)

	if cfg.Bg != "" {
		bg := lipgloss.Color(cfg.Bg)
		LevelDebug = LevelDebug.Background(bg)
		LevelInfo = LevelInfo.Background(bg)
		LevelWarn = LevelWarn.Background(bg)
		LevelError = LevelError.Background(bg)
		TimeStyle = TimeStyle.Background(bg)
		SourceStyle = SourceStyle.Background(bg)
		TraceIDStyle = TraceIDStyle.Background(bg)
		ThreadStyle = ThreadStyle.Background(bg)
		HelpStyle = HelpStyle.Background(bg)
		HelpKeyStyle = HelpKeyStyle.Background(bg)
		FoldedStyle = FoldedStyle.Background(bg)
		NewLogStyle = NewLogStyle.Background(bg)
		DetailLabelStyle = DetailLabelStyle.Background(bg)
		DetailValueStyle = lipgloss.NewStyle().Foreground(AppFgColor).Background(bg)
		DetailDimStyle = DetailDimStyle.Background(bg)
		PopupTabStyle = PopupTabStyle.Background(bg)
		HideMarkStyle = HideMarkStyle.Background(bg)
	}
}

func ResolveTheme(name string, overrides map[string]string) ThemeConfig {
	base := DarkTheme
	if name == "light" {
		base = LightTheme
	}
	if len(overrides) == 0 {
		return base
	}
	if v, ok := overrides["title.fg"]; ok {
		base.TitleFG = v
	}
	if v, ok := overrides["title.bg"]; ok {
		base.TitleBG = v
	}
	if v, ok := overrides["level.debug"]; ok {
		base.LevelDebug = v
	}
	if v, ok := overrides["level.info"]; ok {
		base.LevelInfo = v
	}
	if v, ok := overrides["level.warn"]; ok {
		base.LevelWarn = v
	}
	if v, ok := overrides["level.error"]; ok {
		base.LevelError = v
	}
	if v, ok := overrides["time"]; ok {
		base.Time = v
	}
	if v, ok := overrides["source"]; ok {
		base.Source = v
	}
	if v, ok := overrides["traceId"]; ok {
		base.TraceID = v
	}
	if v, ok := overrides["thread"]; ok {
		base.Thread = v
	}
	if v, ok := overrides["error_line_bg"]; ok {
		base.ErrorLineBg = v
	}
	if v, ok := overrides["warn_line_bg"]; ok {
		base.WarnLineBg = v
	}
	if v, ok := overrides["highlight"]; ok {
		base.Highlight = v
	}
	if v, ok := overrides["selected"]; ok {
		base.Selected = v
	}
	if v, ok := overrides["visual"]; ok {
		base.Visual = v
	}
	if v, ok := overrides["popup.border"]; ok {
		base.PopupBorder = v
	}
	if v, ok := overrides["popup.bg"]; ok {
		base.PopupBg = v
	}
	if v, ok := overrides["dim"]; ok {
		base.Dim = v
	}
	if v, ok := overrides["accent"]; ok {
		base.Accent = v
	}
	if v, ok := overrides["bg"]; ok {
		base.Bg = v
	}
	if v, ok := overrides["fg"]; ok {
		base.Fg = v
	}
	return base
}
