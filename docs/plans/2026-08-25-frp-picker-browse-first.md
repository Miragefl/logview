# FRP 选择器交互修正 实现计划

> 关联 spec: ../specs/2026-08-25-frp-picker-browse-first.md

### Task 1: 统一建隧道命令
- 改：`internal/tui/sourcepicker.go`（`fetchFRPTunnelCmd` 删 `browse` 参数及 `frpTunnelMsg.browse` 字段）、`internal/tui/sourcepicker_ui.go`（L0 已存记录、表单 step5 两处调用同步）
- 验证：`go build ./...`
- [ ] 完成

### Task 2: frpTunnelMsg 统一为浏览分支
- 改：`internal/tui/app.go`（删直达 tail 分支；起始目录 = `conn.Path` 父目录，无 Path 则 `/`，父目录提取加辅助函数）
- 验证：`go test ./internal/tui/ -run TestFRP -count=1`
- [ ] 完成

### Task 3: 测试更新与补充
- 改：`internal/tui/frppicker_test.go`（原直达 tail 用例改为进 L2 浏览断言；新增「已存 Path 父目录作起始目录」「Path 空起始 /」用例；清理 `browse` 字段引用）
- 验证：`go test ./... -count=1`
- [ ] 完成

### Task 4: 回归与构建
- 验证：`go vet ./...` + `go test ./... -count=1` + 按需更新设计文档 `docs/superpowers/specs/2026-08-25-frp-stcp-source-design.md` 中的直达描述
- [ ] 完成
