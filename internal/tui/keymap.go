package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Search   key.Binding
	Enter    key.Binding
	Tab      key.Binding
	Fields   key.Binding
	Expand   key.Binding
	Export   key.Binding
	Jump     key.Binding
	Quit     key.Binding
	Escape   key.Binding
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Search: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "搜索")),
		Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "提取")),
		Tab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "面板")),
		Fields: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "字段")),
		Expand: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "堆栈")),
		Export: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "导出")),
		Jump: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "跳转")),
		Quit: key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "退出")),
		Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "取消")),
		Up: key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "上移")),
		Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "下移")),
		PageUp: key.NewBinding(key.WithKeys("pgup")),
		PageDown: key.NewBinding(key.WithKeys("pgdown")),
	}
}

func (km KeyMap) HelpEntries() []key.Binding {
	return []key.Binding{
		km.Search, km.Tab, km.Enter, km.Fields,
		km.Expand, km.Export, km.Jump, km.Quit,
	}
}