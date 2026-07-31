# 搜索历史列表（ctrl+r popup）设计

日期：2026-07-30
状态：已确认，待实现

## 背景

搜索模式下 ctrl+r 当前是 shell 风格的"循环切换"：每按一次把上一条历史填进搜索框，看不到全貌、不能直接挑选。用户希望改成"弹个列表，自己选"。

搜索历史来自 `addSearchHistory`（每次 Enter 确认搜索时记录，去重，最多 20 条，只存内存）。

## 目标

ctrl+r 弹出搜索历史列表，j/k（或 ↑/↓）上下选，Enter 选中填入搜索框，保留搜索弹窗可继续编辑。替换现有循环切换。

## 非目标（YAGNI）

- 历史持久化到 session（当前只存内存，重启清空，本次不做）
- ctrl+s 反向导航（shell 习惯，本次不做）
- 历史条数配置化（沿用硬编码上限 20）

## 设计

### 状态（App 新增字段）

- `searchHistMode bool` —— 历史列表 overlay 是否展开
- `searchHistCursor int` —— 列表选中索引，`0` = 最新一条（列表倒序，最新在上）
- 退役现有 ctrl+r 循环指针 `searchHistIdx`（删除，改用 `searchHistCursor`）

历史数组 `searchHistory` 为 append 顺序（旧→新），最新在 `searchHistory[len-1]`。列表显示倒序：第 `i` 行对应 `searchHistory[len-1-i]`。

### 按键（仅在 searchMode 且 searchTab==0 生效）

**列表关闭时：**

| 键 | 行为 |
|---|---|
| `ctrl+r` | 有历史 → `searchHistMode=true`，`searchHistCursor=0`；无历史 → 搜索框 placeholder 提示"暂无搜索历史" |

**列表展开时（劫持按键）：**

| 键 | 行为 |
|---|---|
| `j` / `↓` | `searchHistCursor++`，夹紧到 `len-1` |
| `k` / `↑` | `searchHistCursor--`，夹紧到 `0` |
| `Enter` | `searchInput = searchHistory[len-1-searchHistCursor]`，`searchCursor=末尾`，`searchHistMode=false`，`recomputeView()`（实时过滤）。**留在搜索弹窗**，用户可继续改 |
| `Esc` | `searchHistMode=false`，搜索框内容不变 |
| `ctrl+r` | 无操作（避免误关） |
| 其他字符键 | `searchHistMode=false`（关列表），该字符按正常逻辑进入搜索框（续输流畅） |

### 渲染（`buildSearchPopup` → `renderSearchSection`，复用 starFields 模式）

`searchHistMode` 为 true 时，在搜索框上方渲染历史列表，搜索框与提示行保留：

```
┌ 搜索 │ 高亮 │ 隐藏 ┐
  搜索历史
  > ERROR timeout           ← 选中项，SelectedStyle 高亮
    JDBC leak
    level:ERROR AND WARN
    ...
 输入搜索词，支持 field:value AND/OR█     ← 搜索框保留，选中后可改
 j/k选择 Enter填入 Esc取消
```

- 列表倒序，最多显示 8 行；`searchHistCursor` 超出可视区则滚动（保持选中项可见）
- 选中项前缀 `SelectedStyle.Render(" >")`，未选中 `"  "`，与字段补全列表完全一致
- 列表展开时，底部提示行替换为 `j/k选择 Enter填入 Esc取消`

### 边界

- 空历史：ctrl+r 不进入列表模式，搜索框 placeholder 提示"暂无搜索历史"
- searchTab≠0（高亮/隐藏 tab）：不触发历史（现有 ctrl+r 本就限 searchTab==0，保持）
- 历史上限沿用 20 条（`addSearchHistory` 现有逻辑不变）

## 测试（TDD，先写失败测试）

1. `TestSearchHistoryPopupOpens`：有历史时 ctrl+r → `searchHistMode==true`，`searchHistCursor==0`
2. `TestSearchHistoryPopupNavigate`：列表内 j/↓、k/↑ 移动 cursor 并夹紧
3. `TestSearchHistoryPopupEnterFills`：Enter → `searchInput` 等于选中词、`searchHistMode==false`、`filteredView` 已按该词过滤
4. `TestSearchHistoryPopupEscNoFill`：Esc → `searchHistMode==false`，`searchInput` 不变
5. `TestSearchHistoryPopupEmpty`：空历史 ctrl+r → `searchHistMode` 仍为 false
6. `TestSearchHistoryPopupCharCloses`：列表内按字符 → 关列表且字符进入 `searchInput`

## 影响文件

- `internal/tui/app.go` —— 新字段、ctrl+r 重写、列表按键分支（`handleSearchKeys`）
- `internal/tui/searchpopup.go` —— `renderSearchSection` 增加历史列表渲染
- `internal/tui/app_test.go`（或新测试文件）—— 上述测试
