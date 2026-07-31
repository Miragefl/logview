# 搜索历史列表（ctrl+r popup）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搜索模式下 ctrl+r 弹出历史列表，j/k 或 ↑/↓ 选择，Enter 填入搜索框（留弹窗可改），替换现有循环切换。

**Architecture:** 在 App 加 `searchHistMode`/`searchHistCursor` 状态；`handleSearchKeys` 开头劫持进入列表按键处理 `handleSearchHistKeys`；`renderSearchSection` 在 searchHistMode 时渲染倒序列表（复用字段补全 starFields 的 `>`+`SelectedStyle` 模式）；退役旧循环指针 `searchHistIdx`。

**Tech Stack:** Go 1.21+、Bubbletea（charmbracelet/bubbletea）、lipgloss。测试用标准 `testing`。

## Global Constraints

- 历史来自 `addSearchHistory`（每次 Enter 确认搜索记录，去重，最多 20 条，只存内存），本次**不改**其去重/上限逻辑。
- 历史列表仅在 `searchMode && searchTab==0`（搜索 tab）触发，高亮/隐藏 tab 不触发。
- 列表倒序：最新在上，`searchHistCursor=0` 对应 `searchHistory[len-1]`。
- 复用现有样式：`SelectedStyle`（选中）、`HelpKeyStyle`（标题）、`DetailDimStyle`（词条）、`PopupTabStyle`（提示行）—— 全部已在 `style.go`/`searchpopup.go` 中定义并使用，**不要新增样式**。
- 遵循 KISS/YAGNI/DRY，不改无关代码。
- 代码注释用中文，与现有 `internal/tui/` 注释风格一致。

---

## File Structure

- `internal/tui/app.go` —— App struct 新增 2 字段、退役 `searchHistIdx`、`handleSearchKeys` 劫持入口、新增 `handleSearchHistKeys`/`applySearchHistory`、重写 ctrl+r 分支、`addSearchHistory` 去掉 `searchHistIdx` 引用。
- `internal/tui/searchpopup.go` —— `renderSearchSection` 加 searchHistMode 分支、新增 `renderSearchHistoryList`。
- `internal/tui/searchhistory_test.go` —— 新建，全部测试。

---

## Task 1: 状态字段 + ctrl+r 打开列表 + 列表按键骨架

退役旧循环指针，加新状态，ctrl+r 改为"打开列表"，并加入最小按键骨架（Esc 关闭、其他暂不处理）。此任务后列表能开能关，但还不能导航/选中（后续任务加）。

**Files:**
- Modify: `internal/tui/app.go:96-97`（struct 字段）
- Modify: `internal/tui/app.go:511`（addSearchHistory）
- Modify: `internal/tui/app.go:913`（handleSearchKeys 入口）
- Modify: `internal/tui/app.go:984-993`（ctrl+r 分支）
- Create: `internal/tui/searchhistory_test.go`

**Interfaces:**
- Produces: `App.searchHistMode bool`、`App.searchHistCursor int`、`func (a *App) handleSearchHistKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd)`

- [ ] **Step 1: 写失败测试**

创建 `internal/tui/searchhistory_test.go`：

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeKey 构造 default-case 字符串键（如 "ctrl+r"）的 KeyMsg。
// bubbletea 里 ctrl+r 是 KeyCtrlR，其 String() 返回 "ctrl+r"，匹配 handleSearchKeys 的 default 分支。
func fakeKey(s string) tea.KeyMsg {
	switch s {
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// 进搜索 + 确认一次搜索产生历史后，ctrl+r 应打开历史列表。
func TestSearchHistPopupOpens(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 进搜索
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})                    // 确认 → 历史记 "ERROR" + 关弹窗
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 再进搜索
	app.Update(fakeKey("ctrl+r"))                                  // 打开历史列表
	if !app.searchHistMode {
		t.Fatalf("ctrl+r 后 searchHistMode 应为 true，实际 false")
	}
	if app.searchHistCursor != 0 {
		t.Fatalf("searchHistCursor 应为 0（最新），实际 %d", app.searchHistCursor)
	}
}

// 空历史时 ctrl+r 不打开列表。
func TestSearchHistPopupEmpty(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 进搜索，无历史
	app.Update(fakeKey("ctrl+r"))
	if app.searchHistMode {
		t.Fatalf("空历史时 searchHistMode 应保持 false")
	}
}

// 列表打开后 Esc 关闭。
func TestSearchHistPopupEscCloses(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 记历史
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app.Update(fakeKey("ctrl+r")) // 打开
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.searchHistMode {
		t.Fatalf("Esc 后 searchHistMode 应为 false")
	}
}
```

- [ ] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/tui/ -run TestSearchHist -v`
Expected: 编译失败（`searchHistMode` undefined、`fakeKey` 实现不对）

- [ ] **Step 3: 加字段 + 重写 ctrl+r + 骨架**

修改 `internal/tui/app.go:96-97`，把

```go
	searchHistory []string
	searchHistIdx int
```

改为

```go
	searchHistory   []string
	searchHistMode  bool // ctrl+r 历史列表 overlay 是否展开
	searchHistCursor int // 列表选中索引，0=最新（列表倒序）
```

修改 `internal/tui/app.go:511`（addSearchHistory 末尾），删除

```go
	a.searchHistIdx = 0
```

（这是退役 searchHistIdx 的最后一处引用。删除后该函数末尾只剩 `}`，确认无残留。）

修改 `internal/tui/app.go:913` 的 `handleSearchKeys`，在函数体第一行（`input, cursor := a.activeSearchInput()` 之前）插入劫持：

```go
func (a *App) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.searchHistMode {
		return a.handleSearchHistKeys(msg)
	}
	input, cursor := a.activeSearchInput()
	switch msg.Type {
```

修改 `internal/tui/app.go:984-993` 的 ctrl+r 分支，把整个 `case "ctrl+r":` 体替换为：

```go
		case "ctrl+r":
			// 打开搜索历史列表（替换旧的循环切换）
			if a.searchTab == 0 && len(a.searchHistory) > 0 {
				a.searchHistMode = true
				a.searchHistCursor = 0
			}
```

在 `handleSearchKeys` 函数之后（原 `activeSearchInput` 之前，约 `internal/tui/app.go:1003` 之后）新增两个方法：

```go
// handleSearchHistKeys 处理历史列表展开时的按键（导航/选中/关闭）。
func (a *App) handleSearchHistKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		a.searchHistMode = false
	default:
		// 导航(j/k/↑↓)、选中(Enter)、字符续输 在后续任务补全
	}
	return a, nil
}

// applySearchHistory 把选中的历史词填入搜索框、关闭列表、重新过滤。
func (a *App) applySearchHistory(q string) {
	a.searchHistMode = false
	a.searchInput = q
	a.searchCursor = len([]rune(q))
	a.recomputeView()
}
```

- [ ] **Step 4: 跑测试，确认通过**

Run: `go test ./internal/tui/ -run TestSearchHist -v`
Expected: 3 个测试 PASS

- [ ] **Step 5: 全包回归**

Run: `go test ./...`
Expected: 全 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/searchhistory_test.go
git commit -m "feat: ctrl+r opens search history popup (replaces cycle)"
```

---

## Task 2: 渲染历史列表

让 searchHistMode 时搜索弹窗渲染出倒序历史列表（选中标记 + 滚动），搜索框与提示行保留。此任务后列表可见。

**Files:**
- Modify: `internal/tui/searchpopup.go:38-62`（renderSearchSection）
- Test: `internal/tui/searchhistory_test.go`

**Interfaces:**
- Produces: `func (a *App) renderSearchHistoryList(content *strings.Builder)`
- Consumes: `App.searchHistMode`、`App.searchHistCursor`、`App.searchHistory`（Task 1）

- [ ] **Step 1: 写失败测试**

在 `internal/tui/searchhistory_test.go` 追加：

```go
import "strings"

// 列表展开时，搜索弹窗应渲染出历史词条与选中标记、改提示行。
func TestSearchHistPopupRender(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 历史: ["ERROR"]
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "WARN" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 历史: ["ERROR","WARN"]
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app.Update(fakeKey("ctrl+r")) // 打开列表

	view := app.View()
	if !strings.Contains(view, "搜索历史") {
		t.Fatalf("应渲染“搜索历史”标题，view=\n%s", view)
	}
	// 倒序：WARN 在 ERROR 之上（最新在上）
	warnPos := strings.Index(view, "WARN")
	errPos := strings.Index(view, "ERROR")
	if warnPos < 0 || errPos < 0 || warnPos > errPos {
		t.Fatalf("最新历史 WARN 应在 ERROR 之上，warnPos=%d errPos=%d", warnPos, errPos)
	}
	if !strings.Contains(view, "j/k选择") {
		t.Fatalf("列表展开时提示行应为 j/k选择，view=\n%s", view)
	}
}
```

注意：测试文件顶部已有 `import ( "testing"; tea ... )`，把 `"strings"` 合并进去。

- [ ] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/tui/ -run TestSearchHistPopupRender -v`
Expected: FAIL（"搜索历史"未渲染）

- [ ] **Step 3: 实现渲染**

修改 `internal/tui/searchpopup.go:38` 的 `renderSearchSection`，整个函数替换为：

```go
func (a *App) renderSearchSection(content *strings.Builder) {
	if a.searchHistMode {
		a.renderSearchHistoryList(content)
		content.WriteString(a.inputLine(a.searchInput, a.searchCursor, "输入搜索词，支持 field:value AND/OR") + "\n")
		content.WriteString("\n" + PopupTabStyle.Render(" Tab切分区 j/k选择 Enter填入 Esc取消"))
		return
	}
	if len(a.starFields) > 0 {
		nRows := len(a.starFields)
		if nRows > 6 {
			nRows = 6
		}
		for i := 0; i < nRows; i++ {
			sf := a.starFields[i]
			prefix := "  "
			if i == a.starCursor {
				prefix = SelectedStyle.Render(" >")
			}
			if sf.Name == "" {
				content.WriteString(prefix + " " + HelpKeyStyle.Render("确认搜索") + "\n")
			} else {
				name := DetailLabelStyle.Render(sf.Name + ":")
				val := DetailValueStyle.Render(sf.Value)
				content.WriteString(prefix + " " + name + " " + val + "\n")
			}
		}
		content.WriteString("\n")
	}
	content.WriteString(a.inputLine(a.searchInput, a.searchCursor, "输入搜索词，支持 field:value AND/OR") + "\n")
	content.WriteString("\n" + PopupTabStyle.Render(" Tab切分区 C-j/k字段 Enter确认 C-r历史 Esc取消"))
}
```

在 `renderSearchSection` 之后新增 `renderSearchHistoryList`：

```go
// renderSearchHistoryList 渲染倒序搜索历史列表（最新在上），最多 8 行，光标跟随滚动。
func (a *App) renderSearchHistoryList(content *strings.Builder) {
	n := len(a.searchHistory)
	if n == 0 {
		return
	}
	content.WriteString(HelpKeyStyle.Render("搜索历史") + "\n")
	maxRows := 8
	if n < maxRows {
		maxRows = n
	}
	// 滚动窗口起点：保证 searchHistCursor 可见
	start := 0
	if a.searchHistCursor > maxRows-1 {
		start = a.searchHistCursor - maxRows + 1
	}
	for i := 0; i < maxRows; i++ {
		row := start + i // 列表第 i 个可见行
		if row >= n {
			break
		}
		histIdx := n - 1 - row // 倒序映射到 searchHistory
		prefix := "  "
		if row == a.searchHistCursor {
			prefix = SelectedStyle.Render(" >")
		}
		content.WriteString(prefix + " " + DetailDimStyle.Render(a.searchHistory[histIdx]) + "\n")
	}
	content.WriteString("\n")
}
```

- [ ] **Step 4: 跑测试，确认通过**

Run: `go test ./internal/tui/ -run TestSearchHistPopupRender -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/searchpopup.go internal/tui/searchhistory_test.go
git commit -m "feat: render search history list in popup (reverse, scrollable)"
```

---

## Task 3: 列表内导航（j/k + ↑/↓）

展开时 j/↓ 往下选（更旧），k/↑ 往上选（更新），夹紧到边界。

**Files:**
- Modify: `internal/tui/app.go`（handleSearchHistKeys，Task 1 新增的骨架）
- Test: `internal/tui/searchhistory_test.go`

**Interfaces:**
- Consumes: `App.searchHistMode`、`App.searchHistCursor`、`App.searchHistory`

- [ ] **Step 1: 写失败测试**

追加到 `internal/tui/searchhistory_test.go`：

```go
// 历史 ["ERROR","WARN","INFO"]，倒序显示 WARN/INFO/ERROR... 展开后 cursor=0（最新=最后append的）。
// j/↓ 往下（更旧），k/↑ 往上（更新），夹紧。
func TestSearchHistPopupNavigate(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, q := range []string{"ERROR", "WARN", "INFO"} {
		for _, r := range q {
			app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		app.Update(tea.KeyMsg{Type: tea.KeyEnter})
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	}
	app.Update(fakeKey("ctrl+r")) // 打开，cursor=0
	n := len(app.searchHistory)   // 3

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // 往下
	if app.searchHistCursor != 1 {
		t.Fatalf("j 后 cursor 应=1，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // ↓ 往下
	if app.searchHistCursor != 2 {
		t.Fatalf("↓ 后 cursor 应=2，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // 再 ↓ 夹紧
	if app.searchHistCursor != 2 {
		t.Fatalf("到底应夹紧 2，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyUp}) // ↑ 往上
	if app.searchHistCursor != 1 {
		t.Fatalf("↑ 后 cursor 应=1，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}) // k 往上
	if app.searchHistCursor != 0 {
		t.Fatalf("k 后 cursor 应=0，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}) // 到顶夹紧
	if app.searchHistCursor != 0 {
		t.Fatalf("到顶应夹紧 0，实际 %d", app.searchHistCursor)
	}
	_ = n
}
```

- [ ] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/tui/ -run TestSearchHistPopupNavigate -v`
Expected: FAIL（j/k/↑↓ 无响应，cursor 不变）

- [ ] **Step 3: 实现导航**

把 `internal/tui/app.go` 中 Task 1 新增的 `handleSearchHistKeys` 替换为：

```go
// handleSearchHistKeys 处理历史列表展开时的按键（导航/选中/关闭/续输）。
func (a *App) handleSearchHistKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(a.searchHistory)
	switch msg.Type {
	case tea.KeyEscape:
		a.searchHistMode = false
	case tea.KeyUp:
		if a.searchHistCursor > 0 {
			a.searchHistCursor--
		}
	case tea.KeyDown:
		if a.searchHistCursor < n-1 {
			a.searchHistCursor++
		}
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "k":
			if a.searchHistCursor > 0 {
				a.searchHistCursor--
			}
		case "j":
			if a.searchHistCursor < n-1 {
				a.searchHistCursor++
			}
		default:
			// 其他字符：关闭列表，按正常逻辑进入搜索框（Task 5 续输）
			a.searchHistMode = false
			return a.handleSearchKeys(msg)
		}
	default:
		// 其他未识别键（Enter/ctrl+r/Tab 等）：关闭列表，回正常搜索处理（Task 4/5 细化）
		a.searchHistMode = false
		return a.handleSearchKeys(msg)
	}
	return a, nil
}
```

> 注：此 default 让 Enter（Task 4）和 ctrl+r（Task 5）在细化前会关闭列表，从而使各自的测试先 FAIL（红灯），符合 TDD。

- [ ] **Step 4: 跑测试，确认通过**

Run: `go test ./internal/tui/ -run TestSearchHistPopupNavigate -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/searchhistory_test.go
git commit -m "feat: navigate search history list with j/k and arrow keys"
```

---

## Task 4: Enter 选中填入

Enter 把选中历史词填入搜索框、关闭列表、实时过滤，留在搜索弹窗。

**Files:**
- Modify: `internal/tui/app.go`（handleSearchHistKeys 加 Enter 分支）
- Test: `internal/tui/searchhistory_test.go`

**Interfaces:**
- Consumes: `func (a *App) applySearchHistory(q string)`（Task 1 已建）

- [ ] **Step 1: 写失败测试**

追加到 `internal/tui/searchhistory_test.go`：

```go
// Enter 选中：填入 searchInput、关闭列表、按该词过滤。
func TestSearchHistPopupEnterFills(t *testing.T) {
	app := newTestApp() // 20 行 "test message"
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "test" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 历史 ["test"]，过滤到全部含 test
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app.Update(fakeKey("ctrl+r")) // 打开，cursor=0 → 选中 "test"
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选中填入

	if app.searchHistMode {
		t.Fatalf("Enter 后应关闭列表")
	}
	if app.searchInput != "test" {
		t.Fatalf("应填入 test，实际 %q", app.searchInput)
	}
	if len(app.filteredView) != 20 {
		t.Fatalf("test 应匹配全部 20 行（实时过滤），实际 %d", len(app.filteredView))
	}
}
```

- [ ] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/tui/ -run TestSearchHistPopupEnterFills -v`
Expected: FAIL（Enter 未填入，searchInput 仍为打开列表前的值）

- [ ] **Step 3: 加 Enter 分支**

在 `handleSearchHistKeys` 的 `switch msg.Type` 里、`case tea.KeyEscape:` 之后加：

```go
	case tea.KeyEnter:
		// 倒序：cursor=0 对应最新（searchHistory 末尾）
		a.applySearchHistory(a.searchHistory[n-1-a.searchHistCursor])
```

（`n` 已在函数体第一行 `n := len(a.searchHistory)` 定义。）

- [ ] **Step 4: 跑测试，确认通过**

Run: `go test ./internal/tui/ -run TestSearchHistPopupEnterFills -v`
Expected: PASS

- [ ] **Step 5: 全包回归**

Run: `go test ./...`
Expected: 全 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/searchhistory_test.go
git commit -m "feat: Enter fills search box from history list"
```

---

## Task 5: 列表内 ctrl+r 无操作 + 字符续传关闭确认

明确列表内 ctrl+r 无操作（避免误关）；验证字符键关列表续输已在 Task 3 的 default 分支实现，补一个测试锁定行为。

**Files:**
- Modify: `internal/tui/app.go`（handleSearchHistKeys default 分支显式处理 ctrl+r）
- Test: `internal/tui/searchhistory_test.go`

- [ ] **Step 1: 写失败测试**

追加到 `internal/tui/searchhistory_test.go`：

```go
// 列表内 ctrl+r 无操作（不关列表）。
func TestSearchHistPopupCtrlRNoop(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app.Update(fakeKey("ctrl+r")) // 打开
	app.Update(fakeKey("ctrl+r")) // 列表内再 ctrl+r
	if !app.searchHistMode {
		t.Fatalf("列表内 ctrl+r 应无操作（保持展开）")
	}
}

// 列表内按其他字符：关列表，字符进入搜索框。
func TestSearchHistPopupCharClosesAndTypes(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app.Update(fakeKey("ctrl+r")) // 打开
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}) // 续输字符
	if app.searchHistMode {
		t.Fatalf("字符键应关闭列表")
	}
	if app.searchInput != "x" {
		t.Fatalf("字符应进入搜索框，实际 %q", app.searchInput)
	}
}
```

- [ ] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/tui/ -run 'TestSearchHistPopupCtrlRNoop|TestSearchHistPopupCharClosesAndTypes' -v`
Expected: `CtrlRNoop` FAIL（ctrl+r 落入 Task 3 的 default 关了列表，mode 变 false）；`CharCloses` PASS（Task 3 字符续输已实现）。

- [ ] **Step 3: 显式处理 ctrl+r 无操作**

在 `handleSearchHistKeys` 的 `switch msg.Type` 里、`default:` **之前**插入一个 case：

```go
	case tea.KeyCtrlR:
		// 列表内再按 ctrl+r：无操作（避免误关）
```

`default:` 分支保持不变（Task 3 的"关列表回正常"继续处理 Tab 等其他键）。

- [ ] **Step 4: 跑测试，确认通过**

Run: `go test ./internal/tui/ -run TestSearchHist -v`
Expected: 全部 8 个测试 PASS

- [ ] **Step 5: 全包回归 + go vet**

Run: `go test ./... && go vet ./internal/tui/`
Expected: 全 PASS，vet 无输出

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/searchhistory_test.go
git commit -m "feat: ctrl+r noop inside history list, explicit key handling"
```

---

## 完工验证（人工）

实现完所有任务后，构建并手动验证：

```bash
go build -o /tmp/logview-test . && /tmp/logview-test file <某个日志文件>
```

在程序里：
1. `/` 进搜索 → 输词 → Enter（产生历史）→ 重复几次造多条历史
2. `/` 再进搜索 → `ctrl+r` → 应弹出历史列表，最新在上，第一条高亮
3. `j`/`↓`、`k`/`↑` 上下选，光标高亮跟随、到底/到顶夹紧
4. `Enter` → 选中词填入搜索框、列表关闭、日志实时过滤，**留在弹窗可改**
5. 再 `ctrl+r` 打开 → 列表内按字母 → 列表关闭、字母进入搜索框
6. 再 `ctrl+r` 打开 → 列表内 `ctrl+r` → 无反应；`Esc` → 关闭列表不填入
7. 无历史时（新会话）`ctrl+r` → 不弹列表
