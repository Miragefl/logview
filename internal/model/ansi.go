package model

import "regexp"

// ANSIRe 匹配通用 ANSI 转义序列（parser 清洗与 TUI 渲染共用同一份定义）。
var ANSIRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI 移除文本中的 ANSI 转义序列。
func StripANSI(s string) string {
	return ANSIRe.ReplaceAllString(s, "")
}
