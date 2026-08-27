package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/frp"
	"github.com/justfun/logview/internal/stream"
)

// 源选择器（o 键）— 浏览式交互：
//   K8s tab:   context → namespace → 资源多选（Space 勾选，Enter 确认）
//   本地 tab:  目录浏览器（Enter 进目录/开文件，Backspace 返回上级）
//   SSH tab:   主机列表 → 远程目录浏览器
//   FRP tab:   连接列表（+ 新建连接 / 已存记录，搜索过滤）
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
	// FRP 浏览状态（不重置 pickerFRPTunnel：密码流程重开 picker 需保留，清理统一走 closeSourcePicker）
	a.pickerFRPLevel = 0
	a.pickerFRPStep = 0
	a.pickerFRPInput = ""
	a.pickerFRPCursor = 0
	a.pickerFRPServerName = ""
	a.pickerFRPServerAddr = ""
	a.pickerFRPSK = ""
	a.pickerFRPProxy = ""
	a.pickerFRPUser = ""
	a.pickerFRPDir = ""
	a.pickerFRPConnName = ""
}

func (a *App) closeSourcePicker() {
	a.sourcePickerMode = false
	a.pickerCandidates = nil
	a.pickerChecked = nil
	// 未移交的 frp 隧道随弹窗关闭清理
	if a.pickerFRPTunnel != nil {
		a.pickerFRPTunnel.Cleanup()
		a.pickerFRPTunnel = nil
	}
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
	case 1: // 本地目录（. 开头过滤词显示隐藏文件；无过滤时 ../ 返回上级，有过滤时隐藏）
		filter := a.pickerDirFilter
		showHidden := strings.HasPrefix(filter, ".")
		var items []sourceCandidate
		if filter == "" {
			items = append(items, sourceCandidate{label: "../", value: "..", dir: true})
		}
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
	case 3: // FRP
		switch a.pickerFRPLevel {
		case 0:
			return frpConnCandidates(a.pickerFRPInput)
		case 1:
			if a.pickerFRPStep == 0 {
				return frpServerCandidates(a.pickerFRPInput)
			}
			return nil
		default:
			return a.filteredFRPCands()
		}
	}
	return nil
}

// filteredSSHCands 远程目录层：无过滤时首位 ../ 返回上级；有过滤时隐藏 ../，
// 光标默认落在第一个匹配项（搜索场景直接 Enter 即下钻/打开）。
func (a *App) filteredSSHCands() []sourceCandidate {
	filter := strings.ToLower(a.pickerDirFilter)
	var items []sourceCandidate
	if filter == "" && a.pickerSSHDir != "/" {
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
		a.sourceTab = (a.sourceTab + 1) % 4
		return a, a.pickerTabEnterCmd()
	case tea.KeyShiftTab:
		a.sourceTab = (a.sourceTab + 3) % 4
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
	case tea.KeyCtrlX:
		return a, a.pickerDeleteFRP(cands)
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
		before := *input.text
		input.handleEditKeys(msg)
		// 过滤词从非空变空（删完/C-u）：光标归 0（../ 等导航项回到首位）
		if before != "" && *input.text == "" {
			a.pickerCursor = 0
			return a, nil
		}
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

// frpCurName 当前 frp 连接名（记录名或 proxy 名）。
func (a *App) frpCurName() string {
	if a.pickerFRPConnName != "" {
		return a.pickerFRPConnName
	}
	return a.pickerFRPProxy
}

// frpPwKey frp 密码内存缓存 key。
func (a *App) frpPwKey() string { return "frp:" + a.frpCurName() }

// closeFRPBrowse 退出目录浏览：清隧道回连接列表。
func (a *App) closeFRPBrowse() {
	if a.pickerFRPTunnel != nil {
		a.pickerFRPTunnel.Cleanup()
		a.pickerFRPTunnel = nil
	}
	a.pickerFRPLevel = 0
	a.pickerFRPDir = ""
	a.pickerCandidates = nil
	a.pickerCursor = 0
}

// filteredFRPCands FRP 远程目录层候选（同 SSH：无过滤首位 ../，根目录除外）。
func (a *App) filteredFRPCands() []sourceCandidate {
	filter := strings.ToLower(a.pickerDirFilter)
	var items []sourceCandidate
	if filter == "" && a.pickerFRPDir != "/" {
		items = append(items, sourceCandidate{label: "../", value: "..", dir: true})
	}
	for _, c := range a.pickerCandidates {
		if filter == "" || strings.Contains(strings.ToLower(c.label), filter) {
			items = append(items, c)
		}
	}
	return items
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
	case 3: // FRP：表单内逐级返回（step0 时返回 L0）；L2 目录逐级上翻
		switch a.pickerFRPLevel {
		case 1:
			a.pickerFRPInput = ""
			if a.pickerFRPStep > 0 {
				// 选已存服务器直达 step3 时未走过 step1/2：返回直接回 step0
				if a.pickerFRPStep == 3 && a.pickerFRPServerAddr == "" {
					a.pickerFRPStep = 0
				} else {
					a.pickerFRPStep--
				}
			} else {
				a.pickerFRPLevel = 0
			}
		case 2:
			if a.pickerFRPTunnel == nil {
				a.pickerFRPLevel = 0
				return nil
			}
			if a.pickerFRPDir != "/" {
				a.pickerFRPDir = parentPath(a.pickerFRPDir)
				a.pickerCandidates = nil
				a.pickerLoading = true
				return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPTunnel.LocalPort(), a.pickerFRPDir, a.sshPasswords[a.frpPwKey()])
			}
			a.closeFRPBrowse()
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

	// FRP 目录层：过滤框输入以 / 开头 → 路径直达（同 SSH）
	if a.sourceTab == 3 && a.pickerFRPLevel == 2 && a.pickerFRPTunnel != nil && strings.HasPrefix(a.pickerDirFilter, "/") {
		path := strings.TrimSuffix(a.pickerDirFilter, "/")
		if path == "" {
			path = "/"
		}
		if _, err := stream.SSHListDirWithPort(a.pickerFRPUser+"@127.0.0.1", a.pickerFRPTunnel.LocalPort(), path, a.sshPasswords[a.frpPwKey()]); err != nil {
			// 路径不存在或不可读：当作文件尝试打开
			a.pickerRemotePath = path
			a.pickerDirFilter = ""
			return a.confirmFRPPicker()
		}
		a.pickerFRPDir = path
		a.pickerCandidates = nil
		a.pickerCursor = 0
		a.pickerDirFilter = ""
		a.pickerLoading = true
		return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPTunnel.LocalPort(), path, a.sshPasswords[a.frpPwKey()])
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
		// FRP 表单输入步（step1-5 无候选列表）：输入框即提交值
		if a.sourceTab == 3 && a.pickerFRPLevel == 1 {
			return a.pickerFRPFormEnter(sourceCandidate{})
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
	case 3: // FRP
		switch a.pickerFRPLevel {
		case 0:
			if cand.value == "+new" {
				a.pickerFRPLevel = 1
				a.pickerFRPStep = 0
				a.pickerFRPInput = ""
				a.pickerFRPConnName = "" // 清残留：重新表单建档时旧记录名必须作废（终审 F1）
				a.pickerCursor = 0
				return nil
			}
			conn, ok := frpStore().FindConn(cand.value)
			if !ok {
				return nil
			}
			server, ok := frpStore().FindServer(conn.Server)
			if !ok {
				a.appendErrorLine(fmt.Sprintf("frp 记录 %s 引用的服务器 %s 不存在", conn.Name, conn.Server))
				return nil
			}
			a.pickerFRPConnName = conn.Name
			a.pickerFRPUser = conn.User
			a.pickerFRPSK = conn.SK
			a.pickerFRPProxy = conn.Proxy
			a.pickerFRPServerName = conn.Server
			a.pickerLoading = true
			return fetchFRPTunnelCmd(server, conn)
		case 1:
			return a.pickerFRPFormEnter(cand)
		default: // 远程目录浏览
			if a.pickerFRPTunnel == nil {
				return nil
			}
			if cand.value == ".." {
				if a.pickerFRPDir == "/" {
					a.closeFRPBrowse()
					return nil
				}
				a.pickerFRPDir = parentPath(a.pickerFRPDir)
				a.pickerCandidates = nil
				a.pickerCursor = 0
				a.pickerLoading = true
				return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPTunnel.LocalPort(), a.pickerFRPDir, a.sshPasswords[a.frpPwKey()])
			}
			if cand.dir {
				a.pickerFRPDir = strings.TrimSuffix(a.pickerFRPDir, "/") + "/" + cand.value
				a.pickerCandidates = nil
				a.pickerCursor = 0
				a.pickerLoading = true
				a.pickerDirFilter = ""
				return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPTunnel.LocalPort(), a.pickerFRPDir, a.sshPasswords[a.frpPwKey()])
			}
			a.pickerRemotePath = strings.TrimSuffix(a.pickerFRPDir, "/") + "/" + cand.value
			return a.confirmFRPPicker() // Task 8 实现
		}
	}
	return nil
}

// pickerFRPFormEnter 表单各步骤 Enter 提交（step5 提交建隧道由后续任务接管）。
func (a *App) pickerFRPFormEnter(cand sourceCandidate) tea.Cmd {
	input := strings.TrimSpace(a.pickerFRPInput)
	next := func(step int) {
		a.pickerFRPStep = step
		a.pickerFRPInput = ""
		a.pickerCursor = 0
	}
	switch a.pickerFRPStep {
	case 0: // 选服务器
		if cand.value == "+manual" {
			next(1)
		} else {
			a.pickerFRPServerName = cand.value
			a.pickerFRPServerAddr = "" // 清残留：手动输入中途返回后残留会使 step3 回退跳级失效
			next(3) // 已存服务器有 token，直接到 sk
		}
	case 1: // 新服务器地址
		if input == "" {
			return nil
		}
		a.pickerFRPServerAddr = input
		a.pickerFRPServerName = input // 默认名 = 地址
		next(2)
	case 2: // token（可空）
		frpStore().UpsertServer(frp.Server{
			Name:  a.pickerFRPServerName,
			Addr:  a.pickerFRPServerAddr,
			Token: input,
		})
		if err := frpStore().Save(); err != nil {
			a.appendErrorLine(fmt.Sprintf("frp 服务器保存失败: %v", err))
		}
		next(3)
	case 3: // sk
		if input == "" {
			return nil
		}
		a.pickerFRPSK = input
		next(4)
	case 4: // proxy
		if input == "" {
			return nil
		}
		a.pickerFRPProxy = input
		next(5)
	case 5: // user → 提交建隧道
		if input == "" {
			return nil
		}
		a.pickerFRPUser = input
		a.pickerFRPInput = ""
		server, ok := frpStore().FindServer(a.pickerFRPServerName)
		if !ok {
			a.appendErrorLine(fmt.Sprintf("frp 服务器 %s 不存在", a.pickerFRPServerName))
			return nil
		}
		conn := frp.Conn{Name: a.pickerFRPProxy, Server: a.pickerFRPServerName,
			SK: a.pickerFRPSK, Proxy: a.pickerFRPProxy, User: input}
		// 隧道一打通就先存记录：中途放弃（Esc/Backspace/退出）不丢配置，
		// L0 可直接重选（此前只在选文件确认时保存，没走到那步配置就丢）
		if old, ok := frpStore().FindConn(conn.Name); ok && old.Path != "" {
			conn.Path = old.Path // 同名重建：保留已确认的日志路径
		}
		frpStore().UpsertConn(conn)
		if err := frpStore().Save(); err != nil {
			a.appendErrorLine(fmt.Sprintf("frp 记录保存失败: %v", err))
		}
		a.pickerLoading = true
		return fetchFRPTunnelCmd(server, conn)
	}
	return nil
}

// confirmFRPPicker FRP 确认建流：保存记录（含 Path）→ 隧道移交 FRPSource。
func (a *App) confirmFRPPicker() tea.Cmd {
	tunnel := a.pickerFRPTunnel
	if tunnel == nil {
		return nil
	}
	path := strings.TrimSpace(a.pickerRemotePath)
	if path == "" && a.pickerFRPDir != "" {
		path = a.pickerFRPDir
	}
	if path == "" {
		return nil
	}
	name := a.pickerFRPConnName
	if name == "" {
		name = a.pickerFRPProxy
	}
	user := a.pickerFRPUser
	frpStore().UpsertConn(frp.Conn{
		Name: name, Server: a.pickerFRPServerName, SK: a.pickerFRPSK,
		Proxy: a.pickerFRPProxy, User: user, Path: path,
	})
	if err := frpStore().Save(); err != nil {
		a.appendErrorLine(fmt.Sprintf("frp 记录保存失败: %v", err))
	}
	BumpUsage(usageFRPConn + name)
	a.pickerFRPTunnel = nil // 隧道移交 FRPSource，防 closeSourcePicker 清理
	a.closeSourcePicker()
	src := stream.NewFRPSource(name, tunnel, user, path, 200)
	if pw := a.sshPasswords["frp:"+name]; pw != "" {
		src.SetPassword(pw)
	}
	return a.ReplaceStream(src)
}

// frpBrowseRoot 目录浏览起始目录：已存 Path 取父目录（/var/log/a.log → /var/log），无 Path 从根开始。
func frpBrowseRoot(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "/"
	}
	return parentPath(path)
}

// firstErrLine 错误首行（无错误返回空串；多行 ssh stderr 只取首行，弹窗内单行够用）。
func firstErrLine(err error) string {
	if err == nil {
		return ""
	}
	s := strings.TrimSpace(err.Error())
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return s
}

// frpConnLevelErrs ssh 层连接失败特征：TCP/握手断，真因在 frpc 日志（proxy 不存在/sk 错/远端掉线）。
var frpConnLevelErrs = []string{
	"connection reset",
	"kex_exchange_identification",
	"connection refused",
	"connection closed",
	"broken pipe",
	"connection timed out",
}

// frpDirErrText frp 目录拉取失败的显示文本：连接层错误附带 frpc 日志尾 + 常见真因提示。
func (a *App) frpDirErrText(err error) string {
	s := firstErrLine(err)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	for _, pat := range frpConnLevelErrs {
		if !strings.Contains(lower, pat) {
			continue
		}
		if lg, ok := a.pickerFRPTunnel.(interface{ RecentLog() string }); ok {
			if tail := strings.TrimSpace(lg.RecentLog()); tail != "" {
				s += " | frpc: " + tail
			}
		}
		break
	}
	// 真因翻译：visitor 在 frps 上找不到目标 stcp proxy（名字不一致或远端 frpc 掉线）
	if strings.Contains(s, "custom listener for") {
		s += " （proxy 名在 frps 上不存在：核对远端 frpc 的 proxies.name 或远端是否在线）"
	}
	return s
}

// pickerDeleteFRP C-x 删除：FRP L0 删连接记录，L1 step0 删服务器（被引用时拒绝）。
// 删除后持久化并收敛光标；+new/+manual 占位项不可删。
func (a *App) pickerDeleteFRP(cands []sourceCandidate) tea.Cmd {
	if a.sourceTab != 3 || a.pickerCursor >= len(cands) {
		return nil
	}
	cand := cands[a.pickerCursor]
	if a.pickerFRPLevel == 0 {
		if cand.value == "+new" {
			return nil
		}
		if !frpStore().DeleteConn(cand.value) {
			return nil
		}
	} else if a.pickerFRPLevel == 1 && a.pickerFRPStep == 0 {
		if cand.value == "+manual" {
			return nil
		}
		if err := frpStore().DeleteServer(cand.value); err != nil {
			a.appendErrorLine(fmt.Sprintf("frp 服务器删除失败: %v", err))
			return nil
		}
	} else {
		return nil
	}
	if err := frpStore().Save(); err != nil {
		a.appendErrorLine(fmt.Sprintf("frp 配置保存失败: %v", err))
	}
	if n := len(a.visiblePickerCandidates()); a.pickerCursor >= n {
		a.pickerCursor = max(0, n-1)
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
	case 3:
		if a.pickerFRPLevel == 2 {
			return inputRef{&a.pickerDirFilter, &a.pickerFilterCursor}
		}
		return inputRef{&a.pickerFRPInput, &a.pickerFRPCursor}
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

	tabs := []string{"K8s", "本地", "SSH", "FRP"}
	var tabParts []string
	for i, t := range tabs {
		if i == a.sourceTab {
			tabParts = append(tabParts, PopupActiveTabStyle.Render(" "+t+" "))
		} else {
			tabParts = append(tabParts, PopupTabStyle.Render(" "+t+" "))
		}
	}
	content.WriteString(strings.Join(tabParts, "  ") + "\n" + popupTabSep(min(64, a.width-4)) + "\n\n")

	cands := a.visiblePickerCandidates()
	switch a.sourceTab {
	case 0:
		switch a.pickerK8sLevel {
		case 0:
			cur := a.pickerBreadcrumbCtx()
			head := BreadcrumbStyle.Render(" ctx: " + cur + " ")
			content.WriteString(" " + head + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(keycapHint(" 查询中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(keycapHint(" 无 context（检查 kubectl）") + "\n")
			} else {
				content.WriteString(renderCandidateListMark(cands, a.pickerCursor, cur))
			}
		case 1:
			content.WriteString(" " + BreadcrumbStyle.Render(" ctx: "+a.pickerBreadcrumbCtx()+" ") + "\n")
			content.WriteString(a.inputLine(a.pickerNsInput, a.pickerNsCursor, "或输入 namespace 回车直达") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(keycapHint(" 查询中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(keycapHint(" 无 namespace") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8))
			}
		default:
			content.WriteString(" " + BreadcrumbStyle.Render(" "+a.pickerBreadcrumbCtx()+"/"+a.pickerNsInput+" ") + "\n")
			content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤资源名…") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(keycapHint(" 查询中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(keycapHint(" 无资源（检查权限或过滤词）") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, a.pickerChecked, 8))
			}
		}
	case 1:
		content.WriteString(" " + breadcrumbChain(a.pickerLocalDir) + "\n")
		content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤…") + "\n\n")
		if len(cands) == 0 {
			content.WriteString(keycapHint(" 目录为空或不可读") + "\n")
		} else {
			content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10))
		}
	case 2:
		if a.pickerSSHHost == "" {
			hostLine := a.inputLine(a.pickerHostInput, a.pickerHostCursor, "user@host 或选择候选")
			content.WriteString(DetailLabelStyle.Render("主机: ") + hostLine + "\n\n")
			if len(cands) > 0 {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8))
			} else if strings.TrimSpace(a.pickerHostInput) != "" {
				content.WriteString(keycapHint(" 无匹配主机，Enter 直连（C-k 切到路径输入）") + "\n")
			} else {
				content.WriteString(keycapHint(" 无主机候选（~/.ssh/config 为空）") + "\n")
			}
		} else {
			content.WriteString(" " + BreadcrumbStyle.Render(" "+a.pickerSSHHost+" ") + breadcrumbChain(a.pickerSSHDir) + "\n")
			content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤…") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(keycapHint(" 加载中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(keycapHint(" 目录为空或不可读") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10))
			}
		}
	case 3: // FRP
		switch a.pickerFRPLevel {
		case 0: // L0：连接列表
			content.WriteString(a.inputLine(a.pickerFRPInput, a.pickerFRPCursor, "搜索连接（名称/proxy/路径）…") + "\n\n")
			if len(cands) == 0 {
				content.WriteString(keycapHint(" 无保存的连接") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10))
			}
		case 1: // L1：新建表单（逐字段单输入框）
		prompts := []string{
			"选择 frps 服务器", "新服务器地址 host:port", "token（可空）",
			"sk (secret key，与远端 stcp 的 sk 一致)",
			"proxy 名称（远端 stcp 服务名，即 server-name）", "ssh 用户名",
		}
			content.WriteString(DetailLabelStyle.Render(prompts[a.pickerFRPStep]+": ") +
				a.inputLine(a.pickerFRPInput, a.pickerFRPCursor, "") + "\n\n")
			if a.pickerFRPStep == 0 {
				if len(cands) == 0 {
					content.WriteString(keycapHint(" 无保存的服务器") + "\n")
				} else {
					content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8))
				}
			}
		default: // L2：远程目录浏览
			content.WriteString(" " + BreadcrumbStyle.Render(" frp:"+a.frpCurName()+" ") + breadcrumbChain(a.pickerFRPDir) + "\n")
			content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤…") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(keycapHint(" 加载中…") + "\n")
			} else if len(cands) == 0 && a.pickerFRPErr != "" {
				content.WriteString(keycapHint(" 拉取失败: "+a.pickerFRPErr) + "\n")
			} else if len(cands) == 0 {
				content.WriteString(keycapHint(" 目录为空或不可读") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10))
			}
		}
	}

	boxW := min(64, a.width-4)
	box := PopupBoxStyle.Width(boxW).Render(content.String())
	return a.overlayToVL(box, vl)
}

// breadcrumbChain 把路径拆成徽章链：/var/log/app → [ / ][ var ][ log ][ app ]。
func breadcrumbChain(p string) string {
	parts := []string{BreadcrumbStyle.Render(" / ")}
	for _, s := range strings.Split(strings.Trim(p, "/"), "/") {
		if s == "" {
			continue
		}
		parts = append(parts, BreadcrumbStyle.Render(" "+s+" "))
	}
	return strings.Join(parts, " ")
}

// renderCandidateListMark 渲染 context 列表（当前 context 打 ✓ 标）。
func renderCandidateListMark(cands []sourceCandidate, cursor int, current string) string {
	var b strings.Builder
	start, end := scrollWindow(len(cands), cursor, 8)
	for i := start; i < end; i++ {
		prefix := "  "
		labelStyle := candidateLabelStyle(cands[i].label)
		if i == cursor {
			prefix = SelArrowStyle.Render("▶ ")
			labelStyle = SelArrowStyle // 光标行文字高亮（橙色粗体，与 ▶ 同色系）
		}
		mark := "  "
		if cands[i].value == current {
			mark = DetailLabelStyle.Render("✓ ")
		}
		b.WriteString(prefix + " " + mark + labelStyle.Render(cands[i].label) + "\n")
	}
	if len(cands) > end {
		b.WriteString(keycapHint(fmt.Sprintf("   …共%d项", len(cands))) + "\n")
	}
	return b.String()
}

// candidateLabelStyle 目录（"xxx/" 后缀）用蓝色与文件区分，增强弹窗层次。
func candidateLabelStyle(label string) lipgloss.Style {
	if strings.HasSuffix(label, "/") || label == "../" {
		return TraceIDStyle
	}
	return DetailValueStyle
}

// renderCandidateList 渲染候选列表（可滚动窗口，勾选态仅 k8s 多选用）。
func renderCandidateList(cands []sourceCandidate, cursor int, checked map[string]bool, maxRows int) string {
	var b strings.Builder
	start, end := scrollWindow(len(cands), cursor, maxRows)
	for i := start; i < end; i++ {
		prefix := "  "
		labelStyle := candidateLabelStyle(cands[i].label)
		mark := "  "
		if checked != nil && checked[cands[i].value] {
			mark = DetailLabelStyle.Render("✓ ")
		}
		if i == cursor {
			prefix = SelArrowStyle.Render("▶ ")
			labelStyle = SelArrowStyle // 光标行文字高亮（橙色粗体，与 ▶ 同色系）
		}
		b.WriteString(prefix + " " + mark + labelStyle.Render(cands[i].label) + "\n")
	}
	if len(cands) > end {
		b.WriteString(keycapHint(fmt.Sprintf("   …共%d项", len(cands))) + "\n")
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
