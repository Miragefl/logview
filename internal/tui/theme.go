package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/Miragefl/logview/internal/logcore"
)

// 主题数据与解析（ThemeConfig/内置表/ResolveTheme）已迁 internal/logcore（fview 共享单源）；
// tui 保留 lipgloss 渲染层 ApplyTheme 与包内别名，tui/cmd 调用点零改动。
type ThemeConfig = logcore.ThemeConfig

var (
	DarkTheme    = logcore.DarkTheme
	LightTheme   = logcore.LightTheme
	ResolveTheme = logcore.ResolveTheme
	ThemeNames   = logcore.ThemeNames
	darken       = logcore.Darken
	hexToRGB     = logcore.HexToRGB
)

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

	// 高亮词前景色轮换板:彩色粗体+下划线,与搜索命中(HighlightStyle 底色块)形式区分。
	// 10 个独立色,不引用任何字段色(曾用 TraceID 青/Source 紫,与日志对应列撞色);
	// 色相环均布拉开间隔,个别与 WARN/ERROR 徽章的邻近色靠下划线兜底区分;
	// 按主题明暗分两版保证前景对比度。
	hlPalette := []string{ // 暗底亮字版
		"#FF87FF", // 品红
		"#5FD75F", // 亮绿
		"#87D7FF", // 淡天蓝
		"#FF5FD7", // 玫红
		"#AFFF87", // 浅黄绿
		"#D7D7FF", // 淡紫白
		"#FF875F", // 珊瑚橙
		"#5FAFFF", // 中蓝
		"#FFFF5F", // 柠檬黄
		"#00FFD7", // 玉青
	}
	if cfg.IsLightTheme() {
		hlPalette = []string{ // 亮底暗字版
			"#AF00AF", // 品红
			"#008700", // 绿
			"#005FAF", // 蓝
			"#D70087", // 玫红
			"#5F8700", // 黄绿
			"#875FAF", // 淡紫
			"#D75F00", // 橙
			"#0087AF", // 玉青蓝
			"#878700", // 暗黄
			"#00AFAF", // 青
		}
	}
	HighlightColors = nil
	for _, c := range hlPalette {
		HighlightColors = append(HighlightColors, lipgloss.Color(c))
	}

	// 搜索命中色轮换板:暗色系(亮色系归高亮词,明暗双域区分,见 SearchColors 兜底注释);
	// 亮底主题换深字版保证对比度。cfg.Highlight(旧底色块配置)自搜索改色后闲置,保留向后兼容。
	searchPalette := []string{ // 暗底版
		"#A0780E", // 暗琥珀
		"#6E8B3D", // 暗橄榄
		"#3E8B74", // 暗青绿
		"#4A7FA8", // 暗钢蓝
		"#8B6FA8", // 暗紫
		"#A86464", // 暗玫红
		"#8B7D5A", // 暗卡其
		"#5E8B8B", // 暗灰青
	}
	if cfg.IsLightTheme() {
		searchPalette = []string{ // 亮底深字版
			"#7A5A0A", // 深琥珀
			"#4F6A28", // 深橄榄
			"#2A6B54", // 深青绿
			"#345F85", // 深钢蓝
			"#6B4F85", // 深紫
			"#854A4A", // 深玫红
			"#6B5E3F", // 深卡其
			"#426969", // 深灰青
		}
	}
	SearchColors = nil
	for _, c := range searchPalette {
		SearchColors = append(SearchColors, lipgloss.Color(c))
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
