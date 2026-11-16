package tui

// 编码到达行校正测试:App.effEnc(当前生效编码)对 processLine/processBatch
// 到达行的 Reencode 校正 + ReplaceStream 按源记忆预置。

import (
	"path/filepath"
	"testing"

	"github.com/Miragefl/logview/internal/encoding"
	"github.com/Miragefl/logview/internal/model"
)

// mockKeyedStream 带 EncodingKey 的 mock 源(记忆预置测试用)。
type mockKeyedStream struct {
	mockStream
	key string
}

func (m *mockKeyedStream) EncodingKey() string { return m.key }

func lastLine(a *App) *model.ParsedLine { return a.buffer.Get(a.buffer.Len() - 1) }

func TestProcessLineCorrectsIncomingGBK(t *testing.T) {
	app := newTestApp()
	app.effEnc = encoding.EncGBK
	app.processLine(model.RawLine{Text: "\xd6\xd0\xce\xc4 hi", Source: "src", Enc: encoding.EncUTF8})
	pl := lastLine(app)
	if pl.Raw.Text != "中文 hi" {
		t.Errorf("到达行未按 effEnc 校正: %q", pl.Raw.Text)
	}
}

func TestProcessLineBypassWhenAuto(t *testing.T) {
	app := newTestApp() // effEnc 默认 EncAuto
	app.processLine(model.RawLine{Text: "\xd6\xd0\xce\xc4 hi", Source: "src", Enc: encoding.EncUTF8})
	if pl := lastLine(app); pl.Raw.Text != "\xd6\xd0\xce\xc4 hi" {
		t.Errorf("auto 模式不应动行: %q", pl.Raw.Text)
	}
}

func TestProcessLineProgramLineBypass(t *testing.T) {
	app := newTestApp()
	app.effEnc = encoding.EncGBK
	app.processLine(model.RawLine{Text: "[logview] 中文提示", Source: "logview"}) // Enc 零值=程序行
	if pl := lastLine(app); pl.Raw.Text != "[logview] 中文提示" {
		t.Errorf("程序行被误转: %q", pl.Raw.Text)
	}
}

// 检测已转行(Enc=GB18030,文本已 UTF-8)在 effEnc=GBK 下为 no-op 往返,文本不变。
func TestProcessLineDetectedLineNoopUnderGBK(t *testing.T) {
	app := newTestApp()
	app.effEnc = encoding.EncGBK
	app.processLine(model.RawLine{Text: "中文 hi", Source: "src", Enc: encoding.EncGB18030})
	if pl := lastLine(app); pl.Raw.Text != "中文 hi" {
		t.Errorf("已转行应等值往返: %q", pl.Raw.Text)
	}
}

// 大批路径(processBatch ≥512 行并行段)同样校正。
func TestProcessBatchCorrectsIncomingGBK(t *testing.T) {
	app := newTestApp()
	app.effEnc = encoding.EncGBK
	lines := make([]model.RawLine, 600)
	for i := range lines {
		lines[i] = model.RawLine{Text: "\xd6\xd0\xce\xc4 batch", Source: "src", Enc: encoding.EncUTF8}
	}
	app.processBatch(lines)
	if app.buffer.Len() < 600 {
		t.Fatalf("buffer = %d, want ≥600", app.buffer.Len())
	}
	pl := lastLine(app)
	if pl.Raw.Text != "中文 batch" {
		t.Errorf("大批路径未校正: %q", pl.Raw.Text)
	}
}

func TestReplaceStreamLoadsMemory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "encoding.json")
	st := encoding.NewStore(p)
	st.Remember("ssh:web1/app/x.log", encoding.EncGBK)
	app := newTestApp()
	app.encStore = st
	app.ReplaceStream(&mockKeyedStream{key: "ssh:web1/app/x.log"})
	if app.effEnc != encoding.EncGBK {
		t.Errorf("effEnc = %v, want gbk(记忆预置失效)", app.effEnc)
	}
	if app.curEncKey != "ssh:web1/app/x.log" {
		t.Errorf("curEncKey = %q", app.curEncKey)
	}
}

func TestReplaceStreamNoKeyFallback(t *testing.T) {
	app := newTestApp() // mockStream 无 EncodingKey
	app.effEnc = encoding.EncGBK
	app.ReplaceStream(&mockStream{})
	if app.effEnc != encoding.EncAuto || app.curEncKey != "" {
		t.Errorf("无 key 源应回 auto/空, got %v/%q", app.effEnc, app.curEncKey)
	}
}
