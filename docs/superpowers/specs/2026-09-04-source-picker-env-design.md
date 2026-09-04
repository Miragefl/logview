# 源选择器环境过滤 设计文档

- 日期:2026-09-04
- 状态:已确认(brainstorming 产出)
- 需求卡:`~/Obsidian/10-Projects/logview/source-picker-env/README.md`

## 背景与问题

源选择器(`o` 键 / `picker` 子命令)4 tab(K8s/本地/SSH/FRP)硬编码全量显示:

- K8s 无预检测:本机无 kubectl 时打开 K8s tab 异步等 6s 超时才报错,死入口体验稀烂。
- ssh/frp 进入的远端服务器上跑 logview,再嵌套选 SSH/FRP/K8s 源无意义,只该看本机文件。

## 需求

1. 本机不支持 k8s(`exec.LookPath("kubectl")` 失败)→ 不显示 K8s tab。
2. logview 进程处于 ssh/frp 进入的远端会话(`SSH_TTY` 或 `SSH_CONNECTION` 环境变量非空;frp 隧道本质为 ssh over frp,天然覆盖)→ 只显示本地 tab。
3. `picker` 子命令 / 裸 logview TTY 启动 / `o` 键热切换三条入口行为一致。
4. 全部源可用的本机环境行为不变(4 tab 照常)。

## 方案:可见层过滤

内部全局 tab 索引(0=K8s 1=本地 2=SSH 3=FRP)保持不变,新增"可见集"概念,只在入口钳制、Tab 循环、tab 栏渲染三处生效;候选回填 `candidatesMsg.tab` 与 `confirmSourcePicker` 的 switch 均基于全局索引,零改动。

否决的备选:动态 tab 注册表重构(动 1200 行 UI 全部 switch 分支与测试,YAGNI)、tab 置灰显示(死入口违背"不要显示"初衷)、`--no-filter` 逃生 flag(无已知误判场景,YAGNI)。

## 组件设计

### 1. 环境探测 `internal/tui/envdetect.go`(新文件,~40 行)

包级函数变量(沿用项目 `startFRPTunnel`/`sshHostCandidates` 的可注入惯例):

```go
var (
    k8sProbe    = func() bool { _, err := exec.LookPath("kubectl"); return err == nil }
    remoteProbe = func() bool {
        return os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != ""
    }
)

// availableSourceTabs 环境感知的可见 tab 集(全局索引:0=K8s 1=本地 2=SSH 3=FRP)。
func availableSourceTabs() []int {
    if remoteProbe() { return []int{1} }      // 远端会话:只剩本地
    if !k8sProbe()  { return []int{1, 2, 3} } // 无 kubectl:去 K8s
    return []int{0, 1, 2, 3}
}
```

`NewApp` 计算一次存 `a.availableTabs []int` 字段。探测毫秒级、进程生命周期内环境不变(中途装 kubectl 重启即得),不做每次重探测。

### 2. 入口钳制(`sourcepicker_ui.go` openSourcePicker)

`openSourcePicker(tab)` 开头:tab 不在 `a.availableTabs` 则落 `availableTabs[0]`。

- `app.go:277/733/757` 的 `openSourcePicker(0)`(picker 启动/错误重开)自动落到本地 tab。
- `sshpw.go` 的 `openSourcePicker(3)/(2)`:SSH/FRP 流程入口已被隐藏则走不到,钳制纯防御。
- 三条入口零改动统一生效。

### 3. Tab 键循环(`sourcepicker_ui.go:189-193`)

`(a.sourceTab+1)%4` / `(a.sourceTab+3)%4` 改为可见集内循环:在 `a.availableTabs` 中找当前 tab 位置,±1 取模映射回全局索引;当前 tab 不在可见集(理论不发生)回落 `availableTabs[0]`。可见集恒非空(本地 tab 永远可用)。

### 4. tab 栏渲染(`sourcepicker_ui.go:977`)

`tabs := []string{"K8s", "本地", "SSH", "FRP"}` 按可见集过滤绘制,选中态判断基于全局索引不变。

## 错误处理

- LookPath 失败、环境变量缺失均为预期路径,无 error 面。
- 入口钳制后 K8s 隐藏时不主动拉取候选（入口处按 `a.sourceTab == 0` 守卫）；若曾有在途拉取，迟到的候选消息由 app.go 的 tab 守卫（`a.sourceTab != msg.tab`）丢弃，不产生 UI 影响。
- 可见集至少含本地 tab,钳制与循环无越界路径。

## 测试设计(纯单元,沿用 newTestApp() 风格)

- **关键决定**:`newTestApp()` 显式固定 `availableTabs = [0,1,2,3]`,避免测试行为取决于跑测试机器是否装 kubectl;现有测试(含 `frppicker_test.go` 的 4 tab 循环)零改动。
- 新增 `envdetect_test.go`:mock `k8sProbe`/`remoteProbe` 函数变量(t.Cleanup 恢复),断言 远端→`[1]`、无 kubectl→`[1,2,3]`、全可用→`[0,1,2,3]`。
- 新增过滤行为测试:`availableTabs=[1]` 时 `View()` 不含 "K8s"/"SSH"/"FRP" 字样、Tab 循环原地不动、`openSourcePicker(0)` 落本地 tab。

## 顺手项

- `pickerCmd` Short 描述 `(k8s/local/ssh)` 失真(漏 frp),顺手补正。

## 明确不做

- tab 注册表重构、置灰显示、逃生 flag、探测缓存刷新(YAGNI)。
