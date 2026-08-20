# TUI 界面整体优化 实现计划

> 关联 spec: ../specs/TUI界面优化.md
> 全局 gate：每个 Task 结束 `go build ./... && go test ./...` 全绿。

### Task 1: 详情栏只显示非空字段
- 改：`internal/tui/detail.go`（renderDetailBar 空字段跳过而非渲染 `-`）
- 验证：`go test ./internal/tui/` + 新增用例：空字段行渲染后不含 "time: -"
- [ ] 完成

### Task 2: 标题栏增强（级别统计 + 滚动百分比）
- 改：`internal/tui/app.go`（View 标题拼接、levelCounts map 增量维护于 processLine/recomputeView、百分比取 cursor/total）、`app_test.go`
- 布局：`LogView ─ 跟踪中 [plain-text] ─ E:3 W:30 ─ 50条 ─ 62%`
- 验证：单测统计与百分比格式；tmux 截图对比
- [ ] 完成

### Task 3: 弹窗统一（搜索/高亮/隐藏 → 单一三 tab inline 弹窗）
- 改：`internal/tui/app.go`（h/x 按键改 searchTab=1/2 + searchMode=true；删 highlightMode/hideMode 分支）、`internal/tui/searchbar.go`（栏前缀随 tab：搜索:/高亮:/隐藏:）、删除 `highlightpopup.go`、`hidepopup.go`
- 统一规格：boxW=min(60,width-4)、tab 栏、footer hint 同 searchpopup 现有样式
- 验证：`go build ./... && go test ./...`（改写涉及旧 mode 的用例）+ tmux 依次按 `/`、`h`、`x` 截图对比三 tab 一致
- [ ] 完成

### Task 4: 帮助栏拆分（快捷键栏可隐藏 + 状态栏常驻）
- 改：`internal/tui/helpbar.go`（helpItems 拆 shortcutItems/statusItems，隐藏徽章改 `[隐藏:N词]` 零计数不显示）、`internal/tui/app.go`（新增 showKeyHints 状态 + `\` 切换键、View 渲染两栏、visibleLines 适配）、`keymap.go`
- 状态栏内容：`[过滤:ERROR] [隐藏:4词] [搜索:xxx] [匹配:N条]` + 复制反馈
- 验证：`go test ./internal/tui/` + 新增 `\` 切换用例 + tmux 截图（显示/隐藏两态）
- [ ] 完成

### Task 5: 帮助弹窗双栏 + 高度自适应
- 改：`internal/tui/helppopup.go`（≥100 列分组切两半双栏；超出 vl 按分组边界截断并加提示）
- 验证：`go test ./internal/tui/` + tmux 以 80x24 / 180x40 两种尺寸截图验证不丢分组
- [ ] 完成

### Task 6: ERROR/FATAL 背景色带
- 改：`internal/tui/style.go`、`internal/tui/theme.go`（LevelError 深红背景白字；Light 主题浅红背景深字）
- 验证：`go test ./...` + tmux 截图确认选中行不冲突
- [ ] 完成

### Task 7: README + 截图更新
- 改：`README.md`（按键表加 `\` 切换；弹窗交互说明）、`docs/screenshot.png`
- 验证：人工核对
- [ ] 完成
