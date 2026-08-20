package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// o 键打开选择器；Tab 循环切换三 tab；Esc 关闭。
func TestSourcePickerOpenTabClose(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if !app.sourcePickerMode {
		t.Fatal("o 应打开源选择器")
	}
	if app.sourceTab != 0 {
		t.Fatalf("默认 tab 应为 0 (K8s)，实际 %d", app.sourceTab)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if app.sourceTab != 2 {
		t.Fatalf("两次 Tab 后应为 2 (SSH)，实际 %d", app.sourceTab)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if app.sourceTab != 1 {
		t.Fatalf("S-Tab 应回 1 (本地)，实际 %d", app.sourceTab)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.sourcePickerMode {
		t.Fatal("Esc 应关闭选择器")
	}
}

// K8s tab：Space 勾选多个候选，Enter 生成 MultiK8sSource 并热切换清屏。
func TestSourcePickerK8sMultiSelect(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(0)
	app.pickerCandidates = []sourceCandidate{
		{label: "deploy/a", value: "deployment/a"},
		{label: "deploy/b", value: "deployment/b"},
		{label: "pod/c", value: "pod/c"},
	}
	app.pickerLoading = false

	// 勾选 0 和 2（KeySpace）
	app.Update(tea.KeyMsg{Type: tea.KeySpace})
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeySpace})
	if len(app.pickerChecked) != 2 {
		t.Fatalf("应勾选 2 项，实际 %d", len(app.pickerChecked))
	}

	// 预填一些旧状态，切换后应清空
	app.processLine(model.RawLine{Text: "old line", Source: "old"})
	if app.buffer.Len() == 0 {
		t.Fatal("前置条件：应有旧行")
	}

	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.sourcePickerMode {
		t.Fatal("Enter 后应关闭选择器")
	}
	// 无 kubectl 环境：清屏后仅剩 1 条提示/错误行；label 应含两个勾选资源
	if app.buffer.Len() > 1 {
		t.Fatalf("切换源后应清屏（至多 1 条提示行），buffer=%d", app.buffer.Len())
	}
	label := app.stream.Label()
	if !strings.Contains(label, "deployment/a") || !strings.Contains(label, "pod/c") {
		t.Errorf("MultiK8s label 应含勾选项: %s", label)
	}
}

// 未勾选直接 Enter：使用光标所在候选（单源）。
func TestSourcePickerK8sCursorFallback(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(0)
	app.pickerCandidates = []sourceCandidate{{label: "deploy/a", value: "deployment/a"}}
	app.pickerLoading = false
	app.pickerCursor = 0

	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label := app.stream.Label(); !strings.Contains(label, "deployment") {
		t.Errorf("光标项未生效: %s", label)
	}
}

// 本地 tab：输入路径 Enter 生成 FileSource。
func TestSourcePickerLocalFile(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(1)
	app.pickerPathInput = "/tmp/some.log"
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label := app.stream.Label(); label == "test" {
		t.Error("本地文件源未生效（仍是 mock）")
	}
	if app.buffer.Len() > 1 {
		t.Errorf("切换后应清屏（至多 1 条切换提示），buffer=%d", app.buffer.Len())
	}
}

// SSH tab：候选过滤 + 焦点切换 + Enter 生成 SSHSource。
func TestSourcePickerSSH(t *testing.T) {
	SetSSHHosts([]string{"web1", "web2"})
	app := newTestApp()
	app.openSourcePicker(2)
	if len(sshCandidates()) != 2 {
		t.Fatalf("SSH 候选应为 2，实际 %d", len(sshCandidates()))
	}
	// 输入前缀过滤候选
	app.pickerHostInput = "web2"
	cands := app.visiblePickerCandidates()
	if len(cands) != 1 || cands[0].value != "web2" {
		t.Fatalf("前缀过滤失败: %v", cands)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label := app.stream.Label(); label != "ssh://web2/var/log/" {
		t.Errorf("SSH 源 label: %s", label)
	}
}

// SSH tab C-j 焦点切换：主机 ↔ 路径。
func TestSourcePickerSSHFocusToggle(t *testing.T) {
	SetSSHHosts([]string{"h1"})
	app := newTestApp()
	app.openSourcePicker(2)
	if app.pickerSshFocus != 0 {
		t.Fatal("默认焦点应在主机输入")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if app.pickerSshFocus != 1 {
		t.Fatal("C-j 后焦点应在路径输入")
	}
	// 焦点在路径时打字进 remotePath
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if app.pickerRemotePath != "/var/log/a" {
		t.Fatalf("打字应进路径输入，实际 %q", app.pickerRemotePath)
	}
}

// expandHome 展开 ~/ 前缀。
func TestExpandHome(t *testing.T) {
	p := expandHome("~/logs/app.log")
	if strings.HasPrefix(p, "~/") || !strings.Contains(p, "logs/app.log") {
		t.Errorf("expandHome 失败: %s", p)
	}
	if got := expandHome("/abs/path"); got != "/abs/path" {
		t.Errorf("绝对路径不应改写: %s", got)
	}
}
