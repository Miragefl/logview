package model

import "testing"

// TestStripANSIOSCAndExotics OSC 超链接/私有模式 CSI/两字符转义都必须剥干净:
// 残留的 URL 数字会被搜索误命中(TUI 搜 31 命中 "\x1b]8;;http://h/31" 里的 31,
// 2026-11-04 验收反馈:搜索须忽略日志自身的颜色/转义序列)。
func TestStripANSIOSCAndExotics(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"SGR 颜色", "\x1b[31mred\x1b[0m", "red"},
		{"256 色", "\x1b[38;5;196mdeep\x1b[0m", "deep"},
		{"truecolor", "\x1b[38;2;255;0;0mrgb\x1b[0m", "rgb"},
		{"OSC 超链接 BEL 终止", "\x1b]8;;http://host/31/path\x07link\x1b]8;;\x07 ok", "link ok"},
		{"OSC 标题 ST 终止", "\x1b]0;title\x1b\\text", "text"},
		{"私有模式 CSI(光标显隐 ?25h)", "\x1b[?25hcur", "cur"},
		{"两字符转义(字符集 \\x1b(B)", "\x1b(Babc", "abc"},
	}
	for _, c := range cases {
		if got := StripANSI(c.in); got != c.want {
			t.Errorf("%s: StripANSI(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}
