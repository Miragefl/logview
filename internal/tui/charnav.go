// Package tui 字符光标移动(vim-copy):message 消息段 rune 域移动/选区子串/wrap 段投影。
// 语义对齐 vim 简化版——字母/数字/下划线(unicode)为词字符,连续标点为一词,空白跳过;
// 消息首尾停住(不跨行接续,spec 拍板)。宽度(displayWidth/CJK 2 列)是渲染层的事,
// 本文件只用 rune index。TS 侧 fview/frontend/src/lib/charMotion.ts 与本文件
// 逐用例对齐(m*/s*/w* 同表,charnav_test.go)。
package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// Motion 字符移动类型(对齐 TS charMotion.Motion)。
type Motion int

const (
	MotionLeft Motion = iota
	MotionRight
	MotionWordFwd
	MotionWordBack
	MotionWordEnd
	MotionStart
	MotionFirstNonBlank
	MotionEnd
)

// runeClass 字符三分类:词/标点/空白。
type runeClass int

const (
	classWord runeClass = iota
	classPunct
	classSpace
)

func classOf(r rune) runeClass {
	if unicode.IsSpace(r) {
		return classSpace
	}
	if isWordRune(r) {
		return classWord
	}
	return classPunct
}

// isWordRune vim iskeyword 简化:字母/数字/下划线(unicode)。
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// ApplyMotion 消息内 rune 列移动;空消息返回 0,结果 clamp 到 [0, len-1]。
func ApplyMotion(msg string, col int, m Motion) int {
	r := []rune(msg)
	n := len(r)
	if n == 0 {
		return 0
	}
	c := col
	if c < 0 {
		c = 0
	}
	if c > n-1 {
		c = n - 1
	}
	clamp := func(i int) int {
		if i > n-1 {
			return n - 1
		}
		return i
	}
	switch m {
	case MotionLeft:
		if c-1 < 0 {
			return 0
		}
		return c - 1
	case MotionRight:
		return clamp(c + 1)
	case MotionStart:
		return 0
	case MotionFirstNonBlank:
		for i := 0; i < n; i++ {
			if classOf(r[i]) != classSpace {
				return i
			}
		}
		return 0 // 全空白行回行首
	case MotionEnd:
		return n - 1
	case MotionWordFwd:
		// 跳过当前同类段(空白上直接跳空白),落下一词首;到末尾停住
		i := c
		cls := classOf(r[i])
		if cls != classSpace {
			for i < n && classOf(r[i]) == cls {
				i++
			}
		}
		for i < n && classOf(r[i]) == classSpace {
			i++
		}
		return clamp(i)
	case MotionWordBack:
		// 词中(左邻同类)退当前词首;词首/空白退到前一词首
		i := c
		if i > 0 && classOf(r[i]) != classSpace && classOf(r[i-1]) == classOf(r[i]) {
			for i > 0 && classOf(r[i-1]) == classOf(r[i]) {
				i--
			}
			return i
		}
		for i > 0 && classOf(r[i-1]) == classSpace {
			i--
		}
		if i == 0 {
			return 0
		}
		j := i - 1
		cls := classOf(r[j])
		for j > 0 && classOf(r[j-1]) == cls {
			j--
		}
		return j
	case MotionWordEnd:
		// 在词尾(下一字符异类/末尾)→跳过空白到下一词再取其尾;否则取当前词尾
		i := c
		cls := classOf(r[i])
		if cls != classSpace && (i == n-1 || classOf(r[i+1]) != cls) {
			i++
			for i < n && classOf(r[i]) == classSpace {
				i++
			}
		}
		var c2 runeClass = classSpace
		if i < n {
			c2 = classOf(r[i])
		}
		for i < n-1 && classOf(r[i+1]) == c2 {
			i++
		}
		return clamp(i)
	}
	return c
}

// CharSelText 字符选区子串:msgs 按行序(锚..光标已归一为首..尾),
// 首行 [startCol..]、中间行全量、尾行 [..endCol](端点 inclusive,列 clamp 行内);
// chars 为 text 的 rune 计数(含换行)。
func CharSelText(msgs []string, startCol, endCol int) (string, int) {
	if len(msgs) == 0 {
		return "", 0
	}
	sub := func(s string, from, to int) string {
		r := []rune(s)
		last := len(r) - 1
		if last < 0 {
			last = 0
		}
		if from < 0 {
			from = 0
		}
		if from > last {
			from = last
		}
		if to < from {
			to = from
		}
		if to > last {
			to = last
		}
		return string(r[from : to+1])
	}
	parts := make([]string, len(msgs))
	for i, s := range msgs {
		switch {
		case i == 0 && i == len(msgs)-1:
			parts[i] = sub(s, startCol, endCol)
		case i == 0:
			r := []rune(s)
			parts[i] = sub(s, startCol, len(r)-1)
		case i == len(msgs)-1:
			parts[i] = sub(s, 0, endCol)
		default:
			parts[i] = s
		}
	}
	text := strings.Join(parts, "\n")
	return text, len([]rune(text))
}

// ColToSeg wrap 段投影:rune col → 第几段+段内 rune 偏移;col 越界 clamp 末段尾。
func ColToSeg(segs []string, col int) (segIdx, colInSeg int) {
	if len(segs) == 0 {
		return 0, 0
	}
	acc := 0
	for i, s := range segs {
		l := len([]rune(s))
		if i == len(segs)-1 || col < acc+l {
			in := col - acc
			if in < 0 {
				in = 0
			}
			if in > l-1 {
				in = l - 1
			}
			if in < 0 {
				in = 0
			}
			return i, in
		}
		acc += l
	}
	return len(segs) - 1, 0
}

// markReverseAtRune 纯文本第 col 个 rune 处包 reverse-video SGR(字符光标块;
// s 须为无 ANSI 的原文——注入发生在样式 Render/compactJSON 之前,光标字符
// 跟随显示变换走;wrapAnsiText 切段时 SGR replay 自动把标记带到正确段)。
func markReverseAtRune(s string, col int) string {
	r := []rune(s)
	if col < 0 || col >= len(r) {
		return s
	}
	return string(r[:col]) + "\x1b[7m" + string(r[col]) + "\x1b[27m" + string(r[col+1:])
}

// markSelRange 纯文本 rune 区间 [from..to] 包选区背景(to<0 到行尾;端点 inclusive;
// 与 markReverseAtRune 同前置条件——注入在样式 Render/compactJSON 之前)。
func markSelRange(s string, from, to int) string {
	r := []rune(s)
	if from < 0 || from >= len(r) || to < from {
		return s
	}
	end := to
	if end > len(r)-1 {
		end = len(r) - 1
	}
	sel := lipgloss.NewStyle().Background(VisualBgColor).Foreground(VisualFgColor)
	return string(r[:from]) + sel.Render(string(r[from:end+1])) + "\x1b[0m" + string(r[end+1:])
}

// sgrOf 从 lipgloss Style 提取裸 SGR 前缀(Render("") 空内容不输出 SGR,
// 用占位字符截前缀)。
func sgrOf(st lipgloss.Style) string {
	s := st.Render("X")
	i := strings.Index(s, "X")
	if i < 0 {
		return ""
	}
	return s[:i]
}

// markSelAndCursor 选区底纹+光标块一次成型:两个标记都在同一次 rune 遍历中按
// 原文列注入(先 sel 后 cursor 的分次注入会因 ANSI rune 污染列域互相劈开序列)。
// 光标字符=reverse(压选区);selFrom<0 或 cursorCol<0 时退化为单项注入。
func markSelAndCursor(s string, selFrom, selTo, cursorCol int) string {
	if selFrom < 0 && cursorCol < 0 {
		return s
	}
	if selFrom < 0 {
		return markReverseAtRune(s, cursorCol)
	}
	if cursorCol < 0 || cursorCol >= len([]rune(s)) {
		return markSelRange(s, selFrom, selTo)
	}
	r := []rune(s)
	selSGR := sgrOf(lipgloss.NewStyle().Background(VisualBgColor).Foreground(VisualFgColor))
	inSel := func(i int) bool { return i >= selFrom && (selTo < 0 || i <= selTo) }
	var b strings.Builder
	wasSel := false
	for i := 0; i < len(r); i++ {
		if i == cursorCol {
			if wasSel {
				b.WriteString("\x1b[0m")
				wasSel = false
			}
			b.WriteString("\x1b[7m" + string(r[i]) + "\x1b[27m")
			continue
		}
		if inSel(i) {
			if !wasSel {
				b.WriteString(selSGR)
				wasSel = true
			}
		} else if wasSel {
			b.WriteString("\x1b[0m")
			wasSel = false
		}
		b.WriteRune(r[i])
	}
	if wasSel {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}
