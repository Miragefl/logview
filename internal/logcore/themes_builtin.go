package logcore

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

// SolarizedDarkTheme: Solarized Dark — bg #002B36, 经典青墨底(官方 iterm2 scheme
// 的 bright red/green/yellow 为灰占位,红/绿/黄按 Solarized 正色对齐,同 solarized-light 手法)
var SolarizedDarkTheme = ThemeConfig{
	TitleFG:     "#EEE8D5",
	TitleBG:     "#268BD2",
	LevelDebug:  "#586E75",
	LevelInfo:   "#A6B34D",
	LevelWarn:   "#B58900",
	LevelError:  "#DC322F",
	Time:        "#586E75",
	Source:      "#D33682",
	TraceID:     "#2AA198",
	Thread:      "#93A1A1",
	Highlight:   "#B58900",
	Selected:    "#073642",
	Visual:      "#859900",
	PopupBorder: "#268BD2",
	PopupBg:     "#0A3F4B",
	Dim:         "#586E75",
	Accent:      "#2AA198",
	Bg:          "",
	Fg:          "",
}

// KanagawaTheme: Kanagawa Wave — bg #1F1F28, 墨底浮世绘
var KanagawaTheme = ThemeConfig{
	TitleFG:     "#DCD7BA",
	TitleBG:     "#938AA9",
	LevelDebug:  "#727169",
	LevelInfo:   "#97B473",
	LevelWarn:   "#E6C384",
	LevelError:  "#E82424",
	Time:        "#727169",
	Source:      "#957FB8",
	TraceID:     "#7AA89F",
	Thread:      "#727169",
	Highlight:   "#E6C384",
	Selected:    "#16161D",
	Visual:      "#98BB6C",
	PopupBorder: "#938AA9",
	PopupBg:     "#272732",
	Dim:         "#727169",
	Accent:      "#7AA89F",
	Bg:          "",
	Fg:          "",
}

// EverforestTheme: Everforest Dark — bg #232A2E, 森林绿灰低对比
var EverforestTheme = ThemeConfig{
	TitleFG:     "#D3C6AA",
	TitleBG:     "#DF69BA",
	LevelDebug:  "#7A8683",
	LevelInfo:   "#6F8746",
	LevelWarn:   "#DFA000",
	LevelError:  "#F85552",
	Time:        "#7A8683",
	Source:      "#D699B6",
	TraceID:     "#35A77C",
	Thread:      "#859289",
	Highlight:   "#DFA000",
	Selected:    "#1A1F22",
	Visual:      "#8DA101",
	PopupBorder: "#DF69BA",
	PopupBg:     "#2B3338",
	Dim:         "#7A8683",
	Accent:      "#35A77C",
	Bg:          "",
	Fg:          "",
}

// RosePineMoonTheme: Rosé Pine Moon — bg #232136, 玫瑰金夜紫
var RosePineMoonTheme = ThemeConfig{
	TitleFG:     "#E0DEF4",
	TitleBG:     "#C4A7E7",
	LevelDebug:  "#6E6A86",
	LevelInfo:   "#53869B",
	LevelWarn:   "#F6C177",
	LevelError:  "#EB6F92",
	Time:        "#6E6A86",
	Source:      "#C4A7E7",
	TraceID:     "#9CCFD8",
	Thread:      "#6E6A86",
	Highlight:   "#F6C177",
	Selected:    "#1B1929",
	Visual:      "#3E8FB0",
	PopupBorder: "#C4A7E7",
	PopupBg:     "#2A2841",
	Dim:         "#6E6A86",
	Accent:      "#9CCFD8",
	Bg:          "",
	Fg:          "",
}

// MonokaiProTheme: Monokai Pro — bg #2D2A2E, 紫灰底荧光黄绿
var MonokaiProTheme = ThemeConfig{
	TitleFG:     "#FCFCFA",
	TitleBG:     "#AB9DF2",
	LevelDebug:  "#727072",
	LevelInfo:   "#A9C38F",
	LevelWarn:   "#FFD866",
	LevelError:  "#FF6188",
	Time:        "#727072",
	Source:      "#AB9DF2",
	TraceID:     "#78DCE8",
	Thread:      "#727072",
	Highlight:   "#FFD866",
	Selected:    "#232023",
	Visual:      "#A9DC76",
	PopupBorder: "#AB9DF2",
	PopupBg:     "#363337",
	Dim:         "#727072",
	Accent:      "#78DCE8",
	Bg:          "",
	Fg:          "",
}

// Synthwave84Theme: Synthwave '84 — bg #241B2F, 蒸汽波霓虹(官方 scheme bg 为纯黑,
// bg/选中/弹窗取 Synthwave '84 正色调)
var Synthwave84Theme = ThemeConfig{
	TitleFG:     "#F8F8F2",
	TitleBG:     "#FF7EDB",
	LevelDebug:  "#675389",
	LevelInfo:   "#50965D",
	LevelWarn:   "#FDF454",
	LevelError:  "#F841A0",
	Time:        "#675389",
	Source:      "#F85A21",
	TraceID:     "#19CDE6",
	Thread:      "#675389",
	Highlight:   "#FDF454",
	Selected:    "#34294F",
	Visual:      "#25C141",
	PopupBorder: "#F97137",
	PopupBg:     "#2B2140",
	Dim:         "#675389",
	Accent:      "#19CDE6",
	Bg:          "",
	Fg:          "",
}

// OneHalfLightTheme: One Half Light — bg #FAFAFA, One Dark 官方亮色搭档
var OneHalfLightTheme = ThemeConfig{
	TitleFG:     "#FAFAFA",
	TitleBG:     "#A626A4",
	LevelDebug:  "#4F525E",
	LevelInfo:   "#2F9E44",
	LevelWarn:   "#C18401",
	LevelError:  "#E45649",
	Time:        "#4F525E",
	Source:      "#A626A4",
	TraceID:     "#0997B3",
	Thread:      "#4F525E",
	Highlight:   "#C18401",
	Selected:    "#EDEDED",
	Visual:      "#2F9E44",
	PopupBorder: "#A626A4",
	PopupBg:     "#F4F4F4",
	Dim:         "#4F525E",
	Accent:      "#0997B3",
	Bg:          "#FAFAFA",
	Fg:          "#383A42",
}

// RosePineDawnTheme: Rosé Pine Dawn — bg #FAF4ED, 米纸底玫瑰灰紫
var RosePineDawnTheme = ThemeConfig{
	TitleFG:     "#FAF4ED",
	TitleBG:     "#907AA9",
	LevelDebug:  "#9893A5",
	LevelInfo:   "#266A85",
	LevelWarn:   "#EA9D34",
	LevelError:  "#B4637A",
	Time:        "#9893A5",
	Source:      "#907AA9",
	TraceID:     "#56949F",
	Thread:      "#9893A5",
	Highlight:   "#EA9D34",
	Selected:    "#F4E8D9",
	Visual:      "#266A85",
	PopupBorder: "#907AA9",
	PopupBg:     "#F7EEE3",
	Dim:         "#9893A5",
	Accent:      "#56949F",
	Bg:          "#FAF4ED",
	Fg:          "#575279",
}

// AyuLightTheme: Ayu Light — bg #F8F9FA, 极简灰白紫青点缀
var AyuLightTheme = ThemeConfig{
	TitleFG:     "#F8F9FA",
	TitleBG:     "#9E75C7",
	LevelDebug:  "#686868",
	LevelInfo:   "#55B03F",
	LevelWarn:   "#ECA944",
	LevelError:  "#EA6C6D",
	Time:        "#686868",
	Source:      "#9E75C7",
	TraceID:     "#46BA94",
	Thread:      "#686868",
	Highlight:   "#ECA944",
	Selected:    "#E9ECEF",
	Visual:      "#55B03F",
	PopupBorder: "#9E75C7",
	PopupBg:     "#F1F3F5",
	Dim:         "#686868",
	Accent:      "#46BA94",
	Bg:          "#F8F9FA",
	Fg:          "#5C6166",
}

// builtInThemes 注册全部内置主题（含原有 dark/light 之外的扩展）。
var builtInThemes = map[string]ThemeConfig{
	"dracula":         DraculaTheme,
	"tokyo-storm":     TokyoStormTheme,
	"gruvbox":         GruvboxTheme,
	"solarized-dark":  SolarizedDarkTheme,
	"kanagawa":        KanagawaTheme,
	"everforest":      EverforestTheme,
	"rose-pine-moon":  RosePineMoonTheme,
	"monokai-pro":     MonokaiProTheme,
	"synthwave-84":    Synthwave84Theme,
	"gruvbox-light":   GruvboxLightTheme,
	"tokyo-day":       TokyoDayTheme,
	"solarized-light": SolarizedLightTheme,
	"one-half-light":  OneHalfLightTheme,
	"rose-pine-dawn":  RosePineDawnTheme,
	"ayu-light":       AyuLightTheme,
}

// ThemeNames 返回全部可用主题名（含 dark/light）。
func ThemeNames() []string {
	names := []string{"dark", "light"}
	for _, n := range []string{
		"dracula", "tokyo-storm", "gruvbox",
		"solarized-dark", "kanagawa", "everforest", "rose-pine-moon", "monokai-pro", "synthwave-84",
		"gruvbox-light", "tokyo-day", "solarized-light",
		"one-half-light", "rose-pine-dawn", "ayu-light",
	} {
		names = append(names, n)
	}
	return names
}
