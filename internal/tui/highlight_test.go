// isSubseq/splitHighlight 测试(tui-picker-unify):与 fview frontend/src/lib/
// highlight.test.ts 同表逐条对齐(子串优先/非连续子序列/大小写/空词 identity/
// 不 trim),算法零漂移。
package tui

import (
	"reflect"
	"testing"
)

func TestSplitHighlight(t *testing.T) {
	seg := func(ts string, hit bool) HLSeg { return HLSeg{T: ts, Hit: hit} }
	cases := []struct {
		name string
		text string
		kw   string
		want []HLSeg
	}{
		{"空词 identity 整体 miss", "/tmp/app.log", "", []HLSeg{seg("/tmp/app.log", false)}},
		{"全空格词 identity", "/tmp/app.log", "   ", []HLSeg{seg("/tmp/app.log", false)}},
		{"连续子串命中整段高亮", "/tmp/error.log", "error", []HLSeg{seg("/tmp/", false), seg("error", true), seg(".log", false)}},
		{"大小写不敏感", "/tmp/ERROR.log", "error", []HLSeg{seg("/tmp/", false), seg("ERROR", true), seg(".log", false)}},
		{"子串优先:词 m 整段命中 my 而非散点贪心", "my app", "m", []HLSeg{seg("m", true), seg("y app", false)}},
		{"非连续子序列 hde→hardware(fzf 式)", "hardware", "hde", []HLSeg{seg("h", true), seg("ar", false), seg("d", true), seg("war", false), seg("e", true)}},
		{"不命中整体 miss", "/tmp/app.log", "zzz", []HLSeg{seg("/tmp/app.log", false)}},
		{"词长于文本不命中", "abc", "abcd", []HLSeg{seg("abc", false)}},
		{"不 trim:词含空格照匹配(子串优先整段)", "my app log", "y a", []HLSeg{seg("m", false), seg("y a", true), seg("pp log", false)}},
	}
	for _, c := range cases {
		if got := splitHighlight(c.text, c.kw); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: splitHighlight(%q,%q)=%+v want %+v", c.name, c.text, c.kw, got, c.want)
		}
	}
}

func TestIsSubseq(t *testing.T) {
	cases := []struct {
		text string
		kw   string
		want bool
	}{
		{"/tmp/app.log", "", true},       // 空词恒真
		{"hardware", "hde", true},        // 非连续
		{"hardware", "ehd", false},       // 顺序错不命中(e 在 h 后无 d)
		{"/tmp/ERROR.log", "error", true}, // 大小写
		{"abc", "abcd", false},           // 词长于文本
		{"my app", "m", true},            // 子串优先不影响谓词
		{"abc", "zzz", false},
	}
	for _, c := range cases {
		if got := isSubseq(c.text, c.kw); got != c.want {
			t.Errorf("isSubseq(%q,%q)=%v want %v", c.text, c.kw, got, c.want)
		}
	}
}
