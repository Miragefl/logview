package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Search    key.Binding
	Fields    key.Binding
	Export    key.Binding
	Top       key.Binding
	Bottom    key.Binding
	Quit      key.Binding
	Up        key.Binding
	Down      key.Binding
	HalfUp    key.Binding
	HalfDown  key.Binding
	PageUp    key.Binding
	PageDown  key.Binding
	VimPageUp key.Binding
	VimPageDn key.Binding
	Visual    key.Binding
	Yank      key.Binding
	ScreenTop key.Binding
	ScreenMid key.Binding
	ScreenBot key.Binding
	LevelErr  key.Binding
	LevelWarn key.Binding
	LevelInfo key.Binding
	LevelDbg  key.Binding
	LevelAll  key.Binding
	SearchNext key.Binding
	SearchPrev key.Binding
	LineNum    key.Binding
	Stats      key.Binding
	Bookmark   key.Binding
	BmJump     key.Binding
	Help       key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Search:    key.NewBinding(key.WithKeys("/", "f"), key.WithHelp("f或/", "搜索")),
		Fields:    key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "设置")),
		Export:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "导出")),
		Top:       key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "顶部")),
		Bottom:    key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "底部")),
		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "退出")),
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "上移")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "下移")),
		HalfUp:    key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("C-u", "上半页")),
		HalfDown:  key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("C-d", "下半页")),
		PageUp:    key.NewBinding(key.WithKeys("pgup")),
		PageDown:  key.NewBinding(key.WithKeys("pgdown")),
		VimPageUp: key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("C-b", "上翻")),
		VimPageDn: key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("C-f", "下翻")),
		Visual:    key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "选择")),
		Yank:      key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "复制")),
		ScreenTop: key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "屏顶")),
		ScreenMid: key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "屏中")),
		ScreenBot: key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "屏底")),
		LevelErr:  key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "仅ERROR")),
		LevelWarn: key.NewBinding(key.WithKeys("W"), key.WithHelp("W", "ERROR+WARN")),
		LevelInfo: key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "去掉DEBUG")),
		LevelDbg:  key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "全部级别")),
		LevelAll:   key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "取消过滤")),
		SearchNext: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "下一个匹配")),
		SearchPrev: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "上一个匹配")),
		LineNum:    key.NewBinding(key.WithKeys("#"), key.WithHelp("#", "行号")),
		Stats:      key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "统计")),
		Bookmark:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "标记")),
		BmJump:     key.NewBinding(key.WithKeys("'"), key.WithHelp("'", "跳转标记")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "帮助")),
	}
}

func KeyMapFromConfig(config map[string]string) *KeyMap {
	km := DefaultKeyMap()
	if config == nil {
		return &km
	}
	// Don't override keys from config - just store the mapping for future use
	// For now, the default keymap is used and config is stored for reference
	return &km
}

func (km KeyMap) HelpEntries() []key.Binding {
	return []key.Binding{
		km.Search, km.Fields, km.Export,
		km.Top, km.Bottom, km.HalfUp, km.HalfDown, km.VimPageUp, km.VimPageDn,
		km.Visual, km.Yank, km.ScreenTop, km.ScreenMid, km.ScreenBot,
		km.LevelErr, km.LevelWarn, km.LevelInfo, km.LevelDbg, km.LevelAll, km.SearchNext, km.SearchPrev, km.LineNum, km.Stats, km.Bookmark, km.BmJump, km.Help, km.Quit,
	}
}
