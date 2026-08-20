package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/stream"
)

// 源选择器状态机：sourcePickerMode 时接管按键。
// K8s tab：ns 输入 + 候选多选（Space 勾选，Enter 确认 → MultiK8sSource）
// 本地 tab：目录/路径输入 + 候选（Enter → FileSource）
// SSH tab：主机候选 + 远程路径输入（Enter → SSHSource）

// openSourcePicker 打开选择器（tab 可预选）。
func (a *App) openSourcePicker(tab int) {
	a.sourcePickerMode = true
	a.sourceTab = tab
	a.pickerNsInput = "default"
	a.pickerPathInput = ""
	a.pickerHostInput = ""
	a.pickerRemotePath = "/var/log/"
	a.pickerRemoteCursor = len(a.pickerRemotePath)
	a.pickerCursor = 0
	a.pickerSshFocus = 0 // 0=主机输入 1=远程路径输入
	a.pickerChecked = map[string]bool{}
	a.pickerCandidates = nil
	a.pickerLoading = false
	if tab == 0 {
		a.pickerLoading = true
	}
}

func (a *App) closeSourcePicker() {
	a.sourcePickerMode = false
	a.pickerCandidates = nil
	a.pickerChecked = nil
}

// visiblePickerCandidates 当前 tab 的可见候选。
func (a *App) visiblePickerCandidates() []sourceCandidate {
	switch a.sourceTab {
	case 0:
		return a.pickerCandidates
	case 1:
		return loadLocalCandidates(a.pickerPathInput)
	case 2:
		// 主机候选按已输入前缀过滤
		prefix := strings.TrimSpace(a.pickerHostInput)
		var out []sourceCandidate
		for _, c := range sshCandidates() {
			if prefix == "" || strings.HasPrefix(c.value, prefix) || strings.Contains(c.value, prefix) {
				out = append(out, c)
			}
		}
		return out
	}
	return nil
}

func (a *App) handleSourcePickerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cands := a.visiblePickerCandidates()

	switch msg.Type {
	case tea.KeyEscape:
		a.closeSourcePicker()
		return a, nil
	case tea.KeyTab:
		a.sourceTab = (a.sourceTab + 1) % 3
		a.pickerCursor = 0
		if a.sourceTab == 0 && a.pickerCandidates == nil {
			a.pickerLoading = true
			return a, fetchK8sCandidatesCmd(a.pickerNsInput)
		}
		return a, nil
	case tea.KeyShiftTab:
		a.sourceTab = (a.sourceTab + 2) % 3
		a.pickerCursor = 0
		if a.sourceTab == 0 && a.pickerCandidates == nil {
			a.pickerLoading = true
			return a, fetchK8sCandidatesCmd(a.pickerNsInput)
		}
		return a, nil
	case tea.KeyUp:
		if a.pickerCursor > 0 {
			a.pickerCursor--
		}
		return a, nil
	case tea.KeyDown:
		if a.pickerCursor < len(cands)-1 {
			a.pickerCursor++
		}
		return a, nil
	case tea.KeySpace:
		// K8s tab：勾选/取消当前候选（多选）
		if a.sourceTab == 0 && a.pickerCursor < len(cands) {
			v := cands[a.pickerCursor].value
			a.pickerChecked[v] = !a.pickerChecked[v]
		}
		return a, nil
	case tea.KeyEnter:
		return a, a.confirmSourcePicker()
	case tea.KeyCtrlJ:
		// SSH tab：主机 ↔ 路径 切焦点
		if a.sourceTab == 2 {
			a.pickerSshFocus = (a.pickerSshFocus + 1) % 2
		}
		return a, nil
	case tea.KeyCtrlK:
		if a.sourceTab == 2 {
			a.pickerSshFocus = (a.pickerSshFocus + 1) % 2
		}
		return a, nil
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "k":
			if a.pickerCursor > 0 {
				a.pickerCursor--
			}
			return a, nil
		case "j":
			if a.pickerCursor < len(cands)-1 {
				a.pickerCursor++
			}
			return a, nil
		case " ":
			if a.sourceTab == 0 && a.pickerCursor < len(cands) {
				v := cands[a.pickerCursor].value
				a.pickerChecked[v] = !a.pickerChecked[v]
			}
			return a, nil
		}
	}

	// 输入框编辑：ns / path / remote path
	input := a.pickerInputRef()
	if input.text != nil {
		if _, changed := input.handleEditKeys(msg); changed && a.sourceTab == 0 {
			// ns 变化时重新拉取 k8s 候选（防抖：仅 Enter 时刷新，见 confirmSourcePicker）
			return a, nil
		}
	}
	return a, nil
}

// pickerInputRef 返回当前 tab 活跃输入框（ns / 本地路径 / SSH 主机或路径按焦点）。
func (a *App) pickerInputRef() inputRef {
	switch a.sourceTab {
	case 0:
		return inputRef{&a.pickerNsInput, &a.pickerNsCursor}
	case 1:
		return inputRef{&a.pickerPathInput, &a.pickerPathCursor}
	case 2:
		if a.pickerSshFocus == 0 {
			return inputRef{&a.pickerHostInput, &a.pickerHostCursor}
		}
		return inputRef{&a.pickerRemotePath, &a.pickerRemoteCursor}
	}
	return inputRef{}
}

// confirmSourcePicker 组装新 stream 并热切换。返回 ReplaceStream 的 cmd（新流监听）。
func (a *App) confirmSourcePicker() tea.Cmd {
	// 先读状态再关（closeSourcePicker 会清空候选与勾选）
	tab := a.sourceTab
	cursor := a.pickerCursor
	checked := a.pickerChecked
	cands := a.pickerCandidates
	nsInput := a.pickerNsInput
	pathInput := a.pickerPathInput
	hostInput := a.pickerHostInput
	remotePath := a.pickerRemotePath
	a.closeSourcePicker()

	switch tab {
	case 0: // K8s 多选
		var resources []string
		for v, ok := range checked {
			if ok {
				resources = append(resources, v)
			}
		}
		if len(resources) == 0 {
			// 未勾选时若光标在候选上，直接用光标项
			if cursor < len(cands) {
				resources = []string{cands[cursor].value}
			}
		}
		if len(resources) == 0 {
			return nil
		}
		sortStrings(resources)
		var sources []*stream.K8sSource
		for _, r := range resources {
			sources = append(sources, stream.NewK8sSource(r, nsInput, nil, 200))
		}
		var src stream.LogStream
		if len(sources) == 1 {
			src = sources[0]
		} else {
			src = stream.NewMultiK8sSource(sources)
		}
		return a.ReplaceStream(src)
	case 1: // 本地文件
		path := strings.TrimSpace(pathInput)
		cands := loadLocalCandidates(pathInput)
		if path == "" || !strings.ContainsAny(path, "/") && cursor < len(cands) {
			// 纯文件名且光标有候选 → 用候选全路径
			path = cands[cursor].value
		}
		if path == "" {
			return nil
		}
		paths := expandHome(path)
		return a.ReplaceStream(stream.NewFileSource([]string{paths}))
	case 2: // SSH
		host := strings.TrimSpace(hostInput)
		cands := sshCandidates()
		if host == "" && cursor < len(cands) {
			host = cands[cursor].value
		}
		if host == "" {
			return nil
		}
		return a.ReplaceStream(stream.NewSSHSource(host, remotePath, 200))
	}
	return nil
}

// buildSourcePickerLines 渲染选择器（inline popup，同搜索弹窗模式）。
func (a *App) buildSourcePickerLines(vl int) []string {
	var content strings.Builder

	tabs := []string{"K8s", "本地", "SSH"}
	var tabParts []string
	for i, t := range tabs {
		if i == a.sourceTab {
			tabParts = append(tabParts, PopupActiveTabStyle.Render(" "+t+" "))
		} else {
			tabParts = append(tabParts, PopupTabStyle.Render(" "+t+" "))
		}
	}
	content.WriteString(strings.Join(tabParts, PopupTabStyle.Render("│")) + "\n\n")

	cands := a.visiblePickerCandidates()
	switch a.sourceTab {
	case 0:
		content.WriteString(a.inputLine(a.pickerNsInput, a.pickerNsCursor, "namespace") + "\n\n")
		if a.pickerLoading {
			content.WriteString(PopupTabStyle.Render(" 查询中…") + "\n")
		} else if len(cands) == 0 {
			content.WriteString(PopupTabStyle.Render(" 无候选（检查 kubectl/namespace）") + "\n")
		} else {
			content.WriteString(renderCandidateList(cands, a.pickerCursor, a.pickerChecked, 8))
		}
		content.WriteString("\n" + PopupTabStyle.Render(" Space勾选(可多选) Enter确认 Tab切分区 Esc取消"))
	case 1:
		content.WriteString(a.inputLine(a.pickerPathInput, a.pickerPathCursor, "目录或文件路径 (默认当前目录)") + "\n\n")
		if len(cands) == 0 {
			content.WriteString(PopupTabStyle.Render(" 目录下无日志文件") + "\n")
		} else {
			content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8))
		}
		content.WriteString("\n" + PopupTabStyle.Render(" Enter打开 Tab切分区 Esc取消"))
	case 2:
		hostLabel, pathLabel := "主机: ", "路径: "
		_ = pathLabel
		hostLine := a.inputLine(a.pickerHostInput, a.pickerHostCursor, "user@host 或候选中选择")
		pathLine := a.inputLine(a.pickerRemotePath, a.pickerRemoteCursor, "/var/log/xxx.log")
		if a.pickerSshFocus == 0 {
			content.WriteString(DetailLabelStyle.Render(hostLabel) + hostLine + "\n\n")
		} else {
			content.WriteString(PopupTabStyle.Render(hostLabel) + hostLine + "\n\n")
		}
		if len(cands) > 0 {
			content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 6))
			content.WriteString("\n")
		} else if a.pickerHostInput != "" {
			content.WriteString(PopupTabStyle.Render(" 无匹配主机，将直接使用输入值") + "\n\n")
		}
		if a.pickerSshFocus == 1 {
			content.WriteString(DetailLabelStyle.Render(pathLabel) + pathLine + "\n")
		} else {
			content.WriteString(PopupTabStyle.Render(pathLabel) + pathLine + "\n")
		}
		content.WriteString("\n" + PopupTabStyle.Render(" Enter连接tail -F C-j/k切焦点 Tab切分区 Esc取消"))
	}

	boxW := min(60, a.width-4)
	box := PopupBoxStyle.Width(boxW).Render(content.String())
	return a.inlinePopupLines(box, vl)
}

// renderCandidateList 渲染候选列表（可滚动窗口，勾选态仅 k8s 多选用）。
func renderCandidateList(cands []sourceCandidate, cursor int, checked map[string]bool, maxRows int) string {
	var b strings.Builder
	start := 0
	if cursor >= maxRows {
		start = cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(cands) {
		end = len(cands)
	}
	for i := start; i < end; i++ {
		prefix := "  "
		mark := "  "
		if checked != nil && checked[cands[i].value] {
			mark = DetailLabelStyle.Render("✓ ")
		}
		if i == cursor {
			prefix = SelectedStyle.Render(" >")
		}
		b.WriteString(prefix + " " + mark + DetailValueStyle.Render(cands[i].label) + "\n")
	}
	if len(cands) > end {
		b.WriteString(PopupTabStyle.Render(fmt.Sprintf("   …共%d项", len(cands))) + "\n")
	}
	return b.String()
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return strings.Replace(p, "~", home, 1)
		}
	}
	return p
}
