package tui

// 编码弹窗与确认层交互测试:P 呼出、C-j/C-k 导航、Enter 应用、Esc 关闭、
// 确认层 y/n 流程、help 键位说明同步。

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Miragefl/logview/internal/encoding"
	"github.com/Miragefl/logview/internal/model"
)

func pressKey(app *App, r rune) tea.Cmd {
	m, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	_ = m.(*App)
	return cmd
}

func encKeyMsg(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "ctrl+j":
		return tea.KeyMsg{Type: tea.KeyCtrlJ}
	case "ctrl+k":
		return tea.KeyMsg{Type: tea.KeyCtrlK}
	case "y":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	case "n":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestEncodingPopupOpenClose(t *testing.T) {
	app := newEncApp(t)
	pressKey(app, 'P')
	if !app.encPopup {
		t.Fatal("P 未呼出编码弹窗")
	}
	v := app.View()
	for _, want := range []string{"auto", "UTF-8", "GBK", "GB18030"} {
		if !strings.Contains(v, want) {
			t.Errorf("弹窗缺选项 %s", want)
		}
	}
	_, _ = app.Update(encKeyMsg("esc"))
	if app.encPopup {
		t.Error("Esc 未关闭弹窗")
	}
}

func TestEncodingPopupSelectApplies(t *testing.T) {
	app := newEncApp(t)
	pressKey(app, 'P')
	_, _ = app.Update(encKeyMsg("ctrl+j")) // auto→UTF-8
	_, _ = app.Update(encKeyMsg("ctrl+j")) // UTF-8→GBK
	_, _ = app.Update(encKeyMsg("enter"))
	if app.encPopup {
		t.Error("Enter 后弹窗未关")
	}
	if app.effEnc != encoding.EncGBK {
		t.Errorf("effEnc = %v, want gbk", app.effEnc)
	}
	if !strings.Contains(app.View(), "[GBK]") {
		t.Error("应用后状态栏应显示 [GBK] 徽章")
	}
}

func TestEncodingConfirmLayerFlow(t *testing.T) {
	app := newEncApp(t)
	orig := encConfirmBytes
	encConfirmBytes = 32
	defer func() { encConfirmBytes = orig }()
	app.processLine(model.RawLine{Text: "\xd6\xd0\xce\xc4 big", Source: "src", Enc: encoding.EncUTF8})
	pressKey(app, 'P')
	_, _ = app.Update(encKeyMsg("ctrl+j"))
	_, _ = app.Update(encKeyMsg("ctrl+j"))
	_, _ = app.Update(encKeyMsg("enter")) // GBK:超阈值 → 确认层
	if !app.encConfirming {
		t.Fatal("未开确认层")
	}
	if app.effEnc != encoding.EncAuto {
		t.Errorf("确认前生效 = %v, want auto", app.effEnc)
	}
	if !strings.Contains(app.View(), "y") {
		t.Error("确认层应显示 y/n 提示")
	}
	_, _ = app.Update(encKeyMsg("n"))
	if app.encConfirming || app.effEnc != encoding.EncAuto {
		t.Error("n 取消失败")
	}
	// 重来一次,y 放行
	pressKey(app, 'P')
	_, _ = app.Update(encKeyMsg("ctrl+j"))
	_, _ = app.Update(encKeyMsg("ctrl+j"))
	_, _ = app.Update(encKeyMsg("enter"))
	_, _ = app.Update(encKeyMsg("y"))
	if app.effEnc != encoding.EncGBK {
		t.Errorf("y 放行失败: %v", app.effEnc)
	}
	if lastLine(app).Raw.Text != "中文 big" {
		t.Errorf("y 后未重转: %q", lastLine(app).Raw.Text)
	}
}

func TestHelpIncludesEncodingKey(t *testing.T) {
	app := newTestApp()
	app.helpMode = true
	v := app.View()
	if !strings.Contains(v, "P") || !strings.Contains(v, "字符集") {
		t.Error("help 弹窗应含 P 选择字符集条目")
	}
}
