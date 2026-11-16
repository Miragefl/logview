package tui

// applyEncoding 切换重转测试:buffer 存量行 Reencode + 重解析(span 重建)+
// 视图刷新 + 记忆落盘 + 阈值确认层。

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Miragefl/logview/internal/encoding"
	"github.com/Miragefl/logview/internal/model"
	"github.com/Miragefl/logview/internal/parser"
)

func newEncApp(t *testing.T) *App {
	t.Helper()
	app := newTestApp()
	app.encStore = encoding.NewStore(filepath.Join(t.TempDir(), "encoding.json"))
	app.curEncKey = "ssh:web1/app/x.log"
	return app
}

func TestApplyEncodingReflowsBuffer(t *testing.T) {
	app := newEncApp(t)
	app.processLine(model.RawLine{Text: "\xd6\xd0\xce\xc4 \xc8\xd5\xd6\xbe INFO start", Source: "src", Enc: encoding.EncUTF8})
	app.applyEncoding(encoding.EncGBK)
	pl := lastLine(app)
	if pl.Raw.Text != "中文 日志 INFO start" {
		t.Errorf("重转后文本 = %q", pl.Raw.Text)
	}
	// span 重解析:Message span 必须覆盖新文本(不能残留旧偏移)
	if pl.Message() != "中文 日志 INFO start" {
		t.Errorf("Message span 未随重转重建 = %q", pl.Message())
	}
	if pl.Raw.Enc != encoding.EncGBK {
		t.Errorf("行 Enc = %v, want gbk", pl.Raw.Enc)
	}
}

// 检测行(Enc=GB18030)手切 UTF-8 再切回 auto:应恢复检测语义(已转文本)。
func TestApplyEncodingAutoRestoresDetected(t *testing.T) {
	app := newEncApp(t)
	app.processLine(model.RawLine{Text: "中文 detected", Source: "src", Enc: encoding.EncGB18030})
	app.applyEncoding(encoding.EncUTF8) // 看原始字节
	if got := lastLine(app).Raw.Text; got != "\xd6\xd0\xce\xc4 detected" {
		t.Fatalf("手切 UTF-8 未还原原始字节: %q", got)
	}
	app.applyEncoding(encoding.EncAuto) // 切回 auto=恢复检测结论
	if got := lastLine(app).Raw.Text; got != "中文 detected" {
		t.Errorf("auto 未恢复检测语义: %q", got)
	}
}

func TestApplyEncodingWritesMemory(t *testing.T) {
	app := newEncApp(t)
	app.processLine(model.RawLine{Text: "plain", Source: "src", Enc: encoding.EncUTF8})
	app.applyEncoding(encoding.EncGBK)
	if got := app.encStore.LoadFor(app.curEncKey); got != encoding.EncGBK {
		t.Errorf("记忆未落盘: %v", got)
	}
	app.applyEncoding(encoding.EncAuto)
	if got := app.encStore.LoadFor(app.curEncKey); got != encoding.EncAuto {
		t.Errorf("切回 auto 应删条目: %v", got)
	}
}

func TestApplyEncodingNoopWhenSame(t *testing.T) {
	app := newEncApp(t)
	app.processLine(model.RawLine{Text: "plain", Source: "src", Enc: encoding.EncUTF8})
	app.effEnc = encoding.EncGBK
	app.applyEncoding(encoding.EncGBK)
	if got := lastLine(app).Raw.Text; got != "plain" {
		t.Errorf("同编码不应动行: %q", got)
	}
}

// 程序行([logview] 零值旁路)混入 buffer,重转不动它。
func TestReencodeProgramLineUntouched(t *testing.T) {
	app := newEncApp(t)
	app.processLine(model.RawLine{Text: "\xd6\xd0\xce\xc4 data", Source: "src", Enc: encoding.EncUTF8})
	app.processLine(model.RawLine{Text: "[logview] 中文提示行", Source: "logview"}) // 零值
	app.applyEncoding(encoding.EncGBK)
	n := app.buffer.Len()
	prog := app.buffer.Get(n - 1)
	if prog.Raw.Text != "[logview] 中文提示行" {
		t.Errorf("程序行被误转: %q", prog.Raw.Text)
	}
	if prog.Raw.Enc != encoding.EncAuto {
		t.Errorf("程序行 Enc 被改: %v", prog.Raw.Enc)
	}
}

func TestApplyEncodingThresholdConfirm(t *testing.T) {
	app := newEncApp(t)
	orig := encConfirmBytes
	encConfirmBytes = 64 // 调小阈值:单行 100 字节即触发确认
	defer func() { encConfirmBytes = orig }()
	app.processLine(model.RawLine{Text: strings.Repeat("a", 100), Source: "src", Enc: encoding.EncUTF8})
	app.applyEncoding(encoding.EncGBK)
	if !app.encConfirming {
		t.Fatal("超阈值未开确认层")
	}
	if app.effEnc != encoding.EncAuto {
		t.Errorf("确认前不应生效: %v", app.effEnc)
	}
	app.confirmEncoding() // 模拟 y 确认
	if app.encConfirming {
		t.Error("确认后确认层未关")
	}
	if app.effEnc != encoding.EncGBK {
		t.Errorf("确认后未生效: %v", app.effEnc)
	}
}

func TestBadgeShowsOnlyNonUTF8(t *testing.T) {
	app := newEncApp(t)
	app.processLine(model.RawLine{Text: "x", Source: "src", Enc: encoding.EncUTF8})
	app.effEnc = encoding.EncGBK
	if !strings.Contains(app.View(), "[GBK]") {
		t.Error("GBK 生效应显示徽章 [GBK]")
	}
	app.effEnc = encoding.EncAuto
	if strings.Contains(app.View(), "[GBK]") {
		t.Error("auto 不应显示徽章")
	}
	app.effEnc = encoding.EncUTF8
	if strings.Contains(app.View(), "[GBK]") {
		t.Error("utf-8 不应显示徽章")
	}
}

// I1 回归:GBK 流前导 ASCII 段(检测直通行标 UTF8,真结论 GB18030 在后续行才
// 出现)——切 GBK 再切回 auto,两行都必须保持正确显示。
func TestApplyEncodingAutoAfterLateDetection(t *testing.T) {
	app := newEncApp(t)
	app.processLine(model.RawLine{Text: "boot ascii line", Source: "src", Enc: encoding.EncUTF8})
	app.processLine(model.RawLine{Text: "中文 arrived", Source: "src", Enc: encoding.EncGB18030})
	app.applyEncoding(encoding.EncGBK)
	app.applyEncoding(encoding.EncAuto)
	n := app.buffer.Len()
	if got := app.buffer.Get(n - 2).Raw.Text; got != "boot ascii line" {
		t.Errorf("ASCII 前导行 = %q, want 原样", got)
	}
	if got := lastLine(app).Raw.Text; got != "中文 arrived" {
		t.Errorf("GBK 行 = %q, want 中文(切回 auto 恢复检测结论)", got)
	}
}

// I2/RF#4 防线:重转后字段 span 必须随重解析重建——"换文本不重建 span"的
// 假想 bug 下,旧偏移取新文本会得到错串,此断言变红。
func TestApplyEncodingReparseSpanLogback(t *testing.T) {
	p, _ := parser.NewRegexParser("java-logback",
		`(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`)
	app := NewApp(&mockStream{}, parser.NewAutoDetect([]parser.Parser{p}), 1000, nil)
	app.width, app.height = 120, 40
	app.encStore = encoding.NewStore(filepath.Join(t.TempDir(), "encoding.json"))
	app.curEncKey = "file:/gbk.log"
	// "中文启动" GBK = d6d0 cec4 c6f4 b6af
	gbkLine := "2026-11-16 10:00:00.000 [main] INFO  com.example.App - \xd6\xd0\xce\xc4\xc6\xf4\xb6\xaf"
	app.processLine(model.RawLine{Text: gbkLine, Source: "src", Enc: encoding.EncUTF8})
	app.applyEncoding(encoding.EncGBK)
	pl := lastLine(app)
	if pl.Raw.Text != "2026-11-16 10:00:00.000 [main] INFO  com.example.App - 中文启动" {
		t.Errorf("重转后全文 = %q", pl.Raw.Text)
	}
	if pl.Message() != "中文启动" {
		t.Errorf("Message span 未随重转重建: %q", pl.Message())
	}
	if pl.Level() != "INFO" || pl.Thread() != "main" {
		t.Errorf("字段重解析失真: level=%q thread=%q", pl.Level(), pl.Thread())
	}
}
