package tui

import (
	"fmt"

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
	LevelInfo:   "#6B7BB8",
	LevelWarn:   "#FFAF00",
	LevelError:  "#FF005F",
	Time:        "#767676",
	Source:      "#D7AFFF",
	TraceID:     "#87FFFF",
	Thread:      "#9E9E9E",
	Highlight:   "#FFFF00",
	Selected:    "#30354F",
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
	LevelInfo:   "#5F875F",
	LevelWarn:   "#AF5F00",
	LevelError:  "#AF0000",
	Time:        "#9E9E9E",
	Source:      "#AF00AF",
	TraceID:     "#0087AF",
	Thread:      "#6C6C6C",
	Highlight:   "#FFFF00",
	Selected:    "#D0D7E5",
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
	LevelWarn = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LevelWarn)).Bold(true)
	LevelError = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LevelError)).Bold(true)

	TimeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Time))
	SourceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Source))
	TraceIDStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.TraceID))
	ThreadStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Thread))

	HighlightStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(cfg.Highlight)).
		Foreground(lipgloss.Color("#000000"))

	selFg := "#FFFFFF"
	if cfg.IsLightTheme() {
		selFg = "#1B1D24"
	}
	SelectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(cfg.Selected)).
		Foreground(lipgloss.Color(selFg))
	SelectedBgColor = lipgloss.Color(cfg.Selected)
	SelectedFgColor = lipgloss.Color(selFg)
	VisualBgColor = lipgloss.Color(cfg.Visual)
	VisualFgColor = lipgloss.Color("#FFFFFF")

	SelArrowStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(cfg.LevelWarn)).
		Bold(true)
	// 键帽不做色块：亮色按键名 + 灰动作词，靠色相对比（theme accent 自适应）
	KeyCapStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(cfg.Accent)).
		Bold(true)
	FrameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(darken(cfg.Dim, 0.55)))
	TitleBarStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.Accent))
	// tab 选中态与面包屑均不做背景色块：选中=亮色粗体，未选中=灰，靠色相区分
	PopupActiveTabStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(cfg.Accent))
	BreadcrumbStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(cfg.Accent))

	VisualStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(cfg.Visual)).
		Foreground(lipgloss.Color("#FFFFFF"))

	HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Dim))
	HelpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.Accent))

	FoldedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Dim)).Italic(true)
	NewLogStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LevelWarn)).Bold(true)

	DetailLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.Accent))
	DetailDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Dim))

	// 弹窗不铺背景色：透明浮层靠边框界定区域——边框用主题色暗化档（可见但不抢戏）
	PopupBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(darken(cfg.Accent, 0.55))).
		Padding(0, 1)

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
		// LevelWarn/LevelError 保留徽章底色，不铺主题 bg
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

// IsLightTheme 按 Bg 亮度判断亮暗主题（无 Bg 视为 dark，终端底色自适配）。
func (c ThemeConfig) IsLightTheme() bool {
	if c.Bg == "" {
		return false
	}
	r, g, b := hexToRGB(c.Bg)
	luma := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	return luma > 140
}

// darken 将 hex 颜色按比例压暗（f 为保留亮度比例）。
func darken(hex string, f float64) string {
	r, g, b := hexToRGB(hex)
	return fmt.Sprintf("#%02X%02X%02X", int(float64(r)*f), int(float64(g)*f), int(float64(b)*f))
}

// lighten 将 hex 颜色向白色混合（f 为白色占比）。
func lighten(hex string, f float64) string {
	r, g, b := hexToRGB(hex)
	mix := func(v int) int { return int(float64(v) + (255-float64(v))*f) }
	return fmt.Sprintf("#%02X%02X%02X", mix(r), mix(g), mix(b))
}

func ResolveTheme(name string, overrides map[string]string) ThemeConfig {
	base := DarkTheme
	if name == "light" {
		base = LightTheme
	} else if t, ok := builtInThemes[name]; ok {
		base = t
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
