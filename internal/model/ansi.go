package model

import (
	"regexp"
	"strings"
)

// ANSIRe 匹配通用 ANSI 转义序列（parser 清洗与 TUI 渲染共用同一份定义）。
// 三类全覆盖(2026-11-04:旧式只认 CSI 基本型,OSC 超链接里的 URL 数字会被
// 搜索误命中——搜 31 命中 "\x1b]8;;http://h/31"):
//   - CSI 全序列:参数字节 0x30-0x3F(数字;:=?<>) + 中间字节 [ -/] + 最终字节 [@-~]
//     (含 SGR 颜色/256 色/truecolor/私有模式 ?25h)
//   - OSC:\x1b] 开头,到 BEL(\x07)或 ST(\x1b\) 结束(超链接/窗口标题)
//   - 其他 ESC 序列:\x1b + 中间字节 [ -/]* + 终止字节 [\x30-\x7e]
//     (字符集选择 \x1b(B、keypad \x1b=、reverse index \x1bM 等)
var ANSIRe = regexp.MustCompile(`\x1b(?:\[[0-9;:?<=>]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)|[ -/]*[\x30-\x7e])`)

// StripANSI 移除文本中的 ANSI 转义序列。
// 快路径:不含 ESC 控制字节(绝大多数日志行)直接返回原串,免正则全行扫描——
// 该函数每行调用,大文件加载时全行正则扫描占解析路径可观份额。
func StripANSI(s string) string {
	if strings.IndexByte(s, '\x1b') < 0 {
		return s
	}
	return ANSIRe.ReplaceAllString(s, "")
}
