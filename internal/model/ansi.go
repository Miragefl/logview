package model

import (
	"regexp"
	"strings"
)

// ANSIRe 匹配通用 ANSI 转义序列（parser 清洗与 TUI 渲染共用同一份定义）。
var ANSIRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI 移除文本中的 ANSI 转义序列。
// 快路径:不含 ESC 控制字节(绝大多数日志行)直接返回原串,免正则全行扫描——
// 该函数每行调用,大文件加载时全行正则扫描占解析路径可观份额。
func StripANSI(s string) string {
	if strings.IndexByte(s, '\x1b') < 0 {
		return s
	}
	return ANSIRe.ReplaceAllString(s, "")
}
