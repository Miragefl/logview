# 源选择器环境过滤 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 源选择器按运行环境过滤可选源——本机无 kubectl 隐藏 K8s tab;ssh/frp 进入的远端会话只显示本地 tab。

**Architecture:** 可见层过滤——内部全局 tab 索引(0=K8s 1=本地 2=SSH 3=FRP)不变,新增 `availableTabs` 可见集(包级 probe 函数变量探测环境,NewApp 算一次),只在入口钳制、Tab 循环、tab 栏渲染三处生效;候选回填 `candidatesMsg.tab` 与 `confirmSourcePicker` 基于全局索引,零改动。

**Tech Stack:** Go + bubbletea TUI;标准库 `os/exec`(LookPath)、`os`(Getenv);纯单元测试(无外部断言库)。

**Spec:** `docs/superpowers/specs/2026-09-04-source-picker-env-design.md`

## Global Constraints

- 全局 tab 索引语义固定:`0=K8s 1=本地 2=SSH 3=FRP`,任何任务不得重排。
- 远端判定:`SSH_TTY` 或 `SSH_CONNECTION` 环境变量非空 → 可见集 `[1]`(优先级高于 kubectl 判定)。
- kubectl 判定:`exec.LookPath("kubectl")` 失败 → 可见集 `[1,2,3]`。
- 探测用包级函数变量(可测试注入),沿用项目 `startFRPTunnel`/`sshHostCandidates` 惯例。
- 代码注释中文,风格与现有文件一致;测试为纯单元测试,断言用标准库。
- 不做:tab 注册表重构、置灰显示、逃生 flag、探测缓存刷新(YAGNI)。
- 搜索/grep 排除 `.worktrees/`(同名 worktree 副本会双倍命中)。
- 测试命令统一在仓库根 `/Users/viscum/Documents/code/justfun/ai/log` 执行。

---

### Task 1: 环境探测 envdetect.go

**Files:**
- Create: `internal/tui/envdetect.go`
- Test: `internal/tui/envdetect_test.go`

**Interfaces:**
- Consumes: 无(纯新增)。
- Produces: 包级变量 `k8sProbe func() bool`、`remoteProbe func() bool`;函数 `availableSourceTabs() []int`(返回全局索引切片,升序)。Task 2 的 NewApp 依赖 `availableSourceTabs()`。

- [ ] **Step 1: 写失败测试**

创建 `internal/tui/envdetect_test.go`:

```go
package tui

import (
	"reflect"
	"testing"
)

// withProbes 覆盖环境探测变量(测试后恢复),避免真机 kubectl 有无影响测试结果。
func withProbes(t *testing.T, k8s, remote bool) {
	t.Helper()
	oldK8s, oldRemote := k8sProbe, remoteProbe
	k8sProbe, remoteProbe = func() bool { return k8s }, func() bool { return remote }
	t.Cleanup(func() { k8sProbe, remoteProbe = oldK8s, oldRemote })
}

func TestAvailableSourceTabsAll(t *testing.T) {
	withProbes(t, true, false)
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{0, 1, 2, 3}) {
		t.Fatalf("全可用环境可见集 = %v, want [0 1 2 3]", got)
	}
}

func TestAvailableSourceTabsNoKubectl(t *testing.T) {
	withProbes(t, false, false)
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("无 kubectl 可见集 = %v, want [1 2 3]", got)
	}
}

func TestAvailableSourceTabsRemoteOverridesK8s(t *testing.T) {
	withProbes(t, true, true) // 远端优先:即使本机有 kubectl 也只留本地
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("远端会话可见集 = %v, want [1]", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tui/ -run TestAvailableSourceTabs -v`
Expected: FAIL,报 `undefined: k8sProbe`(或 `availableSourceTabs`)

- [ ] **Step 3: 写实现**

创建 `internal/tui/envdetect.go`:

```go
package tui

import (
	"os"
	"os/exec"
)

// 源选择器环境探测(包级函数变量,测试可注入;沿用 startFRPTunnel 等注入惯例)。
// tab 全局索引:0=K8s 1=本地 2=SSH 3=FRP。
var (
	// k8sProbe 本机是否可用 k8s(kubectl 在 PATH 即视为可用)。
	k8sProbe = func() bool {
		_, err := exec.LookPath("kubectl")
		return err == nil
	}
	// remoteProbe 是否处于 ssh/frp 进入的远端会话(隧道本质也是 ssh,SSH_TTY 天然覆盖)。
	remoteProbe = func() bool {
		return os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != ""
	}
)

// availableSourceTabs 环境感知的源选择器可见 tab 集(全局索引,升序)。
// 远端会话只留本地;本机无 kubectl 去 K8s;其余全量。
func availableSourceTabs() []int {
	if remoteProbe() {
		return []int{1}
	}
	if !k8sProbe() {
		return []int{1, 2, 3}
	}
	return []int{0, 1, 2, 3}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/tui/ -run TestAvailableSourceTabs -v`
Expected: 3 个测试全 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/envdetect.go internal/tui/envdetect_test.go CLAUDE.md docs/superpowers/specs/2026-09-04-source-picker-env-design.md docs/superpowers/plans/2026-09-04-source-picker-env.md
git commit -m "feat(tui): 源选择器环境探测——kubectl/远端会话可见集函数"
```

(首次提交顺带收录 spec 与本计划文档)

---

### Task 2: App 可见集字段与入口钳制

**Files:**
- Modify: `internal/tui/app.go:121-123`(App 结构体字段区,`sourceTab` 字段处)
- Modify: `internal/tui/app.go:205-226`(NewApp 的 `&App{...}` 字面量)
- Modify: `internal/tui/app_test.go:24`(newTestApp)
- Modify: `internal/tui/sourcepicker_ui.go:23-25`(openSourcePicker 开头)
- Test: `internal/tui/sourcepicker_test.go`(文件末尾追加)

**Interfaces:**
- Consumes: Task 1 的 `availableSourceTabs() []int`。
- Produces: `App.availableTabs []int` 字段;方法 `clampSourceTab(tab int) int`(不可见回落 `availableTabs[0]`)。Task 3/4 依赖 `a.availableTabs`。

- [ ] **Step 1: 写失败测试**

在 `internal/tui/sourcepicker_test.go` 末尾追加:

```go
// K8s 不可见时 openSourcePicker(0) 应钳制到首个可见 tab;可见 tab 原样保留。
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tui/ -run TestOpenSourcePickerClampsHiddenTab -v`
Expected: 编译 FAIL,`app.availableTabs undefined`(field 不存在)

- [ ] **Step 3: 写实现**

① `internal/tui/app.go` App 结构体,`sourceTab` 字段(约 122 行)下加一行:

```go
	sourcePickerMode bool
	sourceTab        int      // 0=K8s 1=本地 2=SSH 3=FRP
	availableTabs    []int    // 环境可见 tab 集(NewApp 探测一次;空集防御时回本地)
	pickSourceOnStart bool    // 启动即打开源选择器(picker 子命令)
```

② `internal/tui/app.go` NewApp 的 `&App{...}` 字面量(约 224 行 `bookmarks:` 行后)加:

```go
		bookmarks:      make(map[uint64]bool),
		availableTabs:  availableSourceTabs(),
```

③ `internal/tui/sourcepicker_ui.go` openSourcePicker 开头(约 24 行 `a.sourcePickerMode = true` 之前)插入钳制与新方法:

```go
// openSourcePicker 打开选择器(tab 可预选;不可见时钳制到首个可见 tab)。
func (a *App) openSourcePicker(tab int) {
	a.sourceTab = a.clampSourceTab(tab)
	a.sourcePickerMode = true
	// ...(原有函数体不动,删掉原来的 a.sourceTab = tab 一行)
```

```go
// clampSourceTab tab 不在可见集时回落首个可见 tab(可见集恒含本地 tab 1)。
func (a *App) clampSourceTab(tab int) int {
	for _, t := range a.availableTabs {
		if t == tab {
			return tab
		}
	}
	if len(a.availableTabs) == 0 {
		return 1 // 防御:NewApp 未初始化/空集时至少本地可用
	}
	return a.availableTabs[0]
}
```

④ `internal/tui/app_test.go` newTestApp,`app.width/height` 赋值后加固定全量(防止真机 kubectl 有无影响存量测试):

```go
func newTestApp() *App {
	app := NewApp(&mockStream{}, nil, 1000, nil)
	app.width = 120
	app.height = 40
	app.availableTabs = []int{0, 1, 2, 3} // 固定全量:环境过滤由专门测试覆盖
	// ...
```

- [ ] **Step 4: 跑测试确认通过(含存量)**

Run: `go test ./internal/tui/ -run TestOpenSourcePickerClampsHiddenTab -v`
Expected: PASS
Run: `go test ./internal/tui/ -run 'TestSourcePicker|TestView|TestOpen' -v`
Expected: 存量测试全 PASS(newTestApp 固定全量,行为不变)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go internal/tui/sourcepicker_ui.go internal/tui/sourcepicker_test.go
git commit -m "feat(tui): 源选择器入口钳制——不可见 tab 落首个可见 tab"
```

---

### Task 3: Tab 键可见集内循环

**Files:**
- Modify: `internal/tui/sourcepicker_ui.go:189-194`(handleSourcePickerKeys 的 Tab/ShiftTab 分支)
- Test: `internal/tui/sourcepicker_test.go`(末尾追加)

**Interfaces:**
- Consumes: Task 2 的 `a.availableTabs []int`。
- Produces: 方法 `cycleSourceTab(dir int) int`(dir=1 正向/-1 反向,可见集内循环)。

- [ ] **Step 1: 写失败测试**

```go
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
```

(文件头部 import 已有 `tea`;若无按编译提示补 `"github.com/charmbracelet/bubbletea"`)

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tui/ -run TestSourcePickerTabCycle -v`
Expected: FAIL,`FRP(3) Tab 后应到本地(1)... got 0`(旧 `%4` 落到隐藏的 0)

- [ ] **Step 3: 写实现**

`internal/tui/sourcepicker_ui.go` handleSourcePickerKeys(约 189-194 行)替换:

```go
	case tea.KeyTab:
		a.sourceTab = a.cycleSourceTab(1)
		return a, a.pickerTabEnterCmd()
	case tea.KeyShiftTab:
		a.sourceTab = a.cycleSourceTab(-1)
		return a, a.pickerTabEnterCmd()
```

同文件加方法(放 handleSourcePickerKeys 后):

```go
// cycleSourceTab 可见集内循环切换 tab(dir=1 正向,-1 反向);当前 tab 不在可见集时回落首个。
func (a *App) cycleSourceTab(dir int) int {
	if len(a.availableTabs) == 0 {
		return a.sourceTab
	}
	for i, t := range a.availableTabs {
		if t == a.sourceTab {
			return a.availableTabs[(i+dir+len(a.availableTabs))%len(a.availableTabs)]
		}
	}
	return a.availableTabs[0]
}
```

- [ ] **Step 4: 跑测试确认通过(含存量 4-tab 循环)**

Run: `go test ./internal/tui/ -run TestSourcePickerTabCycle -v`
Expected: PASS
Run: `go test ./internal/tui/ -run 'TestFRP' -v`
Expected: PASS(存量 4 tab 循环测试在默认全量下不受影响)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/sourcepicker_ui.go internal/tui/sourcepicker_test.go
git commit -m "feat(tui): 源选择器 Tab 循环限定可见集,跳过隐藏 tab"
```

---

### Task 4: tab 栏按可见集渲染

**Files:**
- Modify: `internal/tui/sourcepicker_ui.go:977-986`(buildSourcePickerLines 的 tab 栏循环)
- Test: `internal/tui/sourcepicker_test.go`(末尾追加)

**Interfaces:**
- Consumes: Task 2 的 `a.availableTabs []int`。
- Produces: 无新接口(渲染内部变化)。

- [ ] **Step 1: 写失败测试**

```go
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tui/ -run TestSourcePickerLinesHides -v`
Expected: FAIL,`远端会话 tab 栏不应包含 K8s`(旧渲染画全量 4 tab)

- [ ] **Step 3: 写实现**

`internal/tui/sourcepicker_ui.go` buildSourcePickerLines(约 977 行)替换 tab 栏循环:

```go
	tabNames := []string{"K8s", "本地", "SSH", "FRP"}
	var tabParts []string
	for _, i := range a.availableTabs {
		if i == a.sourceTab {
			tabParts = append(tabParts, PopupActiveTabStyle.Render(" "+tabNames[i]+" "))
		} else {
			tabParts = append(tabParts, PopupTabStyle.Render(" "+tabNames[i]+" "))
		}
	}
```

- [ ] **Step 4: 跑测试确认通过(含存量渲染断言)**

Run: `go test ./internal/tui/ -run TestSourcePickerLinesHides -v`
Expected: PASS
Run: `go test ./internal/tui/ -run 'TestSourcePicker' -v`
Expected: PASS(存量测试 newTestApp 全量 4 tab,渲染不变)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/sourcepicker_ui.go internal/tui/sourcepicker_test.go
git commit -m "feat(tui): 源选择器 tab 栏按环境可见集渲染"
```

---

### Task 5: picker Short 补正 + 全量回归

**Files:**
- Modify: `cmd/root.go:46`(pickerCmd.Short)
- Test: 无新增(回归验证)

**Interfaces:**
- Consumes: 无。
- Produces: 无。

- [ ] **Step 1: 改 Short 描述**

`cmd/root.go` 约 46 行:

```go
	Use:   "picker",
	Short: "Open TUI with the source picker (k8s/local/ssh/frp)",
```

- [ ] **Step 2: 编译检查**

Run: `go build ./...`
Expected: 无输出(编译通过)

- [ ] **Step 3: 全量回归**

Run: `go test ./internal/tui/ -count=1`
Expected: 全 PASS(现有 ~tui 包测试 + 本计划新增测试)
Run: `go test ./... -count=1`
Expected: 全 PASS(FRP 集成测试带 build tag 隔离,不会真跑)

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "docs(cmd): picker Short 描述补 frp 源"
```

---

## 验收对照(spec → task)

| spec 要求 | task |
|---|---|
| 无 kubectl → 隐藏 K8s tab | Task 1(探测)+ Task 4(渲染)|
| SSH_TTY/SSH_CONNECTION → 仅本地 tab | Task 1(探测)+ Task 4(渲染)|
| 三入口(picker/裸 logview/o 键)一致 | Task 2(钳制统一在 openSourcePicker)|
| 全可用环境行为不变 | Task 2 Step 4 存量回归 |
| 单测覆盖环境→可见集映射 | Task 1(纯函数)+ Task 3(循环)+ Task 4(渲染)|
| pickerCmd Short 失真补正 | Task 5 |
