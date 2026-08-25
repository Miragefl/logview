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
	a.sshPwFromPicker = false
}

// sshPw 返回主机的缓存密码（无则空串=免密）。
func (a *App) sshPw(host string) string {
	return a.sshPasswords[host]
}

// confirmSSHPw 密码确认分流：frp 浏览 → 存密码回 FRP 目录层；
// picker 目录浏览来源 → 存密码回到 SSH 目录浏览；tail 流来源 → 带密码重连。
func (a *App) confirmSSHPw() tea.Cmd {
	host, path, pw := a.sshPwHost, a.sshPwPath, a.sshPwInput
	fromPicker := a.sshPwFromPicker
	frpPort, frpUser := a.sshPwFRPPort, a.sshPwUser
	a.sshPwFRPPort = 0 // 先清标记，防 closeSSHPw 误杀隧道
	a.closeSSHPw()
	if pw == "" || host == "" {
		return nil
	}
	// 密码入内存缓存（后续同主机连接/浏览免重输）
	if a.sshPasswords == nil {
		a.sshPasswords = make(map[string]string)
	}
	a.sshPasswords[host] = pw

	if frpPort > 0 {
		// frp 浏览来源：重开 FRP 目录层，带密码重拉（隧道复用）
		t := a.pickerFRPTunnel
		a.openSourcePicker(3)
		a.pickerFRPLevel = 2
		a.pickerFRPTunnel = t
		a.pickerFRPDir = path
		a.pickerFRPUser = frpUser
		a.pickerFRPProxy = strings.TrimPrefix(host, "frp:")
		a.pickerFRPConnName = strings.TrimPrefix(host, "frp:")
		a.pickerCandidates = nil
		a.pickerCursor = 0
		a.pickerLoading = true
		return fetchFRPDirCmd(frpUser, frpPort, path, pw)
	}
	if fromPicker {
		// 回到 picker 的远程目录层，带密码重拉目录列表（原 SSH 逻辑不变）
		a.openSourcePicker(2)
		a.pickerSSHHost = host
		a.pickerSSHDir = path
		a.pickerSSHRoot = path
		a.pickerCandidates = nil
		a.pickerCursor = 0
		a.pickerLoading = true
		return fetchSSHDirCmd(host, path, pw)
	}
	// frp tail 来源：原源带密码重启（隧道复用，不能走 ReplaceStream 的 Cleanup）
	if strings.HasPrefix(host, "frp:") {
		if src, ok := a.stream.(*stream.FRPSource); ok {
			src.SetPassword(pw)
			return a.restartCurrentStream()
		}
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
	// 从当前 SSH/FRP 源取 host/path 重连
	src, ok := a.stream.(interface {
		Host() string
		Path() string
	})
	if !ok {
		return
	}
	a.sshPwHost = src.Host()
	a.sshPwPath = src.Path()
	a.sshPwFromPicker = false
	a.sshPwMode = true
	a.sshPwInput = ""
	a.sshPwCursor = 0
}

// promptSSHPwForHost 在 picker 内浏览远程目录失败（认证错误）时展开密码框。
// target 为 "host:/path" 形式；Enter 后带密码回到该目录继续浏览。
func (a *App) promptSSHPwForHost(target string) {
	host, path, _ := strings.Cut(target, ":")
	if host == "" {
		return
	}
	if path == "" {
		path = "/"
	}
	a.sshPwHost = host
	a.sshPwPath = path
	a.sshPwFromPicker = true
	a.sshPwMode = true
	a.sshPwInput = ""
	a.sshPwCursor = 0
	a.closeSourcePicker() // 密码框接管渲染；确认后重开定位到目录层
}

// promptFRPPw frp 目录浏览认证失败：弹密码框（隧道保留，确认后继续浏览）。
func (a *App) promptFRPPw() {
	if a.pickerFRPTunnel == nil || a.pickerFRPUser == "" {
		return
	}
	t := a.pickerFRPTunnel
	a.pickerFRPTunnel = nil // 暂摘下，防 closeSourcePicker 清理
	a.closeSourcePicker()
	a.pickerFRPTunnel = t
	a.sshPwHost = a.frpPwKey()
	a.sshPwPath = a.pickerFRPDir
	a.sshPwUser = a.pickerFRPUser
	a.sshPwFRPPort = t.LocalPort()
	a.sshPwFromPicker = true
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
