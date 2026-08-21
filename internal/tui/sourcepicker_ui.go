package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/stream"
)

// 源选择器（o 键）— 浏览式交互：
//   K8s tab:   context → namespace → 资源多选（Space 勾选，Enter 确认）
//   本地 tab:  目录浏览器（Enter 进目录/开文件，Backspace 返回上级）
//   SSH tab:   主机列表 → 远程目录浏览器
// Backspace 逐级返回；手动输入仍保留（路径/ns 直达）。

// openSourcePicker 打开选择器（tab 可预选）。
func (a *App) openSourcePicker(tab int) {
	a.sourcePickerMode = true
	a.sourceTab = tab
	a.pickerChecked = map[string]bool{}
	a.pickerCursor = 0
	a.pickerLoading = false
	a.pickerDirFilter = ""
	a.pickerFilterCursor = 0
	a.pickerPathInput = ""
	// 每 tab 初始化浏览状态
	a.pickerK8sLevel = 0
	a.pickerKubeCtx = ""
	a.pickerCurCtxCache = ""
	a.pickerContexts = nil
	a.pickerNamespaces = nil
	a.pickerNsInput = ""
	a.pickerCandidates = nil
	a.pickerLocalDir = "."
	if cwd, err := os.Getwd(); err == nil {
		a.pickerLocalDir = cwd
	}
	a.pickerSSHHost = ""
	a.pickerSSHDir = ""
	a.pickerSSHRoot = ""
	a.pickerHostInput = ""
	a.pickerRemotePath = ""
	a.pickerSshFocus = 0
}

func (a *App) closeSourcePicker() {
	a.sourcePickerMode = false
	a.pickerCandidates = nil
	a.pickerChecked = nil
}

// visiblePickerCandidates 当前 tab/level 的可见候选。
func (a *App) visiblePickerCandidates() []sourceCandidate {
	switch a.sourceTab {
	case 0: // K8s
		switch a.pickerK8sLevel {
		case 0:
			return a.pickerContexts
		case 1:
			// ns 层按输入前缀过滤（输入即选中）
			filter := strings.ToLower(strings.TrimSpace(a.pickerNsInput))
			if filter == "" {
				return a.pickerNamespaces
			}
			var out []sourceCandidate
			for _, c := range a.pickerNamespaces {
				if strings.Contains(strings.ToLower(c.value), filter) {
					out = append(out, c)
				}
			}
			return out
		default:
			filter := strings.ToLower(a.pickerDirFilter)
			if filter == "" {
				return a.pickerCandidates
			}
			var out []sourceCandidate
			for _, c := range a.pickerCandidates {
				if strings.Contains(strings.ToLower(c.label), filter) {
					out = append(out, c)
				}
			}
			return out
		}
	case 1: // 本地目录（. 开头过滤词显示隐藏文件；../ 返回上级）
		filter := a.pickerDirFilter
		showHidden := strings.HasPrefix(filter, ".")
		items := []sourceCandidate{{label: "../", value: "..", dir: true}}
		for _, e := range listLocalDir(a.pickerLocalDir, showHidden) {
			if filter != "" && !strings.Contains(strings.ToLower(e.name), strings.ToLower(filter)) {
				continue
			}
			if e.isDir {
				items = append(items, sourceCandidate{label: e.name + "/", value: e.name, dir: true})
			} else {
				items = append(items, sourceCandidate{label: e.name, value: e.name})
			}
		}
		return items
	case 2: // SSH
		if a.pickerSSHHost == "" {
			// 主机层：按已输入前缀过滤
			prefix := strings.TrimSpace(a.pickerHostInput)
			var out []sourceCandidate
			for _, c := range sshCandidates() {
				if prefix == "" || strings.HasPrefix(c.value, prefix) || strings.Contains(c.value, prefix) {
					out = append(out, c)
				}
			}
			return out
		}
		return a.filteredSSHCands() // 远程目录层
	}
	return nil
}

// filteredSSHCands 远程目录层：非根目录首位 ../ 返回上级，其余按过滤前缀过滤。
func (a *App) filteredSSHCands() []sourceCandidate {
	filter := strings.ToLower(a.pickerDirFilter)
	var items []sourceCandidate
	if a.pickerSSHDir != "/" {
		items = append(items, sourceCandidate{label: "../", value: "..", dir: true})
	}
	for _, c := range a.pickerCandidates {
		if filter == "" || strings.Contains(strings.ToLower(c.label), filter) {
			items = append(items, c)
		}
	}
	return items
}

// pickerBreadcrumbCtx 面包屑显示的 context：已选 context 或当前 context（缓存一次）。
func (a *App) pickerBreadcrumbCtx() string {
	if a.pickerKubeCtx != "" {
		return a.pickerKubeCtx
	}
	if a.pickerCurCtxCache == "" {
		a.pickerCurCtxCache = currentK8sContext()
	}
	return a.pickerCurCtxCache
}

func (a *App) handleSourcePickerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cands := a.visiblePickerCandidates()

	switch msg.Type {
	case tea.KeyEscape:
		a.closeSourcePicker()
		return a, nil
	case tea.KeyTab:
		a.sourceTab = (a.sourceTab + 1) % 3
		return a, a.pickerTabEnterCmd()
	case tea.KeyShiftTab:
		a.sourceTab = (a.sourceTab + 2) % 3
		return a, a.pickerTabEnterCmd()
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
	case tea.KeyBackspace:
		// 输入框有内容时优先删字符（避免想删字却返回上级）；空时返回上级
		if input := a.pickerInputRef(); input.text != nil && *input.text != "" {
			input.backspace()
			return a, nil
		}
		return a, a.pickerBackspace()
	case tea.KeyEnter:
		return a, a.pickerEnter()
	case tea.KeyCtrlJ:
		// 所有层：C-j 下移（与方向键/Down 等效）
		if a.pickerCursor < len(cands)-1 {
			a.pickerCursor++
		}
		return a, nil
	case tea.KeyCtrlK:
		// 所有层：C-k 上移（与方向键/Up 等效）
		if a.pickerCursor > 0 {
			a.pickerCursor--
		}
		return a, nil
	case tea.KeyCtrlL:
		// SSH 主机层：C-l 在主机输入/路径输入间切焦点
		if a.sourceTab == 2 && a.pickerSSHHost == "" {
			a.pickerSshFocus = (a.pickerSshFocus + 1) % 2
		}
		return a, nil
	case tea.KeySpace:
		// K8s 资源层多选
		if a.sourceTab == 0 && a.pickerK8sLevel == 2 && a.pickerCursor < len(cands) {
			v := cands[a.pickerCursor].value
			a.pickerChecked[v] = !a.pickerChecked[v]
		}
		return a, nil
	case tea.KeyRunes:
		// 选择器内按 q：直接退出 logview（日志页按 q 打开选择器，再按 q 退出）。
		// 输入框有内容时 q 作为过滤字符，避免误退。
		if string(msg.Runes) == "q" {
			if input := a.pickerInputRef(); input.text == nil || *input.text == "" {
				return a, a.shutdown()
			}
		}
		// 有输入框的层（ns/资源过滤/本地/ssh目录）：字母一律进输入框，避免名称含 j/k 丢字
		if input := a.pickerInputRef(); input.text != nil {
			input.handleEditKeys(msg)
			if n := len(a.visiblePickerCandidates()); a.pickerCursor >= n {
				a.pickerCursor = max(0, n-1)
			}
			return a, nil
		}
		// 无输入框的层（context 列表/ssh 主机候选）：j/k 移动
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
		}
	}

	// 输入框编辑（K8s ns/资源过滤、本地路径/过滤、SSH 主机/过滤）
	input := a.pickerInputRef()
	if input.text != nil {
		input.handleEditKeys(msg)
		// 输入改变列表后收敛光标到可见范围
		if n := len(a.visiblePickerCandidates()); a.pickerCursor >= n {
			a.pickerCursor = max(0, n-1)
		}
	}
	return a, nil
}

// pickerTabEnterCmd 切换 tab 后按需拉取候选。
func (a *App) pickerTabEnterCmd() tea.Cmd {
	a.pickerCursor = 0
	switch a.sourceTab {
	case 0:
		if a.pickerK8sLevel == 0 && a.pickerContexts == nil {
			a.pickerLoading = true
			return fetchK8sContextsCmd()
		}
		if a.pickerK8sLevel == 1 && a.pickerNamespaces == nil {
			a.pickerLoading = true
			return fetchK8sNamespacesCmd(a.pickerKubeCtx)
		}
		if a.pickerK8sLevel == 2 && a.pickerCandidates == nil {
			a.pickerLoading = true
			return fetchK8sCandidatesCmd(a.pickerKubeCtx, a.pickerNsInput)
		}
	case 2:
		if a.pickerSSHHost != "" && a.pickerCandidates == nil {
			a.pickerLoading = true
			return fetchSSHDirCmd(a.pickerSSHHost, a.pickerSSHDir, a.sshPw(a.pickerSSHHost))
		}
	}
	return nil
}

// pickerBackspace 逐级返回。
func (a *App) pickerBackspace() tea.Cmd {
	a.pickerCursor = 0
	a.pickerDirFilter = "" // 目录变化重置过滤
	a.pickerPathInput = ""
	switch a.sourceTab {
	case 0:
		if a.pickerK8sLevel > 0 {
			a.pickerK8sLevel--
			if a.pickerK8sLevel == 1 {
				a.pickerCandidates = nil
				a.pickerNsInput = "" // 清残留，避免误导
				a.pickerLoading = true
				return fetchK8sNamespacesCmd(a.pickerKubeCtx)
			}
		}
	case 1:
		parent := filepath.Dir(a.pickerLocalDir)
		if parent != a.pickerLocalDir {
			a.pickerLocalDir = parent
		}
	case 2:
		if a.pickerSSHHost != "" {
			if a.pickerSSHDir != a.pickerSSHRoot {
				a.pickerSSHDir = parentPath(a.pickerSSHDir)
				a.pickerCandidates = nil
				a.pickerLoading = true
				return fetchSSHDirCmd(a.pickerSSHHost, a.pickerSSHDir, a.sshPw(a.pickerSSHHost))
			}
			// 已在起始层：返回主机层
			a.pickerSSHHost = ""
			a.pickerSSHDir = ""
			a.pickerSSHRoot = ""
			a.pickerCandidates = nil
		}
	}
	return nil
}

// pickerEnter 按当前层级分派：下钻 / 勾选确认 / 打开文件。
func (a *App) pickerEnter() tea.Cmd {
	// 异步候选加载中：不分派（此时候选为空/过期，../ 等占位项会导致误退回上级）
	if a.pickerLoading {
		return nil
	}
	cands := a.visiblePickerCandidates()

	// 本地 tab：过滤框输入的是存在的路径时直达（目录进入/文件打开）
	if a.sourceTab == 1 && a.pickerDirFilter != "" {
		p := expandHome(a.pickerDirFilter)
		if st, err := os.Stat(p); err == nil {
			if st.IsDir() {
				a.pickerLocalDir = p
				a.pickerDirFilter = ""
				a.pickerCursor = 0
				return nil
			}
			a.pickerPathInput = p
			return a.confirmSourcePicker()
		}
	}

	// SSH 远程目录层：过滤框输入以 / 开头 → 视为路径直达（目录列出/文件打开）
	if a.sourceTab == 2 && a.pickerSSHHost != "" && strings.HasPrefix(a.pickerDirFilter, "/") {
		path := strings.TrimSuffix(a.pickerDirFilter, "/")
		if path == "" {
			path = "/"
		}
		_, err := stream.SSHListDir(a.pickerSSHHost, path, a.sshPw(a.pickerSSHHost))
		if err != nil {
			// 路径不存在或不可读：当作文件尝试打开
			a.pickerRemotePath = path
			a.pickerDirFilter = ""
			return a.confirmSourcePicker()
		}
		a.pickerSSHDir = path
		a.pickerCandidates = nil
		a.pickerCursor = 0
		a.pickerDirFilter = ""
		a.pickerLoading = true
		return fetchSSHDirCmd(a.pickerSSHHost, path, a.sshPw(a.pickerSSHHost))
	}

	if a.pickerCursor >= len(cands) {
		// ns 层越界（过滤后光标超出）：输入框有值时直达该 namespace
		if a.sourceTab == 0 && a.pickerK8sLevel == 1 {
			ns := strings.TrimSpace(a.pickerNsInput)
			if ns != "" {
				a.pickerK8sLevel = 2
				a.pickerCandidates = nil
				a.pickerCursor = 0
				a.pickerLoading = true
				a.pickerDirFilter = ""
				return fetchK8sCandidatesCmd(a.pickerKubeCtx, ns)
			}
		}
		return nil
	}
	cand := cands[a.pickerCursor]
	// 过滤态只剩 ../（无实际匹配）时 Enter 不退回上级（退回用 Backspace 或清空过滤）
	if cand.value == ".." && a.pickerDirFilter != "" && a.sourceTab != 0 {
		return nil
	}

	switch a.sourceTab {
	case 0:
		switch a.pickerK8sLevel {
		case 0: // context → 切换并进入 ns 层
			useCtx := a.k8sUseContextFn
			if useCtx == nil {
				useCtx = useK8sContext
			}
			if err := useCtx(cand.value); err != nil {
				a.appendErrorLine(fmt.Sprintf("切换 context 失败: %v", err))
				return nil
			}
			a.pickerKubeCtx = cand.value // 记录已选 context，建源时显式指定
			BumpUsage(usageK8sCtx + cand.value)
			a.pickerK8sLevel = 1
			a.pickerNamespaces = nil
			a.pickerCursor = 0
			a.pickerLoading = true
			return fetchK8sNamespacesCmd(a.pickerKubeCtx)
		case 1: // namespace → 进入资源层
			a.pickerNsInput = cand.value
			BumpUsage(usageK8sNS + cand.value)
			a.pickerK8sLevel = 2
			a.pickerCandidates = nil
			a.pickerCursor = 0
			a.pickerLoading = true
			a.pickerDirFilter = ""
			return fetchK8sCandidatesCmd(a.pickerKubeCtx, cand.value)
		default: // 资源层：Enter = 确认（勾选集合或光标项）
			return a.confirmSourcePicker()
		}
	case 1: // 本地
		target := cand.value
		if target == ".." {
			parent := filepath.Dir(a.pickerLocalDir)
			if parent != a.pickerLocalDir {
				a.pickerLocalDir = parent
			}
			a.pickerCursor = 0
			a.pickerDirFilter = ""
			return nil
		}
		if cand.dir {
			a.pickerLocalDir = filepath.Join(a.pickerLocalDir, target)
			a.pickerCursor = 0
			a.pickerDirFilter = ""
			return nil
		}
		a.pickerPathInput = filepath.Join(a.pickerLocalDir, target)
		return a.confirmSourcePicker()
	case 2: // SSH
		if a.pickerSSHHost == "" {
			// 主机层 → 进入远程目录浏览（从根目录开始，输入过滤/Backspace 逐级导航）
			BumpUsage(usageSSHHost + cand.value)
			a.pickerSSHHost = cand.value
			a.pickerSSHDir = "/"
			a.pickerSSHRoot = "/"
			a.pickerCandidates = nil
			a.pickerCursor = 0
			a.pickerLoading = true
			a.pickerDirFilter = ""
			return fetchSSHDirCmd(a.pickerSSHHost, a.pickerSSHDir, a.sshPw(a.pickerSSHHost))
		}
		if cand.value == ".." {
			if a.pickerSSHDir == "/" {
				// 根目录再上 → 返回主机层
				a.pickerSSHHost = ""
				a.pickerSSHDir = ""
				a.pickerSSHRoot = ""
				a.pickerCandidates = nil
				a.pickerCursor = 0
				return nil
			}
			a.pickerSSHDir = parentPath(a.pickerSSHDir)
			a.pickerCandidates = nil
			a.pickerCursor = 0
			a.pickerLoading = true
			a.pickerDirFilter = ""
			return fetchSSHDirCmd(a.pickerSSHHost, a.pickerSSHDir, a.sshPw(a.pickerSSHHost))
		}
		if cand.dir {
			a.pickerSSHDir = strings.TrimSuffix(a.pickerSSHDir, "/") + "/" + cand.value
			a.pickerCandidates = nil
			a.pickerCursor = 0
			a.pickerLoading = true
			a.pickerDirFilter = ""
			return fetchSSHDirCmd(a.pickerSSHHost, a.pickerSSHDir, a.sshPw(a.pickerSSHHost))
		}
		a.pickerRemotePath = strings.TrimSuffix(a.pickerSSHDir, "/") + "/" + cand.value
		return a.confirmSourcePicker()
	}
	return nil
}

// pickerInputRef 返回当前可编辑输入框（浏览态多数层无输入框）。
func (a *App) pickerInputRef() inputRef {
	switch a.sourceTab {
	case 0:
		if a.pickerK8sLevel == 1 {
			return inputRef{&a.pickerNsInput, &a.pickerNsCursor}
		}
		if a.pickerK8sLevel == 2 {
			return inputRef{&a.pickerDirFilter, &a.pickerFilterCursor} // 资源过滤
		}
	case 1:
		if a.pickerPathInput != "" {
			return inputRef{&a.pickerPathInput, &a.pickerPathCursor}
		}
		return inputRef{&a.pickerDirFilter, &a.pickerFilterCursor}
	case 2:
		if a.pickerSSHHost != "" {
			// 远程目录层：过滤框
			return inputRef{&a.pickerDirFilter, &a.pickerFilterCursor}
		}
		if a.pickerSshFocus == 0 {
			return inputRef{&a.pickerHostInput, &a.pickerHostCursor}
		}
		return inputRef{&a.pickerRemotePath, &a.pickerRemoteCursor}
	}
	return inputRef{}
}

// confirmSourcePicker 组装新 stream 并热切换。
func (a *App) confirmSourcePicker() tea.Cmd {
	tab := a.sourceTab
	cursor := a.pickerCursor
	checked := a.pickerChecked
	cands := a.visiblePickerCandidates() // 过滤后的可见候选（K8s 资源层/SSH 目录层受 filter 影响）
	nsInput := a.pickerNsInput
	kubeCtx := a.pickerKubeCtx
	pathInput := a.pickerPathInput
	hostInput := a.pickerHostInput
	remotePath := a.pickerRemotePath
	sshHost := a.pickerSSHHost
	sshDir := a.pickerSSHDir
	a.closeSourcePicker()

	switch tab {
	case 0: // K8s 多选
		var resources []string
		for v, ok := range checked {
			if ok {
				resources = append(resources, v)
			}
		}
		if len(resources) == 0 && cursor < len(cands) {
			resources = []string{cands[cursor].value}
		}
		if len(resources) == 0 {
			return nil
		}
		sortStrings(resources)
		// 热点：确认打开的资源/命名空间计入频次
		for _, r := range resources {
			BumpUsage(usageK8sRes + r)
		}
		if nsInput != "" {
			BumpUsage(usageK8sNS + nsInput)
		}
		var sources []*stream.K8sSource
		for _, r := range resources {
			sources = append(sources, stream.NewK8sSource(r, nsInput, nil, 200))
		}
		var src stream.LogStream
		if len(sources) == 1 {
			sources[0].SetContext(kubeCtx)
			src = sources[0]
		} else {
			m := stream.NewMultiK8sSource(sources)
			m.SetContext(kubeCtx)
			src = m
		}
		return a.ReplaceStream(src)
	case 1: // 本地文件
		path := strings.TrimSpace(pathInput)
		if path == "" {
			return nil
		}
		path = expandHome(path)
		return a.ReplaceStream(stream.NewFileSource([]string{path}))
	case 2: // SSH
		host := strings.TrimSpace(hostInput)
		if host == "" {
			host = sshHost
		}
		if host == "" {
			return nil
		}
		path := strings.TrimSpace(remotePath)
		if path == "" && sshDir != "" {
			path = sshDir
		}
		if path == "" {
			return nil
		}
		BumpUsage(usageSSHHost + host) // 热点：确认打开的 SSH 主机计入频次
		src := stream.NewSSHSource(host, path, 200)
	if pw := a.sshPw(host); pw != "" {
		src.SetPassword(pw)
	}
	return a.ReplaceStream(src)
	}
	return nil
}

// buildSourcePickerLines 渲染选择器（inline popup）。
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
		switch a.pickerK8sLevel {
		case 0:
			cur := a.pickerBreadcrumbCtx()
			head := PopupTabStyle.Render(fmt.Sprintf(" 当前context: %s", cur))
			content.WriteString(head + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 查询中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 无 context（检查 kubectl）") + "\n")
			} else {
				content.WriteString(renderCandidateListMark(cands, a.pickerCursor, cur))
			}
			content.WriteString("\n" + PopupTabStyle.Render(" Enter切换context C-j/k移动 Backspace退出 Esc取消"))
		case 1:
			content.WriteString(PopupTabStyle.Render(fmt.Sprintf(" context: %s", a.pickerBreadcrumbCtx())) + "\n")
			content.WriteString(a.inputLine(a.pickerNsInput, a.pickerNsCursor, "或输入 namespace 回车直达") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 查询中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 无 namespace") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8))
			}
			content.WriteString("\n" + PopupTabStyle.Render(" Enter选择ns C-j/k移动 Backspace返回 Esc取消"))
		default:
			content.WriteString(DetailLabelStyle.Render(fmt.Sprintf(" %s/%s", a.pickerBreadcrumbCtx(), a.pickerNsInput)) + "\n")
			content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤资源名…") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 查询中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 无资源（检查权限或过滤词）") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, a.pickerChecked, 8))
			}
			content.WriteString("\n" + PopupTabStyle.Render(" Space勾选(多选) Enter确认 C-j/k移动 Backspace返回 Esc取消"))
		}
	case 1:
		content.WriteString(DetailLabelStyle.Render(" "+a.pickerLocalDir) + "\n")
		content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤…") + "\n\n")
		if len(cands) == 0 {
			content.WriteString(PopupTabStyle.Render(" 目录为空或不可读") + "\n")
		} else {
			content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10))
		}
		content.WriteString("\n" + PopupTabStyle.Render(" 进目录:Enter 开文件:Enter C-j/k移动 Backspace返回 Esc取消"))
	case 2:
		if a.pickerSSHHost == "" {
			hostLine := a.inputLine(a.pickerHostInput, a.pickerHostCursor, "user@host 或选择候选")
			content.WriteString(DetailLabelStyle.Render("主机: ") + hostLine + "\n\n")
			if len(cands) > 0 {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8))
			} else if strings.TrimSpace(a.pickerHostInput) != "" {
				content.WriteString(PopupTabStyle.Render(" 无匹配主机，Enter 直连（C-k 切到路径输入）") + "\n")
			} else {
				content.WriteString(PopupTabStyle.Render(" 无主机候选（~/.ssh/config 为空）") + "\n")
			}
			content.WriteString("\n" + PopupTabStyle.Render(" Enter连接浏览 C-j/k移动 C-l切路径 Esc取消"))
		} else {
			content.WriteString(DetailLabelStyle.Render(fmt.Sprintf(" %s:%s", a.pickerSSHHost, a.pickerSSHDir)) + "\n")
			content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤…") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 加载中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 目录为空或不可读") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10))
			}
			content.WriteString("\n" + PopupTabStyle.Render(" 进目录:Enter 选文件:Enter C-j/k移动 Backspace返回 Esc取消"))
		}
	}

	boxW := min(64, a.width-4)
	box := PopupBoxStyle.Width(boxW).Render(content.String())
	return a.overlayToVL(box, vl)
}

// renderCandidateListMark 渲染 context 列表（当前 context 打 ✓ 标）。
func renderCandidateListMark(cands []sourceCandidate, cursor int, current string) string {
	var b strings.Builder
	start, end := scrollWindow(len(cands), cursor, 8)
	for i := start; i < end; i++ {
		prefix := "  "
		if i == cursor {
			prefix = SelectedStyle.Render(" >")
		}
		mark := "  "
		if cands[i].value == current {
			mark = DetailLabelStyle.Render("✓ ")
		}
		b.WriteString(prefix + " " + mark + DetailValueStyle.Render(cands[i].label) + "\n")
	}
	if len(cands) > end {
		b.WriteString(PopupTabStyle.Render(fmt.Sprintf("   …共%d项", len(cands))) + "\n")
	}
	return b.String()
}

// renderCandidateList 渲染候选列表（可滚动窗口，勾选态仅 k8s 多选用）。
func renderCandidateList(cands []sourceCandidate, cursor int, checked map[string]bool, maxRows int) string {
	var b strings.Builder
	start, end := scrollWindow(len(cands), cursor, maxRows)
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

// scrollWindow 计算滚动窗口起止。
func scrollWindow(n, cursor, maxRows int) (int, int) {
	start := 0
	if cursor >= maxRows {
		start = cursor - maxRows + 1
	}
	end := start + maxRows
	if end > n {
		end = n
	}
	return start, end
}

// parentPath 返回 path 的上级目录路径。
func parentPath(p string) string {
	p = strings.TrimSuffix(p, "/")
	if idx := strings.LastIndex(p, "/"); idx > 0 {
		return p[:idx]
	}
	return "/"
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func expandHome(p string) string {
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return strings.Replace(p, "~", home, 1)
		}
	}
	return p
}
