package tui

import (
	"regexp"
	"strings"
)

// hintKeyRe 匹配 hint 文本中的按键词（长词优先，避免 C-j 被 Enter 类误切）。
var hintKeyRe = regexp.MustCompile(`C-[a-zA-Z/]+|S-Tab|Backspace|Space|Enter|Esc|Tab|j/k|↑/k|↓/j|←/h|→/l|g/G`)

// keycapHint 把 hint 文本中的按键词渲染为键帽色块，其余文字保持暗灰。
// 分段独立渲染（不做 ANSI 嵌套），reset 不会污染后续文字颜色。
func keycapHint(s string) string {
	matches := hintKeyRe.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return PopupTabStyle.Render(s)
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(PopupTabStyle.Render(s[last:m[0]]))
		b.WriteString(KeyCapStyle.Render(s[m[0]:m[1]]))
		last = m[1]
	}
	b.WriteString(PopupTabStyle.Render(s[last:]))
	return b.String()
}

// popupTabSep 弹窗 tab 栏下的通栏分隔线，宽度与弹窗内容区对齐
// （boxW 为 lipgloss Width 内容宽，含左右 padding 各 1，不含边框）。
func popupTabSep(boxW int) string {
	n := boxW - 2 // 左右 padding 各 1
	if n < 8 {
		n = 8
	}
	return DetailDimStyle.Render(strings.Repeat("─", n))
}
