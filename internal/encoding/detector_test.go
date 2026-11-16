package encoding

import (
	"strings"
	"testing"
)

// stageAll 喂入全部行,收集检测器输出(逐行即时判定,零暂存)。
func stageAll(t *testing.T, d *Detector, lines []string) []StageLine {
	t.Helper()
	var out []StageLine
	for _, ln := range lines {
		out = append(out, d.Stage(ln)...)
	}
	return out
}

func TestDetectGBKChinese(t *testing.T) {
	d := NewDetector()
	out := stageAll(t, d, []string{"\xd6\xd0\xce\xc4 abc", "second \xc8\xd5\xd6\xbe"})
	if len(out) != 2 {
		t.Fatalf("输出 %d 行, want 2(逐行即时输出)", len(out))
	}
	if out[0].Text != "中文 abc" || out[0].Enc != EncGB18030 {
		t.Errorf("out[0] = %q,%v want 中文 abc,gb18030", out[0].Text, out[0].Enc)
	}
	if out[1].Text != "second 日志" || out[1].Enc != EncGB18030 {
		t.Errorf("out[1] = %q,%v want second 日志,gb18030", out[1].Text, out[1].Enc)
	}
	if !d.Concluded() || d.Result() != EncGB18030 {
		t.Errorf("Concluded=%v Result=%v, want true,gb18030", d.Concluded(), d.Result())
	}
}

func TestDetectUTF8ChinesePassesThrough(t *testing.T) {
	d := NewDetector()
	out := stageAll(t, d, []string{"中文 logback", "第二行"})
	if len(out) != 2 {
		t.Fatalf("输出 %d 行, want 2", len(out))
	}
	for i, want := range []string{"中文 logback", "第二行"} {
		if out[i].Text != want || out[i].Enc != EncUTF8 {
			t.Errorf("out[%d] = %q,%v want %q,utf-8", i, out[i].Text, out[i].Enc, want)
		}
	}
	if !d.Concluded() || d.Result() != EncUTF8 {
		t.Errorf("两可时按 UTF-8 优先结案, got Concluded=%v Result=%v", d.Concluded(), d.Result())
	}
}

func TestPureASCIINeverConverts(t *testing.T) {
	d := NewDetector()
	for i := range 1000 {
		line := "2026-11-16 10:00:00 INFO [main] step-" + strings.Repeat("x", i%17) + " done!"
		out := d.Stage(line) // ASCII 行即时直通,不暂存
		if len(out) != 1 || out[0].Text != line || out[0].Enc != EncUTF8 {
			t.Fatalf("ASCII 行应即时直通: 第 %d 行 got %v", i, out)
		}
	}
	if d.Result() != EncUTF8 {
		t.Errorf("Result = %v, want utf-8", d.Result())
	}
}

// 纯 ASCII 段(任意长度)零暂存直通;首个非 ASCII 行到达才定夺。
func TestLazyDetectionAfterASCIILines(t *testing.T) {
	d := NewDetector()
	var lines []string
	for i := range 2000 {
		lines = append(lines, "plain ascii line "+strings.Repeat("y", i%9))
	}
	out := stageAll(t, d, lines)
	if len(out) != len(lines) {
		t.Fatalf("ASCII 应逐行直通, got %d want %d", len(out), len(lines))
	}
	for i := range lines {
		if out[i].Text != lines[i] || out[i].Enc != EncUTF8 {
			t.Fatalf("ASCII 直通被破坏 at %d", i)
		}
	}
	if d.Concluded() {
		t.Fatal("纯 ASCII 段不应结案")
	}
	// GBK 行到达才定夺
	out2 := d.Stage("\xd6\xd0\xce\xc4 tail")
	if len(out2) != 1 || out2[0].Text != "中文 tail" || out2[0].Enc != EncGB18030 {
		t.Errorf("惰性定夺失败: %v", out2)
	}
	if !d.Concluded() || d.Result() != EncGB18030 {
		t.Errorf("Concluded=%v Result=%v want true,gb18030", d.Concluded(), d.Result())
	}
}

func TestIllegalBothGivesUp(t *testing.T) {
	d := NewDetector()
	out := d.Stage("\x81\x3f broken")
	if len(out) != 1 || out[0].Text != "\x81\x3f broken" || out[0].Enc != EncUTF8 {
		t.Errorf("两者皆非法应原样放弃: %v", out)
	}
	if !d.Concluded() || d.Result() != EncUTF8 {
		t.Errorf("放弃结案 Concluded=%v Result=%v", d.Concluded(), d.Result())
	}
}

// 超长 ASCII 行(旧 64KB 窗口上限之外)同样直通——窗口机制已删。
func TestHugeASCIILinePassesThrough(t *testing.T) {
	d := NewDetector()
	big := strings.Repeat("a", 128<<10)
	out := d.Stage(big)
	if len(out) != 1 || out[0].Text != big || out[0].Enc != EncUTF8 {
		t.Errorf("超长 ASCII 行直通失败: len=%d", len(out))
	}
	if d.Concluded() {
		t.Error("纯 ASCII 不结案")
	}
}

func TestPostConclusionPassthrough(t *testing.T) {
	d := NewDetector()
	stageAll(t, d, []string{"\xd6\xd0\xce\xc4 first"})
	if !d.Concluded() {
		t.Fatal("前置:应已结案")
	}
	// 结案后直通:立即返回,不暂存
	out := d.Stage("\xc8\xd5\xd6\xbe next")
	if len(out) != 1 || out[0].Text != "日志 next" || out[0].Enc != EncGB18030 {
		t.Errorf("结案后直通失败: %v", out)
	}
}
