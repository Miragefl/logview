package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// inputRef 把 App 中分离的 (text, cursor) 字段适配为单行编辑器，
// 供搜索/高亮/隐藏三个输入框复用同一套按键编辑逻辑。
type inputRef struct {
	text   *string
	cursor *int
}

// insert 在光标处插入 s。
func (t inputRef) insert(s string) {
	runes := []rune(*t.text)
	pos := *t.cursor
	if pos > len(runes) {
		pos = len(runes)
	}
	runes = append(runes[:pos], append([]rune(s), runes[pos:]...)...)
	*t.text = string(runes)
	*t.cursor = pos + len([]rune(s))
}

// backspace 删除光标前一个字符。
func (t inputRef) backspace() {
	runes := []rune(*t.text)
	if *t.cursor > 0 && len(runes) > 0 {
		*t.cursor--
		runes = append(runes[:*t.cursor], runes[*t.cursor+1:]...)
		*t.text = string(runes)
	}
}

// deleteForward 删除光标处字符。
func (t inputRef) deleteForward() {
	runes := []rune(*t.text)
	if *t.cursor < len(runes) {
		runes = append(runes[:*t.cursor], runes[*t.cursor+1:]...)
		*t.text = string(runes)
	}
}

func (t inputRef) moveLeft() {
	if *t.cursor > 0 {
		*t.cursor--
	}
}

func (t inputRef) moveRight() {
	if *t.cursor < len([]rune(*t.text)) {
		*t.cursor++
	}
}

func (t inputRef) home() { *t.cursor = 0 }

func (t inputRef) end() { *t.cursor = len([]rune(*t.text)) }

func (t inputRef) clear() {
	*t.text = ""
	*t.cursor = 0
}

// clamp 把光标收敛到合法范围（Tab 切换输入框后调用）。
func (t inputRef) clamp() {
	if *t.cursor > len([]rune(*t.text)) {
		*t.cursor = len([]rune(*t.text))
	}
}

// handleEditKeys 处理所有输入框共通的编辑键（退格/插入/移动/删除/清空/空格）。
// 返回 handled=是否已消费该键，changed=键是否修改了文本内容。
func (t inputRef) handleEditKeys(msg tea.KeyMsg) (handled, changed bool) {
	switch msg.Type {
	case tea.KeyBackspace:
		before := *t.text
		t.backspace()
		return true, before != *t.text
	case tea.KeyRunes:
		if len(msg.Runes) > 0 {
			t.insert(string(msg.Runes))
			return true, true
		}
		return true, false
	default:
		switch msg.String() {
		case "left":
			t.moveLeft()
		case "right":
			t.moveRight()
		case "home", "ctrl+a":
			t.home()
		case "end", "ctrl+e":
			t.end()
		case "delete":
			before := *t.text
			t.deleteForward()
			return true, before != *t.text
		case "ctrl+u":
			before := *t.text
			t.clear()
			return true, before != *t.text
		case " ":
			t.insert(" ")
			return true, true
		}
	}
	return false, false
}
