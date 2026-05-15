package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Search    key.Binding
	NextMatch key.Binding
	PrevMatch key.Binding
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
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "搜索")),
		NextMatch: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "下一个")),
		PrevMatch: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "上一个")),
		Fields:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "设置")),
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
	}
}

func (km KeyMap) HelpEntries() []key.Binding {
	return []key.Binding{
		km.Search, km.NextMatch, km.Fields, km.Export,
		km.Top, km.Bottom, km.HalfUp, km.HalfDown, km.VimPageUp, km.VimPageDn, km.Quit,
	}
}
