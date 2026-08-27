# frp stcp 日志源设计

日期：2026-08-25
状态：已确认（方案 A：frpc visitor 子进程）

## 背景与目标

用户通过 frp stcp 隧道 SSH 到远端客户端查看日志。当前源选择器弹窗（按 `o`/`q` 打开）只有 K8s/本地/SSH 三个 tab，frp 连接需要手动起 visitor、手动挑端口，体验割裂。

目标：在源选择器中新增 **FRP** tab：

- 表单收集 frps 地址（可从已保存选择或新增）、token/sk、proxy 名称、ssh 用户名
- 本地 bindPort 自动分配，支持同时连接多个 frpc visitor
- 首次连接：浏览远程目录选日志文件 → tail；确认时把「frp 参数 + 日志路径」整体存为一条记录
- 之后：搜索/选中旧记录直接 tail
- frp 配置独立于 `~/.ssh/config`

非目标：

- 不引入 frp Go 库（保持项目 exec 系统工具的风格）
- 不把 frp 参数写入 rules.yaml（那是用户手改的配置）或 `~/.ssh/config`
- ssh 密码仍仅内存缓存，不落盘（沿用现有约定）

## UI 交互（frp tab 层级状态机）

Tabs 由 `{"K8s", "本地", "SSH"}` 变为 `{"K8s", "本地", "SSH", "FRP"}`，tab 切换取模 `% 3` → `% 4`，`sourceTab=3` 为 frp。

frp tab 内三级交互：

- **L0 连接列表**：顶部固定 `+ 新建连接` 条目，其下为已保存记录（按使用频次排序、热点 ★ 标记，复用 `sortCandidatesHot`）。顶部输入框为搜索框（复用现有过滤机制），按记录名/proxy 名/服务器名过滤。选中旧记录 Enter → 按已存参数建隧道进 L2 目录浏览（起始目录 = 已存 Path 父目录；2026-08-25 交互修正：统一进浏览，不再直达 tail）。
- **L1 新建表单**：依次输入/选择：
  1. frps 服务器 — 已存服务器列表可选；滚到底输入新地址则新增（随后输入 token，一并保存进服务器列表）
  2. sk（secret key，与远端 stcp 的 sk 一致）
  3. proxy 名称（远端 stcp 服务名，即 server-name）
  4. ssh 用户名
  表单提交后**立即保存记录**（隧道打通前；中途放弃不丢配置，L0 可直接重选），随后建隧道进入 L2。
- **L2 远程目录浏览**：与 SSH tab 一致（Enter 进目录/选文件，Backspace 逐级返回）。确认日志文件时保存整条记录（默认名 = proxy 名），随后开始 tail。

## 数据结构与存储

新状态文件 `~/.local/state/logview/frp.json`（与 usage.json/session.json 同目录，app 自动读写）。新增 `internal/frp/store.go`：

```go
type Server struct { Name, Addr, Token string } // Addr: host:port
type Conn struct { Name, Server, SK, Proxy, User, Path string }
type Store struct { Servers []Server; Conns []Conn }
```

- 启动时加载，`root.go` 注入 TUI（仿 `SetSSHHosts` 的 `SetFRPStore`）；记录/服务器变更即写回
- 使用频次走现有 `BumpUsage("frp:"+name)`，与 ssh/k8s 同一套排序机制
- 删除：L0 连接列表 / L1 服务器列表按 `C-x` 删除光标项（`+new`/`+manual` 占位项除外）；服务器仍被连接引用时拒绝删除并提示
- sk/token 落本机状态文件（与 rules.yaml 存 ssh_hosts 同等信任级别）；ssh 密码不落盘

## 隧道管理（internal/frp/tunnel.go）

```go
func StartTunnel(server Server, sk, proxyName string) (*Tunnel, error)
// Tunnel 提供 LocalPort() 与 Cleanup()
```

- **端口分配**：`net.Listen("tcp", "127.0.0.1:0")` 取空闲端口后关闭，传给 visitor；多隧道各占一端口互不冲突
- **复用与生命周期**：`AcquireTunnel` 按参数（服务器+token+proxy+sk）注册表复用存活隧道（引用计数，归零才杀 frpc）；`KillAllTunnels` 进程退出兜底强杀（cmd/root.go p.Run 后调用）
- **visitor 启动**：生成临时 TOML 配置（`serverAddr/serverPort/token` + visitor 块 `name/proxyName/sk/bindPort`），exec `frpc -c <tmpfile>`。选临时配置文件而非 CLI flags：frpc CLI 不一定暴露全部 visitor 字段（如 sk），TOML 方式跨版本最稳（要求 frp ≥ v0.52）
- **就绪探测**：启动后每 200ms 尝试 TCP 连 `127.0.0.1:bindPort`，10s 超时；同时监控 frpc stderr，出错立即失败返回
- **清理**：`Cleanup()` 杀 frpc 进程并删临时配置；`FRPSource.Cleanup()` 串联调用（流切换/退出自动清理）

## 建流（internal/stream/frp.go）

`FRPSource` 组合 `Tunnel` + 复用 `SSHSource`：

- `SSHSource` 增加可选 `port` 字段：`Start`/`SSHListDir` 拼命令时加 `-p <port>`，host 为 `user@127.0.0.1`——对现有 SSH 代码的唯一侵入性改动
- tail 逻辑、stderr 错误上屏、密码弹窗（`maybePromptSSHPassword`）原样复用
- 目录浏览期间隧道由 app 持有常驻（浏览与 tail 共用），确认建流后移交 `FRPSource` 管理

## 错误处理

| 场景 | 行为 |
|---|---|
| `frpc` 不在 PATH | 明确报错「未找到 frpc，请先安装」 |
| 隧道超时/启动失败 | ERROR 行上屏 frpc stderr 关键行，清理残留进程 |
| ssh 认证失败 | 复用现有密码弹窗（内存缓存） |
| frp.json 损坏/不可写 | 降级为空列表继续运行，界面提示但不崩溃 |

## 改动面清单（现有代码）

- `internal/tui/sourcepicker_ui.go`：tabs 数组、`% 3` 取模、8 处 `switch a.sourceTab` 分支（visiblePickerCandidates / pickerTabEnterCmd / pickerBackspace / pickerEnter / pickerInputRef / confirmSourcePicker / buildSourcePickerLines）
- `internal/tui/app.go`：frp 弹窗状态字段、`candidatesMsg` 路由
- `internal/tui/sourcepicker.go`：frp 候选数据与异步命令
- `internal/stream/ssh.go`：`SSHSource` 增加 port 字段
- `cmd/root.go`：启动时注入 frp store

## 测试

- `store.go` 读写/覆盖保存单测
- 端口分配、TOML 生成、ssh 命令拼装（含 `-p`）单测——不真跑 frpc/ssh
- 隧道就绪探测用 fake listener 单测；真实 frpc 集成测试加 build tag，无环境自动跳过
