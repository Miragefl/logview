package tui

// 内置开源主题。配色取自 iTerm2-Color-Schemes（mbadolato）对应 scheme 的官方 16 色。
// 映射约定：red→LevelError、yellow→LevelWarn、green→LevelInfo、cyan/blue→Accent/TraceID、
// purple/magenta→Source/TitleBG、8 号色(bright-black)→Dim/Debug；dark 系取 bright 变体，
// light 系取标准色并设置 Bg/Fg（全屏底色）。

// DraculaTheme: https://draculatheme.com — bg #282A36, purple 主调
var DraculaTheme = ThemeConfig{
	TitleFG:     "#F8F8F2",
	TitleBG:     "#BD93F9",
	LevelDebug:  "#6272A4",
	LevelInfo:   "#5FA88A",
	LevelWarn:   "#F1FA8C",
	LevelError:  "#FF5555",
	Time:        "#6272A4",
	Source:      "#FF79C6",
	TraceID:     "#8BE9FD",
	Thread:      "#9A9AB4",
	Highlight:   "#F1FA8C",
	Selected:    "#3B3355",
	Visual:      "#50FA7B",
	PopupBorder: "#BD93F9",
	PopupBg:     "#343746",
	Dim:         "#6272A4",
	Accent:      "#8BE9FD",
	Bg:          "",
	Fg:          "",
}

// TokyoStormTheme: Tokyo Night Storm — bg #24283B, blue 主调
var TokyoStormTheme = ThemeConfig{
	TitleFG:     "#C0CAF5",
	TitleBG:     "#7AA2F7",
	LevelDebug:  "#414868",
	LevelInfo:   "#6E9464",
	LevelWarn:   "#E0AF68",
	LevelError:  "#F7768E",
	Time:        "#414868",
	Source:      "#BB9AF7",
	TraceID:     "#7DCFFF",
	Thread:      "#565F89",
	Highlight:   "#E0AF68",
	Selected:    "#2A2F4A",
	Visual:      "#9ECE6A",
	PopupBorder: "#7AA2F7",
	PopupBg:     "#2F334D",
	Dim:         "#565F89",
	Accent:      "#7DCFFF",
	Bg:          "",
	Fg:          "",
}

// NordTheme: https://www.nordtheme.com — bg #2E3440, 冷灰蓝
var NordTheme = ThemeConfig{
	TitleFG:     "#ECEFF4",
	TitleBG:     "#81A1C1",
	LevelDebug:  "#4C566A",
	LevelInfo:   "#77866B",
	LevelWarn:   "#EBCB8B",
	LevelError:  "#BF616A",
	Time:        "#4C566A",
	Source:      "#B48EAD",
	TraceID:     "#88C0D0",
	Thread:      "#7B88A1",
	Highlight:   "#EBCB8B",
	Selected:    "#3B4252",
	Visual:      "#A3BE8C",
	PopupBorder: "#88C0D0",
	PopupBg:     "#3B4252",
	Dim:         "#4C566A",
	Accent:      "#88C0D0",
	Bg:          "",
	Fg:          "",
}

// GruvboxTheme: https://github.com/morhetz/gruvbox — bg #282828, 复古暖色
var GruvboxTheme = ThemeConfig{
	TitleFG:     "#EBDBB2",
	TitleBG:     "#B57614",
	LevelDebug:  "#928374",
	LevelInfo:   "#87843D",
	LevelWarn:   "#FABD2F",
	LevelError:  "#FB4934",
	Time:        "#928374",
	Source:      "#D3869B",
	TraceID:     "#8EC07C",
	Thread:      "#A89984",
	Highlight:   "#FABD2F",
	Selected:    "#32302F",
	Visual:      "#B8BB26",
	PopupBorder: "#FABD2F",
	PopupBg:     "#32302F",
	Dim:         "#928374",
	Accent:      "#8EC07C",
	Bg:          "",
	Fg:          "",
}

// CatppuccinTheme: Catppuccin Mocha — bg #1E1E2E, 柔和
var CatppuccinTheme = ThemeConfig{
	TitleFG:     "#CDD6F4",
	TitleBG:     "#89B4FA",
	LevelDebug:  "#585B70",
	LevelInfo:   "#6FA08C",
	LevelWarn:   "#F9E2AF",
	LevelError:  "#F38BA8",
	Time:        "#585B70",
	Source:      "#F5C2E7",
	TraceID:     "#94E2D5",
	Thread:      "#7F849C",
	Highlight:   "#F9E2AF",
	Selected:    "#313244",
	Visual:      "#A6E3A1",
	PopupBorder: "#89B4FA",
	PopupBg:     "#292C3C",
	Dim:         "#585B70",
	Accent:      "#94E2D5",
	Bg:          "",
	Fg:          "",
}

// OneDarkTheme: Atom One Dark — bg #21252B
var OneDarkTheme = ThemeConfig{
	TitleFG:     "#ABB2BF",
	TitleBG:     "#61AFEF",
	LevelDebug:  "#767676",
	LevelInfo:   "#6E8B67",
	LevelWarn:   "#E5C07B",
	LevelError:  "#E06C75",
	Time:        "#767676",
	Source:      "#C678DD",
	TraceID:     "#56B6C2",
	Thread:      "#828997",
	Highlight:   "#E5C07B",
	Selected:    "#2C313C",
	Visual:      "#98C379",
	PopupBorder: "#61AFEF",
	PopupBg:     "#2C313A",
	Dim:         "#767676",
	Accent:      "#56B6C2",
	Bg:          "",
	Fg:          "",
}

// GruvboxLightTheme: Gruvbox Light — bg #FBF1C7
var GruvboxLightTheme = ThemeConfig{
	TitleFG:     "#FBF1C7",
	TitleBG:     "#B57614",
	LevelDebug:  "#928374",
	LevelInfo:   "#79740E",
	LevelWarn:   "#B57614",
	LevelError:  "#9D0006",
	Time:        "#928374",
	Source:      "#8F3F71",
	TraceID:     "#076678",
	Thread:      "#7C6F64",
	Highlight:   "#FABD2F",
	Selected:    "#D5C4A1",
	Visual:      "#79740E",
	PopupBorder: "#B57614",
	PopupBg:     "#F2E5BC",
	Dim:         "#928374",
	Accent:      "#076678",
	Bg:          "#FBF1C7",
	Fg:          "#3C3836",
}

// CatppuccinLatteTheme: Catppuccin Latte — bg #EFF1F5
var CatppuccinLatteTheme = ThemeConfig{
	TitleFG:     "#EFF1F5",
	TitleBG:     "#1E66F5",
	LevelDebug:  "#ACB0BE",
	LevelInfo:   "#40A02B",
	LevelWarn:   "#DF8E1D",
	LevelError:  "#D20F39",
	Time:        "#ACB0BE",
	Source:      "#EA76CB",
	TraceID:     "#179299",
	Thread:      "#8C8FA1",
	Highlight:   "#E49931",
	Selected:    "#DCE0E8",
	Visual:      "#40A02B",
	PopupBorder: "#1E66F5",
	PopupBg:     "#E6E9EF",
	Dim:         "#ACB0BE",
	Accent:      "#179299",
	Bg:          "#EFF1F5",
	Fg:          "#4C4F69",
}

// NordLightTheme: Nord Light — bg #E5E9F0
var NordLightTheme = ThemeConfig{
	TitleFG:     "#ECEFF4",
	TitleBG:     "#81A1C1",
	LevelDebug:  "#4C566A",
	LevelInfo:   "#5E8159",
	LevelWarn:   "#BC9558",
	LevelError:  "#BF616A",
	Time:        "#4C566A",
	Source:      "#B48EAD",
	TraceID:     "#5A90A8",
	Thread:      "#5C6478",
	Highlight:   "#BC9558",
	Selected:    "#D8DEE9",
	Visual:      "#5E8159",
	PopupBorder: "#81A1C1",
	PopupBg:     "#DCE1EA",
	Dim:         "#4C566A",
	Accent:      "#5A90A8",
	Bg:          "#E5E9F0",
	Fg:          "#414858",
}

// TokyoDayTheme: TokyoNight Day — bg #E1E2E7
var TokyoDayTheme = ThemeConfig{
	TitleFG:     "#E1E2E7",
	TitleBG:     "#2E7DE9",
	LevelDebug:  "#A1A6C5",
	LevelInfo:   "#587539",
	LevelWarn:   "#8C6C3E",
	LevelError:  "#F52A65",
	Time:        "#A1A6C5",
	Source:      "#9854F1",
	TraceID:     "#007197",
	Thread:      "#848CB5",
	Highlight:   "#8C6C3E",
	Selected:    "#D5D9E4",
	Visual:      "#587539",
	PopupBorder: "#2E7DE9",
	PopupBg:     "#D4D7E0",
	Dim:         "#A1A6C5",
	Accent:      "#007197",
	Bg:          "#E1E2E7",
	Fg:          "#3760BF",
}

// SolarizedLightTheme: Solarized Light — bg #FDF6E3
var SolarizedLightTheme = ThemeConfig{
	TitleFG:     "#FDF6E3",
	TitleBG:     "#268BD2",
	LevelDebug:  "#93A1A1",
	LevelInfo:   "#859900",
	LevelWarn:   "#B58900",
	LevelError:  "#DC322F",
	Time:        "#93A1A1",
	Source:      "#D33682",
	TraceID:     "#2AA198",
	Thread:      "#839496",
	Highlight:   "#B58900",
	Selected:    "#EEE8D5",
	Visual:      "#859900",
	PopupBorder: "#268BD2",
	PopupBg:    "#EEE8D5",
	Dim:         "#93A1A1",
	Accent:      "#2AA198",
	Bg:          "#FDF6E3",
	Fg:          "#657B83",
}

// builtInThemes 注册全部内置主题（含原有 dark/light 之外的扩展）。
var builtInThemes = map[string]ThemeConfig{
	"dracula":          DraculaTheme,
	"tokyo-storm":      TokyoStormTheme,
	"nord":             NordTheme,
	"gruvbox":          GruvboxTheme,
	"catppuccin":       CatppuccinTheme,
	"one-dark":         OneDarkTheme,
	"gruvbox-light":    GruvboxLightTheme,
	"catppuccin-latte": CatppuccinLatteTheme,
	"nord-light":       NordLightTheme,
	"tokyo-day":        TokyoDayTheme,
	"solarized-light":  SolarizedLightTheme,
}

// ThemeNames 返回全部可用主题名（含 dark/light）。
func ThemeNames() []string {
	names := []string{"dark", "light"}
	for _, n := range []string{
		"dracula", "tokyo-storm", "nord", "gruvbox", "catppuccin", "one-dark",
		"gruvbox-light", "catppuccin-latte", "nord-light", "tokyo-day", "solarized-light",
	} {
		names = append(names, n)
	}
	return names
}
