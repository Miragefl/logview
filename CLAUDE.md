# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

LogView 是 Go 编写的终端日志查看器(TUI):支持本地文件 / 管道 / SSH / K8s / FRP 隧道多日志源,自动解析 java-logback / JSON / 纯文本格式。文档与注释使用中文。

## 知识库归属

- 项目:logview(`~/Obsidian/10-Projects/logview/`,个人开源项目,无 DB/接口交付物豁免)
- 分支映射:main → grep-context(grep 上下文混入+管道 DSL,已完成,v0.17.4 发布,含 2026-09-14~15 验收迭代:sticky 多锚混入/主界面 j/k;2026-10-09 修正连按合并输入被吞,v0.18.1 发布);main → upgrade-exdev-fix(自升级跨设备 EXDEV 修复:rename 失败兜底同目录 copy+rename,2026-10-10);main → keybind-unify(导航键统一 C-j/C-k,已完成,v0.17.2 发布,主日志界面已于 v0.17.4 改回 j/k,弹窗类界面仍 C-j/C-k);main → search-input-perf(搜索输入性能优化,已完成,v0.17.2 发布);main → gzip-log-support(gzip 日志支持,已完成,v0.17.0 发布);main → keyword-history-freq(关键词历史频次+拆词统计+源 scope 隔离,已完成,v0.17.0 发布,含 2026-09-08 历史追加修正 v0.17.1 发布);main → source-picker-env(源选择器环境过滤,已完成,v0.16.0 发布);main → time-range-search(时间范围搜索 time: 语法,已完成,v0.16.0 发布);main → tui-overhaul(TUI 界面翻新,已完成);main → search-hl-split(搜索命中与高亮词视觉区分:高亮词=10 色亮色系粗体下划线;2026-10-12 迭代:搜索命中弃黄底块改暗色系 8 色轮换粗体,明暗双域区分,v0.19.0 发布);main → perf-1g-load(1G 大文件加载性能迭代,v0.18.0 发布九迭代:列存模型+通道批量化+尾读/--all+分帧搜索刷新+状态栏内存/加载徽章,1G/664 万行加载 300s→28s、--all RSS 1.8G→2.3G、默认档 71MB;2026-10-11 迭代 10 内存再压缩:默认档 GOGC 50+FreeOSMemory+徽章 Sys-HeapReleased 口径+base 去重驻留 --all 活对象再 -106MB;2026-10-12 迭代 11 mmap 零拷贝:≥64MB 文件全量读原文出堆直指映射区,5G/3325 万行 --all 真实占用(phys_footprint)5.0G vs 改前 ~11G 独占;迭代 12 搜索分片 8 worker 并行(664 万行全扫 25s+→≤3s,打字不卡);迭代 13 加载解析并行化(1G 28s→15s、5G 176s→74s 完全线性,真凶系正则串行非渲染);另有混入视口跟锚修复(锚定后 zz 居中一次+连续命中段提示),v0.19.0 发布)。历史需求(initial-build/multi-k8s-resource/search-syntax-enhancement/search-history-popup/frp-stcp-source/source-picker-v2/code-refactor/builtin-themes)已归档 vault,开工前先 ls 需求目录; feature/fview-desktop → main(fview 桌面版 Wails MVP:M1-M3 已完成——logcore 抽取/合仓/发布双产物/tab 会话/虚拟滚动/follow/gzip/搜索/过滤/共享 rules.yaml/主题,Final Review 修复 pass 后 13 包+race 全绿;M4-M7 连接管家/FRP/k8s/打磨待续;详见 vault 10-Projects/fview); feature/fview-ui-vue → fview(fview 桌面版 UI 验收迭代,续用 10-Projects/fview:2026-10-30 第二轮 8 项——k8s/FRP 面板弹窗交互 bug/TUI 式分栏/搜索+高亮+隐藏统一弹窗/vim 键全量回归/主题日志字段配色/顶栏文件名+源信息/设置图标统一/字体下拉宽度);main → titlebar-merge(mac 标题栏融合 tab 栏 IDEA 式:HiddenInset+78px 红绿灯让位+drag-spacer,2026-11-03 spike 验收通过;含同日字号放大布局修复——写死 px 容器全改 --ui-font-size 派生,及关于页版本注入/检测更新 gitee 回落);main → edit-menu-paste(mac ⌘V 粘贴修复:自定义应用菜单替换 Wails 默认菜单后无 Edit 菜单,WKWebView paste: 无人认领致 FRP 表单等所有输入框粘不了,menu.go 补 menu.EditMenu() 官方 Role,2026-11-04,续用 10-Projects/fview;含同日 TUI FRP 新建表单 q 误退修复:q-退出只认输入框空、表单字段每步从空起,首字母 q 直接杀 TUI,sourcepicker_ui q-退出排除表单层 pickerFRPLevel==1);main → panel-search-unify(源面板搜索+新增统一 r1+r2:r1 PanelSearch 常驻框/⌃a 新增统一/SSH 弹窗/⌃c⌃n+Final Review 两修;r2 真机验收改无框打字过滤+高亮+Esc 逐级(panelFilter 会话级共享)/d/e→⌃d⌃e/HelpModal 分组双列,302→309 测试绿+Final Review 四修(I1 模板残迹>/q? 元键放行/⌃c⌃n 无条件消费)+子序列匹配(hde→hardware)+K8s 多资源(Space 多选/Enter 合流 ⌘Enter 各开/来源列,MultiK8sSource 存量接线+Final Review 六修(chip 定宽/hasSource 来源不唯一判定/合流失败提示/动作清标记等),315 测试绿),2026-11-04,续用 10-Projects/fview);main → vim-copy(字符级光标+vim 移动+可视选择复制双端:h/l/w/b/e/0/$/^ 字符光标、v 字符选择+V 行级、yy 整行/visual 内 y 选区、fview 拖选字符级松手即复制;键位腾挪:高亮→ctrl+h、折叠→ctrl+e、换行→ctrl+w(fview ctrl+w 弃关 tab,mac ⌘W 原生菜单保留,Win/Linux 关 tab 只剩 tab 栏 ×);TUI 不做鼠标保持终端原生;2026-11-05 r1 十任务+Final Review(opus)五修完成(临时 wrap 死代码根治/直入 v 锚列/选区行强制 wrap/窗外松手终点/visual 连按 y),双端测试全绿 fview 369+Go 15 包,charMotion/charnav 同用例表对齐,fview.app md5 e766a3da+logview md5 93b56e11 待真机验收;2026-11-05 r2 真机验收三项:隐藏键 x→ctrl+x 双端同步(顺手修 ctrl+e 孤儿弹窗残留)/WordsPopup 删词改 ctrl+d(根因=自动聚焦 input+!inInput 防御真机恒失效,测试 dispatch target 恒 document 伪造不出,补 keyOnInput 测试)/k8s 面板 context/ns NSelect tiny→small,fview 371+Go 全绿,fview.app md5 ba341c34+logview md5 b9b594d2 真机验收通过,v0.22.0 发布,独立需求目录 10-Projects/fview/vim-copy)

## 常用命令

```bash
make build              # 构建单文件二进制 logview(ldflags 注入 main.version/commit/date)
go build ./...          # 纯编译检查
go test ./...           # 全量测试(~187 个 Test 函数,几十秒)
go test ./internal/tui/ -run TestXxx -v   # 跑单个测试(标准 go test,无外部断言库)

# FRP 集成测试(build tag 隔离,需真实 frpc + 可达 frps + FRPS_ADDR/FRPS_TOKEN/FRP_SK/FRP_PROXY 环境变量)
go test -tags frpintegration ./internal/frp/ -run TestStartTunnelReal -v
```

- 无 lint 配置(无 .golangci.yml),无测试类 CI;`.github/workflows/` 仅有 Gitee 镜像同步。
- 发布走 `script/release.sh [patch|minor|major]`(goreleaser + brew tap);日常体验版构建见 PROJECT-CONTEXT.md 的 test_build_cmd。
- fview 桌面版本地验收包:`script/build-fview.sh`(git describe 推版本注入 ldflags——裸 `wails build` 的 version 是空串,关于页只显示 dev;脚本自动删旧包重拷 `~/Documents/` 并 md5 双端核对;`--keep` 只构建不交付)。

## 架构

### 数据流

```
LogStream.Start() → chan model.RawLine → tui.App.Update(batchMsg)
  → parser.AutoDetect 探测 → Parser.Parse → buffer.RingBuffer(容量 = --buffer-size)
  → 过滤/搜索 → logview.go 渲染 → View()
```

### 入口与装配

- `main.go` 只声明版本变量 + 委托 `cmd.Execute()`;**全部 CLI 在 `cmd/root.go` 单文件**:子命令 `picker / k8s / tail / file / pipe / upgrade / completion`,裸 `logview` 在 `Execute()` 里自动补子命令(stdin 是管道 → pipe,TTY → picker)。
- `expandTailArgs` 支持 `-200f` 语法糖(展开为 `--tail 200 -f`)。
- `runTUI()`(root.go)是统一装配点:`tui.NewApp` + AltScreen,退出时 `frp.KillAllTunnels()` 兜底。
- Persistent flags:`--rule`(指定解析规则,空则自动探测)、`--buffer-size`(默认 100000)、`--config`(配置目录,默认 `~/.config/logview`)。

### internal/ 各包职责

| 包 | 职责 |
|---|---|
| `stream/` | `LogStream` 接口(`Start/Label/Cleanup`,stream.go)+ 各日志源。K8s 与 SSH 均 **exec 系统命令**(`kubectl`/`ssh`),不用 SDK;`sshconfig.go` 解析 `~/.ssh/config` |
| `parser/` | `Parser` 接口 + `RegexParser`(命名捕获组)/ `JSONParser`;`rules.go` 加载 rules.yaml、`AutoDetect` 按源逐条探测(pending 每源最多 50 行,超限回落 plain-text,后续命中可升级解析器并回灌重解析) |
| `model/` | `RawLine{Text, Source, Seq}` → `ParsedLine`(7 个标准 Field,`FieldMask` 控制显隐,`AllFields` 可被配置覆盖) |
| `buffer/` | 定容环形缓冲存 `*ParsedLine`,滚动窗口内存边界 |
| `tui/` | **`app.go` 的 `App` 是唯一 tea.Model**;`logview.go` 是其渲染层(视觉行/换行/ANSI);`ReplaceStream` 支持源热切换;`sourcepicker*.go` 源选择器,`sshpw.go` SSH 密码认证重连 |
| `frp/` | `tunnel.go` 拉起 frpc 子进程建 stcp visitor 隧道(要求 frp ≥ v0.52 TOML);`store.go` 持久化到 `~/.local/state/logview/frp.json`。`stream/frp.go` 的 FRPSource = 隧道 + SSH tail |
| `export/` `stacktrace/` `upgrade/` | 选中行导出;Java 堆栈折叠;自升级(brew 安装则委托 `brew upgrade`) |

### 配置与状态文件

- 配置:`~/.config/logview/rules.yaml`(首跑自动落盘默认配置;schema 见 `parser/rules.go` 的 `rulesFile`,含 patterns/rules/fields/theme/hides/ssh_hosts)。
- 运行时状态:`~/.local/state/logview/{frp.json, session.json, usage.json}`(usage 是源选择器 frecent 热点,7 天半衰期)。

## 测试风格

- TUI 测试是纯单元测试(不用 teatest):`newTestApp()` 直接构造 App + mockStream,直接调 `processLine`/`Update`,断言 `View()` 字符串输出。参考 `internal/tui/app_test.go` 头部。
- `internal/tui/main_test.go` 的 `TestMain` 把 HOME 重定向到临时目录,防止污染真实 usage.json/session.json——新写 tui 测试依赖此隔离。
- 共享测试 fixture 在 `testutil/`(如 `JavaLogbackLines()`)。

## 注意事项

- **搜索/grep 时务必排除 `.worktrees/`**——里面是同名副本 worktree,不排除则每个文件命中两遍。
- skill 产物(spec/plan/design)一律写入 vault 需求目录 `specs/`/`plans/`(命名 `YYYY-MM-DD-主题.md`),禁止写入代码仓库;历史 23 份已于 2026-09-04 迁入 vault 各需求目录,代码仓库 `docs/` 仅存 README 截图。
- 版本注入变量在 `main.go`(`main.version/commit/date`),Makefile 与 .goreleaser.yml 的 ldflags 必须一致。
