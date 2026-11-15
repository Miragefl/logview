package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Miragefl/logview/internal/frp"
	"github.com/Miragefl/logview/internal/stream"
)

// 源选择器（o 键）— 浏览式交互：
//   K8s tab:   context → namespace → 资源多选（Space 勾选，Enter 确认）
//   本地 tab:  目录浏览器（Enter 进目录/开文件，Backspace 返回上级）
//   SSH tab:   主机列表 → 远程目录浏览器
//   FRP tab:   连接列表（+ 新建连接 / 已存记录，搜索过滤）
// Backspace 逐级返回；手动输入仍保留（路径/ns 直达）。

// openSourcePicker 打开选择器（tab 可预选；不可见时钳制到首个可见 tab）。
func (a *App) openSourcePicker(tab int) {
	a.sourceTab = a.clampSourceTab(tab)
	a.sourcePickerMode = true
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
	// FRP 浏览状态（不重置 pickerFRPTunnel：密码流程重开 picker 需保留，清理统一走 closeSourcePicker）
	a.pickerFRPLevel = 0
	a.pickerFRPStep = 0
	a.pickerFRPInput = ""
	a.pickerFRPCursor = 0
	a.pickerFRPFormType = ""
	a.pickerFRPServerAddr = ""
	a.pickerFRPToken = ""
	a.pickerFRPSecretKey = ""
	a.pickerFRPSName = ""
	a.pickerFRPRemotePort = 0
	a.pickerFRPUser = ""
	a.pickerFRPAlias = ""
	a.pickerFRPDir = ""
	a.pickerFRPConnName = ""
	a.pickerSSHInputMode = false // tui-picker-unify:SSH 默认过滤态(C-a 进输入态)
}

// clampSourceTab tab 不在可见集时回落首个可见 tab（可见集恒含本地 tab 1）。
func (a *App) clampSourceTab(tab int) int {
	for _, t := range a.availableTabs {
		if t == tab {
			return tab
		}
	}
	if len(a.availableTabs) == 0 {
		return 1 // 防御：NewApp 未初始化/空集时至少本地可用
	}
	return a.availableTabs[0]
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
	a.pickerFRPHost = ""
	a.pickerFRPPort = 0
	a.pickerFRPCur = nil
}

// filterCands 子序列过滤(空词原样返回;大小写不敏感,tui-picker-unify)。
func filterCands(cands []sourceCandidate, kw string) []sourceCandidate {
	if strings.TrimSpace(kw) == "" {
		return cands
	}
	var out []sourceCandidate
	for _, c := range cands {
		if isSubseq(c.label, kw) {
			out = append(out, c)
		}
	}
	return out
}

// visiblePickerCandidates 当前 tab/level 的可见候选。
func (a *App) visiblePickerCandidates() []sourceCandidate {
	switch a.sourceTab {
	case 0: // K8s
		switch a.pickerK8sLevel {
		case 0:
			return filterCands(a.pickerContexts, a.pickerDirFilter) // tui-picker-unify:无框子序列过滤
		case 1:
			// ns 层按输入前缀过滤（输入即选中）
			filter := strings.TrimSpace(a.pickerNsInput)
			if filter == "" {
				return a.pickerNamespaces
			}
			return filterCands(a.pickerNamespaces, filter) // 子序列(tui-picker-unify)
		default:
			return filterCands(a.pickerCandidates, a.pickerDirFilter) // 子序列(tui-picker-unify)
		}
	case 1: // 本地目录（. 开头过滤词显示隐藏文件；无过滤时 ../ 返回上级，有过滤时隐藏）
		filter := a.pickerDirFilter
		showHidden := strings.HasPrefix(filter, ".")
		var items []sourceCandidate
		if filter == "" {
			items = append(items, sourceCandidate{label: "../", value: "..", dir: true})
		}
		for _, e := range listLocalDir(a.pickerLocalDir, showHidden) {
			if filter != "" && !isSubseq(e.name, filter) {
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
			// tui-picker-unify:主机层无框子序列过滤(A3 表单态=全列表,打字进表单框)
			return filterCands(sshCandidates(), a.pickerDirFilter)
		}
		return a.filteredSSHCands() // 远程目录层
	case 3: // FRP
		switch a.pickerFRPLevel {
		case 0:
			return frpConnCandidates(a.pickerDirFilter) // tui-picker-unify:词源统一 pickerDirFilter
		case 1:
			if a.pickerFRPStep == 0 { // type 选择（候选列表；其余步骤纯输入框）
				return frpTypeCandidates()
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
	filter := strings.TrimSpace(a.pickerDirFilter)
	var items []sourceCandidate
	if filter == "" && a.pickerSSHDir != "/" {
		items = append(items, sourceCandidate{label: "../", value: "..", dir: true})
	}
	for _, c := range a.pickerCandidates {
		if filter == "" || isSubseq(c.label, filter) { // 子序列(tui-picker-unify Final Review I4)
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
		// tui-picker-unify:Esc 逐级(fview 同款)——SSH 输入态退过滤态 → 清过滤词
		// → 层内回退(FRP L2→L0/K8s 资源→ns→ctx) → 顶层关 picker。
		if a.sourceTab == 2 && a.pickerSSHInputMode {
			a.pickerSSHInputMode = false
			a.pickerHostInput = ""
			return a, nil
		}
		// 清词统一"当前层 inputRef 词非空→清"(Final Review I3:ns 层词在
		// NsInput,原 tab!=0 特判漏掉它→Esc 跳层+幽灵复燃)
		if input := a.pickerInputRef(); input.text != nil && *input.text != "" {
			*input.text = ""
			a.pickerCursor = 0
			return a, nil
		}
		// 层级回退一步(fview BrowseDialog 准绳:Esc 不逐目录上翻——那是
		// Backspace/C-h 的活;此处只回 K8s level/表单 step/SSH 主机层/FRP L0)
		if cmd, handled := a.pickerLevelBack(); handled {
			return a, cmd
		}
		a.closeSourcePicker()
		return a, nil
	case tea.KeyTab:
		a.sourceTab = a.cycleSourceTab(1)
		return a, a.pickerTabEnterCmd()
	case tea.KeyShiftTab:
		a.sourceTab = a.cycleSourceTab(-1)
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
		cmd, _ := a.pickerBackspace()
		return a, cmd
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
	case tea.KeyCtrlD: // tui-picker-unify:C-d 删除(fview 键位;C-x 退役)
		if a.sourceTab == 2 && a.pickerSSHHost == "" && !a.pickerSSHInputMode {
			// keybind-unify-r2 A3：删除选中主机（仅 rules 源，发现源跳过）
			if a.pickerCursor < len(cands) {
				a.deleteSSHHost(cands[a.pickerCursor].value)
			}
			return a, nil
		}
		return a, a.pickerDeleteFRP(cands)
	case tea.KeyCtrlE: // tui-picker-unify:C-e 编辑(FRP 回填表单;SSH 无 store 搁置)
		if a.sourceTab == 3 && a.pickerFRPLevel == 0 {
			a.pickerEditFRP(cands)
			return a, nil
		}
		// 无编辑语义的层转发输入编辑(C-e=行尾)——表单层吞键是既有缺陷,
		// frp-edit-prefill 预填光标常态在尾后被放大暴露(Final Review Important-1)
		if input := a.pickerInputRef(); input.text != nil {
			input.handleEditKeys(msg)
		}
		return a, nil
	case tea.KeyCtrlA: // tui-picker-unify:C-a 新增统一(FRP 空表单/SSH 主机输入态)
		if a.sourceTab == 3 && a.pickerFRPLevel == 0 {
			a.pickerNewFRPForm()
			return a, nil
		}
		if a.sourceTab == 2 && a.pickerSSHHost == "" && !a.pickerSSHInputMode {
			// Final Review I3：仅主机列表层开表单——目录浏览层表单无渲染路径
			// （隐形态：helpbar 谎报 Enter=保存、Esc 被吃、打字进过滤词）
			a.pickerSSHInputMode = true // keybind-unify-r2 A3：新增主机表单（fview 同款单字段保存）
			a.pickerHostInput = ""
			return a, nil
		}
		// 无新增语义的层转发输入编辑(C-a=行首,表单内改前缀/端口的刚需)
		if input := a.pickerInputRef(); input.text != nil {
			input.handleEditKeys(msg)
		}
		return a, nil
	case tea.KeyCtrlC: // tui-picker-unify:K8s C-c 直达 ctx 层(C-cC-n=fview 切 context/ns 的层级直达形态)
		if a.sourceTab == 0 {
			a.pickerK8sLevel = 0
			a.pickerDirFilter = ""
			a.pickerCursor = 0
		}
		return a, nil
	case tea.KeyCtrlN:
		if a.sourceTab == 0 && a.pickerKubeCtx != "" {
			a.pickerK8sLevel = 1
			a.pickerDirFilter = ""
			a.pickerCursor = 0
			if a.pickerNamespaces == nil {
				a.pickerLoading = true
				return a, fetchK8sNamespacesCmd(a.pickerKubeCtx)
			}
		}
		return a, nil
	case tea.KeyCtrlH:
		// 目录浏览层 C-h 回上级（keybind-unify-r2 B1，fview BrowseDialog 同款）：
		// 本地/SSH 目录/FRP L2 词非空也直接回（词随目录切换清空，区别于
		// Backspace 词非空只删词）；其余层（K8s/主机列表/FRP L0·表单）不响应。
		if a.sourceTab == 1 ||
			(a.sourceTab == 2 && a.pickerSSHHost != "") ||
			(a.sourceTab == 3 && a.pickerFRPLevel == 2) {
			if cmd, handled := a.pickerBackspace(); handled {
				return a, cmd
			}
			return a, nil
		}
	case tea.KeySpace:
		// K8s 资源层多选
		if a.sourceTab == 0 && a.pickerK8sLevel == 2 && a.pickerCursor < len(cands) {
			v := cands[a.pickerCursor].value
			a.pickerChecked[v] = !a.pickerChecked[v]
		}
		return a, nil
	case tea.KeyRunes:
		// 选择器内按 q：关 picker 回主界面（keybind-unify-r2 A1，fview 准绳：
		// q 永不杀程序，退出仅 C-c）。
		// 输入框有内容时 q 作为过滤字符，避免误关。
		// FRP 新建表单（level 1）除外：字段每步从空开始，空输入框按 q 是打字
		// 常态（qq.example.com 首字母），不得关表单——q 一律进输入框。
		if string(msg.Runes) == "q" && !(a.sourceTab == 3 && a.pickerFRPLevel == 1) &&
			!(a.sourceTab == 2 && a.pickerSSHInputMode) { // Final Review I6:输入语义层 q 一律进输入(edit-menu-paste 同标准)
			if input := a.pickerInputRef(); input.text == nil || *input.text == "" {
				a.closeSourcePicker()
				return a, nil
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
		// 无输入框的层（context 列表/ssh 主机候选）：导航统一 C-j/C-k（外层 KeyCtrlJ/KeyCtrlK 分支），裸 j/k 退役为无操作
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

// cycleSourceTab 可见集内循环切换 tab(dir=1 正向,-1 反向);当前 tab 不在可见集时回落首个。
func (a *App) cycleSourceTab(dir int) int {
	if len(a.availableTabs) == 0 {
		return a.sourceTab // 防御:可见集为空时保持当前 tab 不动
	}
	for i, t := range a.availableTabs {
		if t == a.sourceTab {
			return a.availableTabs[(i+dir+len(a.availableTabs))%len(a.availableTabs)]
		}
	}
	return a.availableTabs[0]
}

// pickerTabEnterCmd 切换 tab 后按需拉取候选。
func (a *App) pickerTabEnterCmd() tea.Cmd {
	a.pickerCursor = 0
	a.pickerDirFilter = "" // 切 tab 清词(fview "切面板清词"同款)
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

// frpCurName 当前 frp 连接名（记录名；新建未落名时回落表单 serverName）。
func (a *App) frpCurName() string {
	if a.pickerFRPConnName != "" {
		return a.pickerFRPConnName
	}
	return a.pickerFRPSName
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
	filter := strings.TrimSpace(a.pickerDirFilter)
	var items []sourceCandidate
	if filter == "" && a.pickerFRPDir != "/" {
		items = append(items, sourceCandidate{label: "../", value: "..", dir: true})
	}
	for _, c := range a.pickerCandidates {
		if filter == "" || isSubseq(c.label, filter) { // 子序列(tui-picker-unify Final Review I4)
			items = append(items, c)
		}
	}
	return items
}

// pickerBackspace 逐级返回。
func (a *App) pickerBackspace() (tea.Cmd, bool) {
	a.pickerCursor = 0
	a.pickerDirFilter = "" // 目录变化重置过滤
	a.pickerPathInput = ""
	// 返回 bool=本次是否消费了回退(Esc 逐级据此判"已顶层才关";Final Review C1
	// 快照键方案的根治替代——层内任何导航态都覆盖)
	switch a.sourceTab {
	case 0:
		if a.pickerK8sLevel > 0 {
			a.pickerK8sLevel--
			if a.pickerK8sLevel == 1 {
				a.pickerCandidates = nil
				a.pickerNsInput = "" // 清残留，避免误导
				a.pickerLoading = true
				return fetchK8sNamespacesCmd(a.pickerKubeCtx), true
			}
			a.pickerNsInput = "" // ns→ctx 同清(Final Review I3:防幽灵过滤复燃)
			return nil, true
		}
	case 1:
		parent := filepath.Dir(a.pickerLocalDir)
		if parent != a.pickerLocalDir {
			a.pickerLocalDir = parent
			return nil, true
		}
	case 2:
		if a.pickerSSHHost != "" {
			if a.pickerSSHDir != a.pickerSSHRoot {
				a.pickerSSHDir = parentPath(a.pickerSSHDir)
				a.pickerCandidates = nil
				a.pickerLoading = true
				return fetchSSHDirCmd(a.pickerSSHHost, a.pickerSSHDir, a.sshPw(a.pickerSSHHost)), true
			}
			// 已在起始层：返回主机层
			a.pickerSSHHost = ""
			a.pickerSSHDir = ""
			a.pickerSSHRoot = ""
			a.pickerCandidates = nil
			return nil, true
		}
	case 3: // FRP：表单内逐级返回（step0 时返回 L0）；L2 目录逐级上翻
		switch a.pickerFRPLevel {
		case 1:
			a.pickerFRPInput = ""
			if a.pickerFRPStep > 0 {
				a.pickerFRPStep--
				a.frpSeedStepInput(a.pickerFRPStep) // 回退带出上一步当前值（frp-edit-prefill）
			} else {
				a.pickerFRPLevel = 0
			}
			return nil, true
		case 2:
			if a.pickerFRPPort == 0 { // 未初始化浏览目标（stcp 隧道未就绪/tcp 未落位）
				a.pickerFRPLevel = 0
				return nil, true
			}
			if a.pickerFRPDir != "/" {
				a.pickerFRPDir = parentPath(a.pickerFRPDir)
				a.pickerCandidates = nil
				a.pickerLoading = true
				return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPHost, a.pickerFRPPort, a.pickerFRPDir, a.sshPasswords[a.frpPwKey()]), true
			}
			a.closeFRPBrowse()
			return nil, true
		}
	}
	return nil, false
}

// pickerEnter 按当前层级分派：下钻 / 勾选确认 / 打开文件。
func (a *App) pickerEnter() tea.Cmd {
	// keybind-unify-r2 A3：SSH 新增主机表单态——Enter=保存（fview 同款；
	// 空输入留框不保存，重复主机幂等跳过）
	if a.sourceTab == 2 && a.pickerSSHHost == "" && a.pickerSSHInputMode {
		host := strings.TrimSpace(a.pickerHostInput)
		if host == "" {
			return nil
		}
		dup := false
		for _, h := range sshRulesHosts {
			if h == host {
				dup = true
			}
		}
		if !dup {
			sshRulesHosts = append(sshRulesHosts, host)
			if err := refreshSSHCandidates(a.rulesPath); err != nil { // Final Review I5
				a.yankMsg = "主机已加但写盘失败: " + err.Error()
			}
		}
		a.pickerSSHInputMode = false
		a.pickerHostInput = ""
		a.pickerCursor = 0
		return nil
	}
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
	if a.sourceTab == 3 && a.pickerFRPLevel == 2 && a.pickerFRPPort != 0 && strings.HasPrefix(a.pickerDirFilter, "/") {
		path := strings.TrimSuffix(a.pickerDirFilter, "/")
		if path == "" {
			path = "/"
		}
		if _, err := stream.SSHListDirWithPort(stream.FRPDefaultUser(a.pickerFRPUser)+"@"+a.pickerFRPHost, a.pickerFRPPort, path, a.sshPasswords[a.frpPwKey()]); err != nil { // 空走默认用户,裸拼 "@host" ssh 无法解析(frp-pwd-focus 同修)
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
		return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPHost, a.pickerFRPPort, path, a.sshPasswords[a.frpPwKey()])
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
			a.pickerDirFilter = "" // 下钻清词(tui-picker-unify)
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
			a.pickerSSHInputMode = false // 下钻退输入态(tui-picker-unify)
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
				// 复用 pickerNewFRPForm 全清（与 C-a 等价；此前内联只清 ConnName，
				// 编辑回填的 serverAddr/token 等残值会混进新记录——顺手根治）
				a.pickerNewFRPForm()
				return nil
			}
			conn, ok := frpStore().FindConn(cand.value)
			if !ok {
				return nil
			}
			// 完整快照落位（confirm 保存记录 / 密码框暂存都以它为准，比零散字段组回可靠）
			cur := conn
			a.pickerFRPCur = &cur
			a.pickerFRPConnName = conn.Name
			a.pickerFRPUser = conn.User
			a.pickerLoading = true
			return fetchFRPTunnelCmd(conn)
		case 1:
			return a.pickerFRPFormEnter(cand)
		default: // 远程目录浏览
			if a.pickerFRPPort == 0 {
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
				return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPHost, a.pickerFRPPort, a.pickerFRPDir, a.sshPasswords[a.frpPwKey()])
			}
			if cand.dir {
				a.pickerFRPDir = strings.TrimSuffix(a.pickerFRPDir, "/") + "/" + cand.value
				a.pickerCandidates = nil
				a.pickerCursor = 0
				a.pickerLoading = true
				a.pickerDirFilter = ""
				return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPHost, a.pickerFRPPort, a.pickerFRPDir, a.sshPasswords[a.frpPwKey()])
			}
			a.pickerRemotePath = strings.TrimSuffix(a.pickerFRPDir, "/") + "/" + cand.value
			return a.confirmFRPPicker() // Task 8 实现
		}
	}
	return nil
}

// pickerFRPFormEnter 表单各步骤 Enter 提交（官方字段逐项收集）。
// 步骤按 type 释义：0=type 1=serverAddr 2=token；stcp：3=secretKey 4=serverName 5=user 6=alias；
// tcp：3=remotePort 4=user 5=alias（别名可空，末步回车即提交建连：stcp 拉隧道 / tcp 直连）。
func (a *App) pickerFRPFormEnter(cand sourceCandidate) tea.Cmd {
	input := strings.TrimSpace(a.pickerFRPInput)
	next := func(step int) {
		a.pickerFRPStep = step
		a.pickerCursor = 0
		a.frpSeedStepInput(step) // 进入该步即带出当前值（frp-edit-prefill）
	}
	switch a.pickerFRPStep {
	case 0: // 连接类型（候选列表选择）
		if cand.value != frp.TypeSTCP && cand.value != frp.TypeTCP {
			return nil
		}
		a.pickerFRPFormType = cand.value
		next(1)
	case 1: // serverAddr（host[:port]，端口缺省 7000）
		if input == "" {
			return nil
		}
		a.pickerFRPServerAddr = input
		next(2)
	case 2: // auth token（可空回车跳过）
		a.pickerFRPToken = input
		next(3)
	case 3:
		if a.pickerFRPFormType == frp.TypeTCP { // remotePort
			port, err := strconv.Atoi(input)
			if err != nil || port <= 0 || port > 65535 {
				return nil
			}
			a.pickerFRPRemotePort = port
			next(4)
			return nil
		}
		if input == "" { // stcp secretKey 必填
			return nil
		}
		a.pickerFRPSecretKey = input
		next(4)
	case 4:
		if a.pickerFRPFormType == frp.TypeTCP { // user → step5(alias)
			if input == "" {
				return nil
			}
			a.pickerFRPUser = input
			next(5)
			return nil
		}
		if input == "" { // stcp serverName 必填
			return nil
		}
		a.pickerFRPSName = input
		next(5)
	case 5: // tcp: alias → 提交 / stcp: user（必填）
		if a.pickerFRPFormType == frp.TypeTCP {
			a.pickerFRPAlias = input
			return a.submitFRPForm()
		}
		if input == "" {
			return nil
		}
		a.pickerFRPUser = input
		next(6)
	case 6: // stcp: alias → 提交
		a.pickerFRPAlias = input
		return a.submitFRPForm()
	}
	return nil
}

// submitFRPForm 表单收尾：组连接（官方字段平铺）→ 先存记录（中途放弃不丢配置，
// L0 可直接重选）→ 建连（stcp 拉 visitor 隧道 / tcp 零配置直连）。
func (a *App) submitFRPForm() tea.Cmd {
	a.pickerFRPInput = ""
	host, port := frp.SplitServerAddr(a.pickerFRPServerAddr)
	name := a.pickerFRPSName // stcp 默认名 = serverName（与远端 proxies name 对齐）
	if a.pickerFRPFormType == frp.TypeTCP {
		name = host // tcp 默认名 = frps 地址
	}
	conn := frp.Conn{
		Name: name, Type: a.pickerFRPFormType,
		ServerAddr: host, ServerPort: port, Token: a.pickerFRPToken,
		ServerName: a.pickerFRPSName, SecretKey: a.pickerFRPSecretKey,
		RemotePort: a.pickerFRPRemotePort, User: a.pickerFRPUser,
		Alias: strings.TrimSpace(a.pickerFRPAlias),
	}
	// 按型清空对侧字段（对齐 fview saveForm isTCP 语义）：编辑态 step0 可切型，
	// 无差别组字段会把 sk/serverName 残留进 tcp（或 remotePort 残留进 stcp）
	if a.pickerFRPFormType == frp.TypeTCP {
		conn.SecretKey, conn.ServerName = "", ""
	} else {
		conn.RemotePort = 0
	}
	if old, ok := frpStore().FindConn(conn.Name); ok && old.Path != "" {
		conn.Path = old.Path // 同名重建：保留已确认的日志路径
	}
	frpStore().UpsertConn(conn)
	if err := frpStore().Save(); err != nil {
		a.appendErrorLine(fmt.Sprintf("frp 记录保存失败: %v", err))
	}
	a.pickerLoading = true
	return fetchFRPTunnelCmd(conn)
}

// confirmFRPPicker FRP 确认建流：快照补 Path 落库 → stcp 隧道移交 FRPSource / tcp 直连 FRPDirectSource。
func (a *App) confirmFRPPicker() tea.Cmd {
	cur := a.pickerFRPCur
	if cur == nil || a.pickerFRPPort == 0 {
		return nil
	}
	path := strings.TrimSpace(a.pickerRemotePath)
	if path == "" && a.pickerFRPDir != "" {
		path = a.pickerFRPDir
	}
	if path == "" {
		return nil
	}
	saved := *cur
	saved.Path = path
	frpStore().UpsertConn(saved)
	if err := frpStore().Save(); err != nil {
		a.appendErrorLine(fmt.Sprintf("frp 记录保存失败: %v", err))
	}
	BumpUsage(usageFRPConn + saved.Name)
	pw := a.sshPasswords["frp:"+saved.Name]
	tunnel := a.pickerFRPTunnel
	a.pickerFRPTunnel = nil // 隧道移交 FRPSource（tcp 无隧道），防 closeSourcePicker 清理
	a.closeSourcePicker()
	if saved.Type == frp.TypeTCP {
		src := stream.NewFRPDirectSource(saved.Name, saved.ServerAddr, saved.RemotePort, saved.User, path, 200)
		src.SetBufferLines(a.bufSize)
		src.SetAlias(saved.Alias) // 主界面 label：别名空回落 name（对齐 fview tab 语义）
		if pw != "" {
			src.SetPassword(pw)
		}
		return a.ReplaceStream(src)
	}
	if tunnel == nil {
		return nil
	}
	src := stream.NewFRPSource(saved.Name, tunnel, saved.User, path, 200)
	src.SetBufferLines(a.bufSize) // gz 归档传回行数对齐本地 ring 容量
	src.SetAlias(saved.Alias)     // 同上：stcp 形态
	if pw != "" {
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

// pickerLevelBack Esc 逐级的层级回退一步(不含目录上翻,fview 准绳);
// 无层级可回=false(调用方关 picker)。
func (a *App) pickerLevelBack() (tea.Cmd, bool) {
	switch a.sourceTab {
	case 0:
		if a.pickerK8sLevel > 0 {
			a.pickerK8sLevel--
			a.pickerNsInput = ""
			a.pickerCandidates = nil
			if a.pickerK8sLevel == 1 {
				a.pickerLoading = true
				return fetchK8sNamespacesCmd(a.pickerKubeCtx), true
			}
			return nil, true
		}
	case 2:
		if a.pickerSSHHost != "" {
			a.pickerSSHHost = ""
			a.pickerSSHDir = ""
			a.pickerSSHRoot = ""
			a.pickerCandidates = nil
			return nil, true
		}
	case 3:
		switch a.pickerFRPLevel {
		case 1:
			a.pickerFRPInput = ""
			if a.pickerFRPStep > 0 {
				a.pickerFRPStep--
				a.frpSeedStepInput(a.pickerFRPStep) // 回退带出上一步当前值（frp-edit-prefill）
			} else {
				a.pickerFRPLevel = 0
			}
			return nil, true
		case 2:
			a.closeFRPBrowse()
			a.pickerFRPLevel = 0
			return nil, true
		}
	}
	return nil, false
}

// frpSeedStepInput 表单进入 step 时把该步当前值带进输入框、光标置 runes 尾
// （frp-edit-prefill：编辑带出原值，Enter 保留改哪动哪；回退再前进不丢已输值；
// 新表单字段全空，种子自然为空退化现状）。step0 是候选列表：不进输入框，
// 只把光标落到当前类型行（stcp=0/tcp=1，与 frpTypeCandidates 顺序一致）。
func (a *App) frpSeedStepInput(step int) {
	if step == 0 {
		if a.pickerFRPFormType == frp.TypeTCP {
			a.pickerCursor = 1
		} else {
			a.pickerCursor = 0
		}
		return
	}
	var v string
	switch step {
	case 1:
		v = a.pickerFRPServerAddr // 原始串（编辑时含 :port，提交 SplitServerAddr 拆回）
	case 2:
		v = a.pickerFRPToken
	case 3:
		if a.pickerFRPFormType == frp.TypeTCP {
			if a.pickerFRPRemotePort > 0 { // 0=未设（异常存量/新表单），种子空让用户必输
				v = strconv.Itoa(a.pickerFRPRemotePort)
			}
		} else {
			v = a.pickerFRPSecretKey
		}
	case 4:
		if a.pickerFRPFormType == frp.TypeTCP {
			v = a.pickerFRPUser
		} else {
			v = a.pickerFRPSName
		}
	case 5:
		if a.pickerFRPFormType == frp.TypeTCP {
			v = a.pickerFRPAlias
		} else {
			v = a.pickerFRPUser
		}
	case 6:
		v = a.pickerFRPAlias
	}
	a.pickerFRPInput = v
	a.pickerFRPCursor = len([]rune(v))
}

// pickerEditFRP C-e:选中连接回填表单(编辑保存走同名 Upsert 覆盖,Path 保留)。
// frp-edit-prefill:addr 原始串带 :port(≠7000 时)——提交 SplitServerAddr 拆回,
// 端口完整往返(此前不带端口,非默认端口编辑一轮被吞成 7000);
// User 空(=默认 root 语义)显式化 root,解 user 必填步卡死;
// type 光标落原类型行,Enter 即保留原类型。
func (a *App) pickerEditFRP(cands []sourceCandidate) {
	if a.pickerCursor >= len(cands) {
		return
	}
	cur := cands[a.pickerCursor]
	conn, ok := frpStore().FindConn(cur.value)
	if !ok {
		return
	}
	a.pickerFRPFormType = string(conn.Type)
	a.pickerFRPServerAddr = conn.ServerAddr
	if conn.ServerPort > 0 && conn.ServerPort != 7000 { // 默认 7000 不拼(输入框干净),提交拆回同值
		a.pickerFRPServerAddr = fmt.Sprintf("%s:%d", conn.ServerAddr, conn.ServerPort)
	}
	a.pickerFRPToken = conn.Token
	a.pickerFRPSecretKey = conn.SecretKey
	a.pickerFRPSName = conn.ServerName
	a.pickerFRPRemotePort = conn.RemotePort
	a.pickerFRPUser = conn.User
	if a.pickerFRPUser == "" {
		a.pickerFRPUser = "root" // "空默认 root"显式化(语义等价),user 步种子非空解卡
	}
	a.pickerFRPAlias = conn.Alias
	a.pickerFRPStep = 0
	a.pickerFRPInput = ""
	a.pickerFRPConnName = conn.Name
	a.pickerFRPCur = &conn
	a.pickerFRPLevel = 1
	a.pickerDirFilter = "" // 清 L0 残词(防表单幽灵高亮,Final Review M8)
	a.frpSeedStepInput(0)  // type 光标落原类型行(stcp=0/tcp=1)
}

// pickerNewFRPForm C-a:FRP 空表单(等价 +new 入口,字段全清)。
func (a *App) pickerNewFRPForm() {
	if a.pickerFRPLevel == 2 { // L2 浏览进表单:释放未接管隧道(Final Review I2)
		a.closeFRPBrowse()
	}
	a.pickerFRPFormType = ""
	a.pickerFRPServerAddr = ""
	a.pickerFRPToken = ""
	a.pickerFRPSecretKey = ""
	a.pickerFRPSName = ""
	a.pickerFRPRemotePort = 0
	a.pickerFRPUser = ""
	a.pickerFRPAlias = ""
	a.pickerFRPStep = 0
	a.pickerFRPInput = ""
	a.pickerFRPConnName = ""
	a.pickerFRPCur = nil
	a.pickerFRPLevel = 1
	a.pickerDirFilter = "" // 清 L0 残词(Final Review M8)
	a.pickerCursor = 0
}

// pickerDeleteFRP C-d 删除：FRP L0 删连接记录。
// 删除后持久化并收敛光标；+new 占位项与表单/浏览层不可删。
// deleteSSHHost 删除主机并写回 rules.yaml（A3；~/.ssh/config 发现源跳过并提示，
// fview SSHPanel 同款——发现源改动应去改 config 本身）。
func (a *App) deleteSSHHost(host string) {
	if sshDiscoveredSet[host] {
		a.yankMsg = host + " 来自 ~/.ssh/config，不可在此删除"
		return
	}
	out := sshRulesHosts[:0]
	for _, h := range sshRulesHosts {
		if h != host {
			out = append(out, h)
		}
	}
	sshRulesHosts = out
	if err := refreshSSHCandidates(a.rulesPath); err != nil { // Final Review I5
		a.yankMsg = "主机已删但写盘失败: " + err.Error()
		return
	}
	a.pickerCursor = 0 // 列表缩短，光标回顶
	a.yankMsg = "已删除主机 " + host
}

func (a *App) pickerDeleteFRP(cands []sourceCandidate) tea.Cmd {
	if a.sourceTab != 3 || a.pickerCursor >= len(cands) {
		return nil
	}
	cand := cands[a.pickerCursor]
	if a.pickerFRPLevel != 0 || cand.value == "+new" {
		return nil
	}
	if !frpStore().DeleteConn(cand.value) {
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
		if a.pickerK8sLevel == 0 {
			return inputRef{&a.pickerDirFilter, &a.pickerFilterCursor} // tui-picker-unify:ctx 列表无框过滤
		}
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
		if a.pickerSSHInputMode { // A3：新增主机表单（单字段，fview 同款）
			return inputRef{&a.pickerHostInput, &a.pickerHostCursor}
		}
		return inputRef{&a.pickerDirFilter, &a.pickerFilterCursor} // tui-picker-unify:过滤态无框打字
	case 3:
		if a.pickerFRPLevel == 0 {
			return inputRef{&a.pickerDirFilter, &a.pickerFilterCursor} // tui-picker-unify:L0 连接列表无框过滤
		}
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
		return a.ReplaceStream(stream.NewFileSource([]string{path}).WithTailLines(a.bufSize))
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
		src.SetBufferLines(a.bufSize) // gz 归档传回行数对齐本地 ring 容量
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

	tabNames := []string{"K8s", "本地", "SSH", "FRP"}
	var tabParts []string
	for _, i := range a.availableTabs {
		if i == a.sourceTab {
			tabParts = append(tabParts, PopupActiveTabStyle.Render(" "+tabNames[i]+" "))
		} else {
			tabParts = append(tabParts, PopupTabStyle.Render(" "+tabNames[i]+" "))
		}
	}
	// tab 行与分隔线延后渲染:boxW 由内容最长行驱动,sep 需与最终 boxW 对齐(picker-width-hints)

	cands := a.visiblePickerCandidates()
	switch a.sourceTab {
	case 0:
		switch a.pickerK8sLevel {
		case 0:
			cur := a.pickerBreadcrumbCtx()
			head := BreadcrumbStyle.Render(" ctx: " + cur + " ")
			content.WriteString(" " + head + "\n")
			if a.pickerDirFilter != "" {
				content.WriteString(keycapHint(" ⌕ "+a.pickerDirFilter) + "\n") // 无框过滤回显(Final Review I5)
			}
			content.WriteString("\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(keycapHint(" 查询中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(keycapHint(" 无 context（检查 kubectl）") + "\n")
			} else {
				content.WriteString(renderCandidateListMark(cands, a.pickerCursor, cur, a.pickerDirFilter))
			}
		case 1:
			content.WriteString(" " + BreadcrumbStyle.Render(" ctx: "+a.pickerBreadcrumbCtx()+" ") + "\n")
			content.WriteString(a.inputLine(a.pickerNsInput, a.pickerNsCursor, "或输入 namespace 回车直达") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(keycapHint(" 查询中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(keycapHint(" 无 namespace") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8, a.pickerNsInput)) // 高亮词源=本层词(Final Review M7)
			}
		default:
			content.WriteString(" " + BreadcrumbStyle.Render(" "+a.pickerBreadcrumbCtx()+"/"+a.pickerNsInput+" ") + "\n")
			content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤资源名…") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(keycapHint(" 查询中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(keycapHint(" 无资源（检查权限或过滤词）") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, a.pickerChecked, 8, a.pickerDirFilter))
			}
		}
	case 1:
		content.WriteString(" " + breadcrumbChain(a.pickerLocalDir) + "\n")
		content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤…") + "\n\n")
		if len(cands) == 0 {
			content.WriteString(keycapHint(" 目录为空或不可读") + "\n")
		} else {
			content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10, a.pickerDirFilter))
		}
	case 2:
		if a.pickerSSHHost == "" {
			if a.pickerSSHInputMode { // A3：新增主机表单（fview 同款单字段，Enter 保存/Esc 取消）
				hostLine := a.inputLine(a.pickerHostInput, a.pickerHostCursor, "user@host[:port] 回车保存")
				content.WriteString(DetailLabelStyle.Render("新增主机: ") + hostLine + "\n\n")
			} else { // tui-picker-unify 过滤态:无框打字+⌕ 回显(Final Review I5)
				if a.pickerDirFilter != "" {
					content.WriteString(keycapHint(" ⌕ "+a.pickerDirFilter) + "\n\n")
				} // 快捷键说明沉底部栏(picker-width-hints),弹窗内不再印
			}
			if len(cands) > 0 {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8, a.pickerDirFilter))
			} else if a.pickerDirFilter != "" {
				content.WriteString(keycapHint(" 无匹配主机") + "\n")
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
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10, a.pickerDirFilter))
			}
		}
	case 3: // FRP
		switch a.pickerFRPLevel {
		case 0: // L0：连接列表(tui-picker-unify:无框打字+⌕ 回显,死框退役)
			if a.pickerDirFilter != "" {
				content.WriteString(keycapHint(" ⌕ " + a.pickerDirFilter) + "\n\n")
			} // 快捷键说明沉底部栏(picker-width-hints),弹窗内不再印
			if len(cands) == 0 {
				content.WriteString(keycapHint(" 无保存的连接") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10, a.pickerDirFilter))
			}
		case 1: // L1：新建表单（官方字段逐项收集，tcp/stcp 步骤集不同）
			prompts := map[string][]string{
				frp.TypeTCP: {
					"连接类型", "serverAddr（frps 地址 host[:port]）", "token（可空）",
					"remotePort（frps 暴露端口）", "ssh 用户名", "别名（可空，展示用）",
				},
				frp.TypeSTCP: {
					"连接类型", "serverAddr（frps 地址 host[:port]）", "token（可空）",
					"secretKey（与远端 stcp 的 sk 一致）",
					"serverName（远端 stcp 服务名，即 proxies.name）", "ssh 用户名", "别名（可空，展示用）",
				},
			}
			pl := prompts[a.pickerFRPFormType]
			if len(pl) == 0 {
				pl = prompts[frp.TypeSTCP]
			}
			prompt := "连接类型"
			if a.pickerFRPStep < len(pl) {
				prompt = pl[a.pickerFRPStep]
			}
			content.WriteString(DetailLabelStyle.Render(prompt+": ") +
				a.inputLine(a.pickerFRPInput, a.pickerFRPCursor, "") + "\n\n")
			if a.pickerFRPStep == 0 {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8, a.pickerDirFilter))
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
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10, a.pickerDirFilter))
			}
		}
	}

	// 内容驱动动态宽(picker-width-hints):最长内容行(含 tab 行)决定 boxW——
	// 超宽行撑开弹窗刚好装下,短内容保持 64 视觉零变化,窄终端封顶 a.width-4。
	tabLine := strings.Join(tabParts, "  ")
	maxW := displayWidth(tabLine)
	for _, l := range strings.Split(content.String(), "\n") {
		if w := displayWidth(l); w > maxW {
			maxW = w
		}
	}
	boxW := pickerBoxW(maxW, a.width)
	full := tabLine + "\n" + popupTabSep(boxW) + "\n\n" + content.String()
	box := PopupBoxStyle.Width(boxW).Render(full)
	return a.overlayToVL(box, vl)
}

// pickerBoxW 计算选择器弹窗内容宽（lipgloss Width 口径，含左右 padding 各 1）：
// 内容不超 64 时保持现状视觉零变化；超长行驱动撑宽（+2 padding 恰好装下）；
// 窄终端封顶 termW-4（边框 2 + padding 2，box 总宽不越终端）。
func pickerBoxW(maxContentW, termW int) int {
	return min(max(64, maxContentW+2), termW-4)
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
func renderCandidateListMark(cands []sourceCandidate, cursor int, current, filter string) string {
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
		b.WriteString(prefix + " " + mark + hlLabel(cands[i].label, filter, labelStyle) + "\n")
	}
	if len(cands) > end {
		b.WriteString(keycapHint(fmt.Sprintf("   …共%d项", len(cands))) + "\n")
	}
	return b.String()
}

// FilterHitStyle 过滤命中段样式(tui-picker-unify:金粗体,fview kw-hit 同位语义)。
var FilterHitStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e2b93b"))

// hlLabel 候选 label 按过滤词分段渲染:命中段 FilterHitStyle,其余保持 base
// (光标行 base=SelArrowStyle 时光标色与命中金叠加可辨,双态不冲突)。
func hlLabel(label, filter string, base lipgloss.Style) string {
	if filter == "" {
		return base.Render(label)
	}
	var b strings.Builder
	for _, seg := range splitHighlight(label, filter) {
		if seg.Hit {
			b.WriteString(FilterHitStyle.Render(seg.T))
		} else {
			b.WriteString(base.Render(seg.T))
		}
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
func renderCandidateList(cands []sourceCandidate, cursor int, checked map[string]bool, maxRows int, filter string) string {
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
		b.WriteString(prefix + " " + mark + hlLabel(cands[i].label, filter, labelStyle) + "\n")
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
