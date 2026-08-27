# FRP 选择器交互修正（选已存连接进目录浏览）

## 目标
FRP 界面两条路径统一收敛到远程目录浏览：选已存连接 = 用已存参数建隧道 SSH 登录后**进目录浏览**（不再直达 tail 固定路径）；新建连接 = 先选 frps 服务器列表再逐项填写（维持现状）。

## 约束
- Go + bubbletea，无新依赖
- 不改 `frp.json` 数据结构；`Conn.Path` 语义调整为「浏览起始目录提示 + 记录展示」
- 不动 `internal/frp`（tunnel/store 现状满足）

## 模块拆解
- `internal/tui/sourcepicker.go`：`fetchFRPTunnelCmd` 去掉 `browse` 参数（两条路径行为一致）
- `internal/tui/sourcepicker_ui.go`：L0 已存记录 Enter 改走浏览建隧道
- `internal/tui/app.go`：`frpTunnelMsg` 分支统一——删直达 tail 分支，起始目录取 `conn.Path` 父目录（无 Path 则 `/`）
- `internal/tui/frppicker_test.go`：直达用例改浏览断言，补起始目录用例

## 关键决策
- **选已存 → 进目录浏览**（用户确认）：已存路径失效不再直接失败，且换文件看更灵活
- **起始目录 = `conn.Path` 父目录**：`/var/log/a.log` → `/var/log`，兼顾直达效率（一两次按键可达常用目录）
- **删 `browse` 参数**：表单新建与已存记录打开行为完全一致，分支无保留价值
- `confirmFRPPicker` 不变：选完文件仍更新记录 `Path`（下次浏览起始目录更准）

## 风险
- 失去一键直达 tail → 起始目录用已存 Path 父目录缓解
- 隧道建立等待（最长 10s）→ 维持现有 loading 态，不新增处理
