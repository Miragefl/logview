package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/stream"
)

// SSH 密码认证流程：SSHSource 失败且 stderr 含 Permission denied 时，
// 在日志流插入提示行并展开密码输入框；Enter 后带密码重建 SSHSource 热切换。

// handleSSHPwKeys 密码框按键：Enter 重连 / Esc 取消。
func (a *App) handleSSHPwKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	input := inputRef{&a.sshPwInput, &a.sshPwCursor}
	switch msg.Type {
	case tea.KeyEscape:
		a.closeSSHPw()
	case tea.KeyEnter:
		return a, a.confirmSSHPw()
	default:
		input.handleEditKeys(msg)
	}
	return a, nil
}

func (a *App) closeSSHPw() {
	a.sshPwMode = false
	a.sshPwInput = ""
	a.sshPwCursor = 0
}

// confirmSSHPw 带密码重建 SSHSource 并热切换。
func (a *App) confirmSSHPw() tea.Cmd {
	host, path, pw := a.sshPwHost, a.sshPwPath, a.sshPwInput
	a.closeSSHPw()
	if pw == "" || host == "" {
		return nil
	}
	src := stream.NewSSHSource(host, path, 200)
	src.SetPassword(pw)
	return a.ReplaceStream(src)
}

// maybePromptSSHPassword 检查错误行：Permission denied 时展开密码框。
// 由 processLine 对 logview 自产的 ERROR 行调用。
func (a *App) maybePromptSSHPassword(text string) {
	if a.sshPwMode || a.sourcePickerMode {
		return
	}
	if !strings.Contains(strings.ToLower(text), "permission denied") {
		return
	}
	// 从当前 SSH 源取 host/path 重连
	src, ok := a.stream.(*stream.SSHSource)
	if !ok {
		return
	}
	a.sshPwHost = src.Host()
	a.sshPwPath = src.Path()
	a.sshPwMode = true
	a.sshPwInput = ""
	a.sshPwCursor = 0
}

// buildSSHPwLines 渲染密码输入弹窗（居中，日志透出）。
func (a *App) buildSSHPwLines(vl int) []string {
	var content strings.Builder
	content.WriteString(DetailLabelStyle.Render(fmt.Sprintf(" SSH 密码认证: %s", a.sshPwHost)) + "\n\n")
	// 密码不回显，仅显示掩码长度
	mask := strings.Repeat("*", len([]rune(a.sshPwInput)))
	content.WriteString(" 密码: " + mask + "█\n")
	content.WriteString("\n" + PopupTabStyle.Render(" Enter重连 Esc取消"))
	boxW := min(48, a.width-6)
	box := PopupBoxStyle.Width(boxW).Render(content.String())
	return a.overlayToVL(box, vl)
}
