package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// setupLocalBrowseFixture 自建本地浏览测试夹具（/tmp 会被系统定期清理，
// 测试不能依赖外部残留文件）。
func setupLocalBrowseFixture(t *testing.T) {
	t.Helper()
	for _, p := range []string{"/tmp/lvbrowse/sub"} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"/tmp/lvbrowse/readme.txt", "/tmp/lvbrowse/sub/app.log"} {
		if err := os.WriteFile(filepath.FromSlash(p), []byte("hello\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// o 键打开选择器（context 层）；Tab 循环切换三 tab；Esc 关闭。
func TestSourcePickerOpenTabClose(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if !app.sourcePickerMode {
		t.Fatal("o 应打开源选择器")
	}
	if app.pickerK8sLevel != 0 {
		t.Fatalf("K8s 默认层级应为 0 (context)，实际 %d", app.pickerK8sLevel)
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

// K8s context → ns → 资源：Enter 下钻、Backspace 返回。
func TestSourcePickerK8sLevels(t *testing.T) {
	app := newTestApp()
	app.k8sUseContextFn = func(string) error { return nil } // mock context 切换
	app.openSourcePicker(0)
	app.pickerContexts = []sourceCandidate{{label: "ctx-a", value: "ctx-a"}, {label: "ctx-b", value: "ctx-b"}}
	app.pickerLoading = false // 回填完成态
	// context 层 Enter：选中第一项，应切到 ns 层
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerK8sLevel != 1 {
		t.Fatalf("context Enter 后应到 ns 层，实际 %d", app.pickerK8sLevel)
	}
	// ns 候选回填后 Enter 进入资源层
	app.pickerNamespaces = []sourceCandidate{{label: "default", value: "default"}, {label: "kube-system", value: "kube-system"}}
	app.pickerLoading = false // 回填完成态
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerK8sLevel != 2 || app.pickerNsInput != "default" {
		t.Fatalf("ns Enter 后应在资源层/ns=default，实际 level=%d ns=%q", app.pickerK8sLevel, app.pickerNsInput)
	}
	// Backspace 逐级返回
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerK8sLevel != 1 {
		t.Fatalf("Backspace 应回 ns 层，实际 %d", app.pickerK8sLevel)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerK8sLevel != 0 {
		t.Fatalf("Backspace 应回 context 层，实际 %d", app.pickerK8sLevel)
	}
}

// K8s 资源层：Space 多选 + Enter 生成 MultiK8sSource。
func TestSourcePickerK8sMultiSelect(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(0)
	app.pickerK8sLevel = 2
	app.pickerNsInput = "default"
	app.pickerCandidates = []sourceCandidate{
		{label: "deploy/a", value: "deployment/a"},
		{label: "deploy/b", value: "deployment/b"},
		{label: "pod/c", value: "pod/c"},
	}

	app.Update(tea.KeyMsg{Type: tea.KeySpace})
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeySpace})
	if len(app.pickerChecked) != 2 {
		t.Fatalf("应勾选 2 项，实际 %d", len(app.pickerChecked))
	}

	app.processLine(model.RawLine{Text: "old line", Source: "old"})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.sourcePickerMode {
		t.Fatal("Enter 后应关闭选择器")
	}
	if app.buffer.Len() > 1 {
		t.Fatalf("切换源后应清屏（至多 1 条提示行），buffer=%d", app.buffer.Len())
	}
	label := app.stream.Label()
	if !strings.Contains(label, "deployment/a") || !strings.Contains(label, "pod/c") {
		t.Errorf("MultiK8s label 应含勾选项: %s", label)
	}
}

// 本地 tab：目录浏览（目录下钻 / 文件打开 / Backspace 上级）。
func TestSourcePickerLocalBrowse(t *testing.T) {
	setupLocalBrowseFixture(t)
	app := newTestApp()
	app.openSourcePicker(1)
	app.pickerLocalDir = "/tmp/lvbrowse"
	cands := app.visiblePickerCandidates()
	if len(cands) != 3 || cands[0].value != ".." || cands[1].label != "sub/" || cands[2].label != "readme.txt" {
		t.Fatalf("顶层候选应为 [../, sub/, readme.txt]（目录在前、全部文件显示），实际 %v", cands)
	}
	// Enter(../ 不动) → j 到 sub/ → Enter 进目录
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerLocalDir != "/tmp/lvbrowse/sub" {
		t.Fatalf("Enter 应进子目录，实际 %s", app.pickerLocalDir)
	}
	// 目录内 j 跳过 ../ 打开 app.log
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.sourcePickerMode {
		t.Fatal("文件 Enter 应关闭选择器并切换源")
	}
	if label := app.stream.Label(); label == "test" {
		t.Error("本地文件源未生效（仍是 mock）")
	}
	// Backspace 上级
	app2 := newTestApp()
	app2.openSourcePicker(1)
	app2.pickerLocalDir = "/tmp/lvbrowse/sub"
	app2.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app2.pickerLocalDir != "/tmp/lvbrowse" {
		t.Fatalf("Backspace 应回上级，实际 %s", app2.pickerLocalDir)
	}
}

// SSH tab：主机层 Enter 进入远程目录浏览态，Backspace 返回主机层。
func TestSourcePickerSSHBrowse(t *testing.T) {
	SetSSHHosts([]string{"web1", "web2"})
	app := newTestApp()
	app.openSourcePicker(2)
	// 主机层候选过滤
	app.pickerHostInput = "web2"
	cands := app.visiblePickerCandidates()
	if len(cands) != 1 || cands[0].value != "web2" {
		t.Fatalf("主机过滤失败: %v", cands)
	}
	// Enter 连接：进入远程目录浏览态（起始 /var/log）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerSSHHost != "web2" {
		t.Fatalf("应进入远程目录层，host=%q", app.pickerSSHHost)
	}
	if app.pickerSSHDir != "/" || app.pickerSSHRoot != "/" {
		t.Fatalf("起始目录应 /，dir=%q root=%q", app.pickerSSHDir, app.pickerSSHRoot)
	}
	// 目录候选回填（模拟 msg）：目录 + 文件
	app.pickerCandidates = []sourceCandidate{
		{label: "logs/", value: "logs", dir: true},
		{label: "app.log", value: "app.log"},
	}
	// Backspace：起始层直接回主机层
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerSSHHost != "" {
		t.Fatal("起始层 Backspace 应回主机层")
	}
}

// SSH 远程目录：目录下钻、文件确认。
func TestSourcePickerSSHDirConfirm(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(2)
	app.pickerSSHHost = "web1"
	app.pickerSSHDir = "/var/log"
	app.pickerSSHRoot = "/var/log"
	app.pickerCandidates = []sourceCandidate{
		{label: "nginx/", value: "nginx", dir: true},
		{label: "sys.log", value: "sys.log"},
	}
	// j 跳过 ../ 到 nginx/，Enter 下钻
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerSSHDir != "/var/log/nginx" {
		t.Fatalf("目录下钻失败: %s", app.pickerSSHDir)
	}
	// Backspace 回上级，j 跳过 ../ 选文件确认
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	app.pickerCandidates = []sourceCandidate{{label: "sys.log", value: "sys.log"}}
	app.pickerLoading = false // 回填完成态
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.sourcePickerMode {
		t.Fatal("文件 Enter 应关闭选择器")
	}
	if label := app.stream.Label(); label != "ssh://web1/var/log/sys.log" {
		t.Errorf("SSH 源 label: %s", label)
	}
}

// 目录浏览过滤：输入前缀过滤候选；输入存在的路径直达。
func TestSourcePickerLocalFilter(t *testing.T) {
	setupLocalBrowseFixture(t)
	app := newTestApp()
	app.openSourcePicker(1)
	app.pickerLocalDir = "/tmp/lvbrowse"
	app.pickerDirFilter = "su"
	cands := app.visiblePickerCandidates()
	if len(cands) != 1 || cands[0].label != "sub/" {
		t.Fatalf("过滤 'su' 应只剩 sub/（过滤态隐藏 ../，光标落在首个匹配），实际 %v", cands)
	}
	// 过滤后无匹配 → 空列表（过滤态无 ../）
	app.pickerDirFilter = "zzz"
	if len(app.visiblePickerCandidates()) != 0 {
		t.Fatal("无匹配应为空列表")
	}
	// 输入存在的目录路径直达
	app.pickerDirFilter = "/tmp/lvbrowse/sub"
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerLocalDir != "/tmp/lvbrowse/sub" {
		t.Fatalf("路径直达失败: %s", app.pickerLocalDir)
	}
	// 输入存在的文件路径直接打开
	app2 := newTestApp()
	app2.openSourcePicker(1)
	app2.pickerLocalDir = "/tmp"
	app2.pickerDirFilter = "/tmp/lvbrowse/sub/app.log"
	app2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app2.sourcePickerMode {
		t.Fatal("文件路径直达应关闭选择器并切换源")
	}
}

// K8s 资源层过滤：输入前缀过滤候选；Backspace 先删字符再返回上级。
func TestSourcePickerK8sResourceFilter(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(0)
	app.pickerK8sLevel = 2
	app.pickerNsInput = "default"
	app.pickerCandidates = []sourceCandidate{
		{label: "deploy/api-gateway", value: "deployment/api-gateway"},
		{label: "deploy/user-svc", value: "deployment/user-svc"},
		{label: "pod/gateway-abc", value: "pod/gateway-abc"},
	}
	// 通过打字输入过滤词（保证 cursor 位置正确）
	for _, r := range "gateway" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	cands := app.visiblePickerCandidates()
	if len(cands) != 2 {
		t.Fatalf("过滤 'gateway' 应剩 2 项，实际 %v", cands)
	}
	// Backspace：先删字符（不返回上级）
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerK8sLevel != 2 || app.pickerDirFilter != "gatewa" {
		t.Fatalf("Backspace 应删字符而非返回：level=%d filter=%q", app.pickerK8sLevel, app.pickerDirFilter)
	}
	// 连续删空后再 Backspace 才返回 ns 层
	for i := 0; i < 7; i++ {
		app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	if app.pickerK8sLevel != 1 {
		t.Fatalf("filter 删空后 Backspace 应回 ns 层，实际 %d", app.pickerK8sLevel)
	}
}

// SSH 过滤框：Backspace 删字符不退层；C-u 清空。
func TestSourcePickerSSHFilterBackspace(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(2)
	app.pickerSSHHost = "ht-1"
	app.pickerSSHDir = "/var/log"
	app.pickerCandidates = []sourceCandidate{{label: "cron", value: "cron"}}
	app.pickerLoading = false // 回填完成态
	for _, r := range "cro" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerSSHHost == "" || app.pickerDirFilter != "cr" {
		t.Fatalf("Backspace 应删字符而非退主机层：host=%q filter=%q", app.pickerSSHHost, app.pickerDirFilter)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if app.pickerDirFilter != "" {
		t.Fatalf("C-u 应清空 filter，实际 %q", app.pickerDirFilter)
	}
}

// 防回归：过滤后光标项打开的必须是可见（过滤后）候选，而非原始列表同位置项。
func TestSourcePickerK8sFilteredCursorOpen(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(0)
	app.pickerK8sLevel = 2
	app.pickerNsInput = "default"
	app.pickerCandidates = []sourceCandidate{
		{label: "deploy/api-gateway", value: "deployment/api-gateway"},
		{label: "deploy/billing", value: "deployment/billing"},
		{label: "deploy/user-svc", value: "deployment/user-svc"},
	}
	// 过滤到 billing：可见列表只剩 billing，光标 0
	for _, r := range "billing" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if cands := app.visiblePickerCandidates(); len(cands) != 1 || cands[0].value != "deployment/billing" {
		t.Fatalf("过滤后应只剩 billing: %v", cands)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	label := app.stream.Label()
	if !strings.Contains(label, "billing") {
		t.Fatalf("应打开过滤后的 billing，实际 %s", label)
	}
	if strings.Contains(label, "api-gateway") {
		t.Fatalf("不应打开原始列表同位置的 api-gateway: %s", label)
	}
}

// 防回归：ns 层输入即过滤可见列表；输入后光标越界 Enter 用输入值直达。
func TestSourcePickerNsTypedFilter(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(0)
	app.pickerK8sLevel = 1
	app.pickerNamespaces = []sourceCandidate{
		{label: "default", value: "default"},
		{label: "kube-system", value: "kube-system"},
		{label: "parking", value: "parking"},
	}
	// 输入 park 过滤
	for _, r := range "park" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	cands := app.visiblePickerCandidates()
	if len(cands) != 1 || cands[0].value != "parking" {
		t.Fatalf("ns 过滤失败: %v", cands)
	}
	// 光标 clamp 到 0 后 Enter 进 parking 资源层（候选项补全 ns 名）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerK8sLevel != 2 || app.pickerNsInput != "parking" {
		t.Fatalf("应进资源层: level=%d ns=%q", app.pickerK8sLevel, app.pickerNsInput)
	}
	// 补全 ns：模拟完整输入的直达（光标越界场景）
	app2 := newTestApp()
	app2.openSourcePicker(0)
	app2.pickerK8sLevel = 1
	app2.pickerNamespaces = []sourceCandidate{{label: "default", value: "default"}}
	app2.pickerLoading = false // 回填完成态
	for _, r := range "parking" {
		app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if got := len(app2.visiblePickerCandidates()); got != 0 {
		t.Fatalf("无匹配 ns 应为空列表，实际 %d", got)
	}
	app2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app2.pickerK8sLevel != 2 || app2.pickerNsInput != "parking" {
		t.Fatalf("越界 Enter 应直达输入 ns: level=%d ns=%q", app2.pickerK8sLevel, app2.pickerNsInput)
	}
}

// 防回归：ns 层输入含 j/k 的名称不丢字（字母进输入框而非移动光标）。
func TestSourcePickerNsTypingJK(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(0)
	app.pickerK8sLevel = 1
	for _, r := range "kube-system" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if app.pickerNsInput != "kube-system" {
		t.Fatalf("输入 kube-system 应完整，实际 %q", app.pickerNsInput)
	}
	if app.pickerCursor != 0 {
		t.Fatalf("输入时光标不应移动，实际 %d", app.pickerCursor)
	}
}

// 所有 picker 层 C-j/C-k 上下移动候选。
func TestSourcePickerCtrlJK(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(1)
	app.pickerLocalDir = "/tmp/lvbrowse"
	if app.pickerCursor != 0 {
		t.Fatal("初始光标应在 0")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if app.pickerCursor != 1 {
		t.Fatalf("C-j 应下移到 1，实际 %d", app.pickerCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	if app.pickerCursor != 0 {
		t.Fatalf("C-k 应上移到 0，实际 %d", app.pickerCursor)
	}
	// K8s 资源层同样生效
	app2 := newTestApp()
	app2.openSourcePicker(0)
	app2.pickerK8sLevel = 2
	app2.pickerCandidates = []sourceCandidate{{label: "a", value: "a"}, {label: "b", value: "b"}}
	app2.pickerLoading = false // 回填完成态
	app2.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if app2.pickerCursor != 1 {
		t.Fatalf("K8s 资源层 C-j 应下移，实际 %d", app2.pickerCursor)
	}
	// SSH 远程目录层同样生效
	app3 := newTestApp()
	app3.openSourcePicker(2)
	app3.pickerSSHHost = "h"
	app3.pickerCandidates = []sourceCandidate{{label: "x/", value: "x", dir: true}, {label: "y.log", value: "y.log"}}
	app3.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if app3.pickerCursor != 1 {
		t.Fatalf("SSH 目录层 C-j 应下移，实际 %d", app3.pickerCursor)
	}
}

// q 键：日志页打开选择器；选择器内 q（输入为空）退出会话；过滤中 q 作为字符。
func TestQKeyPickerFlow(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !app.sourcePickerMode {
		t.Fatal("日志页按 q 应打开源选择器")
	}
	// 选择器内 q（context 层无输入框）→ 返回 tea.Cmd 退出
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("选择器内按 q 应返回退出 cmd")
	}
	// tea.Quit cmd 执行产生 tea.QuitMsg
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("cmd 应为 tea.Quit")
	}
	// 本地 tab 过滤中输入 q：不退出、进入过滤框
	app2 := newTestApp()
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	app2.Update(tea.KeyMsg{Type: tea.KeyTab}) // 本地 tab
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // 先输入字符
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}) // q 作为过滤字符
	if !app2.sourcePickerMode {
		t.Fatal("过滤中按 q 不应退出")
	}
	if app2.pickerDirFilter != "lq" {
		t.Fatalf("q 应进入过滤框，实际 %q", app2.pickerDirFilter)
	}
}

// 防回归：目录候选加载中 Enter 不分派（../ 占位项曾导致下钻后误退回根目录）。
func TestSourcePickerSSHLoadingGuard(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(2)
	app.pickerSSHHost = "h"
	app.pickerSSHDir = "/park-cube"
	app.pickerCandidates = nil
	app.pickerLoading = true // fetch 进行中
	// 过滤框输入 park：可见候选仅剩 ../（占位）
	app.pickerDirFilter = "park"
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerSSHDir != "/park-cube" {
		t.Fatalf("加载中 Enter 不应改变目录，实际 %s", app.pickerSSHDir)
	}

	// 候选回填后：过滤无匹配时 Enter 选到 ../ 也不应退回上级
	app.pickerLoading = false
	app.pickerCandidates = []sourceCandidate{{label: "other/", value: "other", dir: true}}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerSSHDir != "/park-cube" {
		t.Fatalf("过滤态下 Enter 不应经 ../ 退回上级，实际 %s", app.pickerSSHDir)
	}

	// 清空过滤后 Enter ../ 正常返回上级
	app.pickerDirFilter = ""
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerSSHDir != "/" {
		t.Fatalf("清空过滤后 Enter ../ 应回上级，实际 %s", app.pickerSSHDir)
	}
}

// 防回归：过滤态隐藏 ../，光标默认在首个匹配项上，Enter 直接下钻。
func TestSourcePickerFilterCursorOnMatch(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(2)
	app.pickerSSHHost = "ht-1"
	app.pickerSSHDir = "/var/log"
	app.pickerLoading = false
	app.pickerCandidates = []sourceCandidate{
		{label: "anaconda/", value: "anaconda", dir: true},
		{label: "audit/", value: "audit", dir: true},
		{label: "boot.log", value: "boot.log"},
	}
	// 输入过滤词：列表只剩 anaconda/，光标 0 即匹配项，Enter 直接下钻
	for _, r := range "ana" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	cands := app.visiblePickerCandidates()
	if len(cands) != 1 || cands[0].value != "anaconda" {
		t.Fatalf("过滤后应只剩 anaconda/（无 ../），实际 %v", cands)
	}
	if app.pickerCursor != 0 {
		t.Fatalf("光标应在首个匹配（0），实际 %d", app.pickerCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerSSHDir != "/var/log/anaconda" {
		t.Fatalf("Enter 应直接下钻到 anaconda，实际 %s", app.pickerSSHDir)
	}
	// 下钻后 filter 自动清空：全列表展示，../ 回到首位且光标归 0
	if app.pickerDirFilter != "" {
		t.Fatalf("下钻后 filter 应清空，实际 %q", app.pickerDirFilter)
	}
	if app.pickerCursor != 0 {
		t.Fatalf("下钻后光标应归 0，实际 %d", app.pickerCursor)
	}
	app.pickerLoading = false
	app.pickerCandidates = []sourceCandidate{{label: "audit/", value: "audit", dir: true}}
	if c := app.visiblePickerCandidates(); len(c) != 2 || c[0].value != ".." {
		t.Fatalf("无过滤时 ../ 应回首位: %v", c)
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

// parentPath 上级目录。
func TestParentPath(t *testing.T) {
	cases := map[string]string{
		"/var/log/nginx": "/var/log",
		"/var":           "/",
		"/":              "/",
		"/var/log/":      "/var",
	}
	for in, want := range cases {
		if got := parentPath(in); got != want {
			t.Errorf("parentPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// K8s 不可见时 openSourcePicker(0) 应钳制到首个可见 tab；可见 tab 原样保留。
func TestOpenSourcePickerClampsHiddenTab(t *testing.T) {
	app := newTestApp()
	app.availableTabs = []int{1, 2, 3} // 模拟无 kubectl
	app.openSourcePicker(0)            // 请求不可见的 K8s
	if app.sourceTab != 1 {
		t.Fatalf("K8s 不可见时 openSourcePicker(0) 应落本地 tab, got %d", app.sourceTab)
	}
	app.openSourcePicker(2) // 可见的 SSH 原样保留
	if app.sourceTab != 2 {
		t.Fatalf("可见 tab 应原样保留, got %d", app.sourceTab)
	}
}

// 空集防御:可见集为空时钳制回本地 tab。
func TestClampSourceTabEmptySet(t *testing.T) {
	app := newTestApp()
	app.availableTabs = nil
	if got := app.clampSourceTab(2); got != 1 {
		t.Fatalf("空集防御应回本地 tab 1, got %d", got)
	}
}

// K8s 隐藏时 Tab 循环 3→1、ShiftTab 1→3(跳过 0);单可见 tab 原地不动。
func TestSourcePickerTabCycleSkipsHidden(t *testing.T) {
	app := newTestApp()
	app.availableTabs = []int{1, 2, 3}
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if app.sourceTab != 1 {
		t.Fatalf("FRP(3) Tab 后应到本地(1),跳过隐藏的 K8s(0), got %d", app.sourceTab)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if app.sourceTab != 3 {
		t.Fatalf("本地(1) ShiftTab 后应到 FRP(3), got %d", app.sourceTab)
	}
}

func TestSourcePickerTabCycleSingleTab(t *testing.T) {
	app := newTestApp()
	app.availableTabs = []int{1} // 远端会话:仅本地
	app.openSourcePicker(1)
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if app.sourceTab != 1 {
		t.Fatalf("单可见 tab 时 Tab 应原地不动, got %d", app.sourceTab)
	}
}

// 远端会话可见集 [1]:弹窗 tab 栏只画本地,不画 K8s/SSH/FRP。
func TestSourcePickerLinesHidesUnavailableTabs(t *testing.T) {
	app := newTestApp()
	app.availableTabs = []int{1}
	app.openSourcePicker(1)
	lines := stripANSI(strings.Join(app.buildSourcePickerLines(10), "\n"))
	for _, name := range []string{"K8s", "SSH", "FRP"} {
		if strings.Contains(lines, name) {
			t.Fatalf("远端会话 tab 栏不应包含 %s", name)
		}
	}
	if !strings.Contains(lines, "本地") {
		t.Fatal("远端会话 tab 栏应包含 本地")
	}
}
