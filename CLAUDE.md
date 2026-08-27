# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

LogView 是 Go 编写的终端日志查看器(TUI):支持本地文件 / 管道 / SSH / K8s / FRP 隧道多日志源,自动解析 java-logback / JSON / 纯文本格式。文档与注释使用中文。

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
- 设计文档有两套并行体系:`docs/plans/` + `docs/specs/` 是 plan 工作流产物(spec 与任务清单同名配对);`docs/superpowers/{plans,specs}/` 是 superpowers/writing-plans 工作流的日期前缀文档(仍在用,如 2026-08-25-frp-stcp-source)。新工作按需选一套,别混放。
- 版本注入变量在 `main.go`(`main.version/commit/date`),Makefile 与 .goreleaser.yml 的 ldflags 必须一致。
