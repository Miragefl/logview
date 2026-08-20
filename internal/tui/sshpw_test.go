package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/stream"
)

// 高亮/隐藏确认时记录历史；C-r 打开对应分区历史并可填入。
func TestHighlightHideHistory(t *testing.T) {
	app := newTestApp()
	// 高亮 tab 确认两次（第二次前清空回显值）
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	for _, r := range "err,fail" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlU}) // 清回显
	for _, r := range "timeout" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.highlightHistory) != 2 {
		t.Fatalf("高亮历史应 2 条，实际 %v", app.highlightHistory)
	}

	// 隐藏 tab 确认
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	for _, r := range "health" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.hideHistory) != 1 {
		t.Fatalf("隐藏历史应 1 条，实际 %d", len(app.hideHistory))
	}
	// 高亮 tab C-r 打开历史并填入最新一条（先清空残留输入）
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if !app.searchHistMode {
		t.Fatal("高亮 tab C-r 应打开历史列表")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 填入最新（timeout）
	if app.highlightInput != "timeout" {
		t.Fatalf("历史应填入高亮输入框，实际 %q", app.highlightInput)
	}
}

// SSH Permission denied 错误行 → 弹密码框；Enter 带密码重连。
func TestSSHPwPromptFlow(t *testing.T) {
	app := newTestApp()
	// 模拟已连接 SSH 源失败（stream 是 mock，直接构造场景）
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	app.openSourcePicker(2)
	app.pickerSSHHost = ""
	app.closeSourcePicker()
	// 手动挂 SSH 源后注入错误行
	app.stream = stream.NewSSHSource("needpw", "/var/log/x.log", 10)
	app.processLine(model.RawLine{Text: "ERROR Permission denied (publickey,password)", Source: "needpw"})
	if !app.sshPwMode {
		t.Fatal("Permission denied 应弹密码框")
	}
	if app.sshPwHost != "needpw" {
		t.Fatalf("密码框应记录主机，实际 %q", app.sshPwHost)
	}
	// 输入密码 Enter → ReplaceStream 到带密码的 SSH 源
	for _, r := range "secret" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	view := app.buildSSHPwLines(app.visibleLines())
	if !strings.Contains(view2str(view), "***") {
		t.Fatalf("密码应掩码显示，view=\n%s", view2str(view))
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.sshPwMode {
		t.Fatal("Enter 后密码框应关闭")
	}
}

// picker 目录浏览来源的密码确认：不建 tail 源，回目录浏览层并缓存密码。
func TestSSHPwFromPickerReturnsToBrowse(t *testing.T) {
	app := newTestApp()
	app.promptSSHPwForHost("lane:/var/log")
	if !app.sshPwMode || !app.sshPwFromPicker {
		t.Fatal("picker 来源应设 fromPicker 并展开密码框")
	}
	for _, r := range "secret" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.sshPwMode {
		t.Fatal("确认后密码框应关闭")
	}
	if app.sshPasswords["lane"] != "secret" {
		t.Fatalf("密码应入内存缓存，实际 %q", app.sshPasswords["lane"])
	}
	// 应回到 picker 远程目录层（而非切换 tail 流）
	if !app.sourcePickerMode || app.pickerSSHHost != "lane" || app.pickerSSHDir != "/var/log" {
		t.Fatalf("应回到目录浏览层: mode=%v host=%q dir=%q", app.sourcePickerMode, app.pickerSSHHost, app.pickerSSHDir)
	}
	if label := app.stream.Label(); label == "ssh://lane/var/log" {
		t.Fatal("picker 来源不应直接建 tail 流")
	}
}

// tail 流来源的密码确认：带密码 ReplaceStream（现状行为）。
func TestSSHPwFromStreamReconnects(t *testing.T) {
	app := newTestApp()
	app.stream = stream.NewSSHSource("needpw", "/var/log/x.log", 10)
	app.processLine(model.RawLine{Text: "ERROR Permission denied (publickey,password)", Source: "needpw"})
	if !app.sshPwMode || app.sshPwFromPicker {
		t.Fatal("stream 来源应弹密码框且 fromPicker=false")
	}
	for _, r := range "pw123" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label := app.stream.Label(); label != "ssh://needpw/var/log/x.log" {
		t.Fatalf("stream 来源应重连 tail: %s", label)
	}
	if app.sshPasswords["needpw"] != "pw123" {
		t.Fatal("密码应缓存")
	}
}

func view2str(lines []string) string { return strings.Join(lines, "\n") }
