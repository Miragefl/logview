package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// 本文件集中承载搜索/高亮/隐藏三个同构弹窗的状态逻辑：
// 输入编辑、历史、字段建议、确认与匹配跳转。app.go 只保留按键分发。

func (a *App) updateSearchStats() {
	if a.searchInput == "" {
		a.searchMatchCount = 0
		a.searchMatchIdx = 0
		return
	}
	q := a.currentQuery()
	count := 0
	idx := 0
	for i, line := range a.filteredView {
		if q.MatchLine(line) {
			count++
			if i <= a.cursor {
				idx = count
			}
		}
	}
	a.searchMatchCount = count
	a.searchMatchIdx = idx
}

func (a *App) addSearchHistory(query string) {
	if query == "" {
		return
	}
	// deduplicate
	for i, h := range a.searchHistory {
		if h == query {
			a.searchHistory = append(a.searchHistory[:i], a.searchHistory[i+1:]...)
			break
		}
	}
	a.searchHistory = append(a.searchHistory, query)
	if len(a.searchHistory) > 20 {
		a.searchHistory = a.searchHistory[len(a.searchHistory)-20:]
	}
}

func (a *App) matchHides(line *model.ParsedLine) bool {
	text := line.Raw.Text
	for _, kw := range a.hides {
		if containsIgnoreCase(text, kw) {
			return true
		}
	}
	return false
}

func (a *App) currentQuery() SearchQuery {
	if isPartialQuery(a.searchInput) {
		return parseSearchQuery(strippedQuery(a.searchInput)) // 中间态：去掉末尾未完成操作符，用剩余搜索
	}
	if a.cachedQuery.Raw != a.searchInput {
		a.cachedQuery = parseSearchQuery(a.searchInput)
	}
	return a.cachedQuery
}

func (a *App) jumpSearchMatch(dir int) {
	if len(a.filteredView) == 0 {
		return
	}
	var matches []int
	if a.searchInput != "" {
		q := a.currentQuery()
		if q.IsEmpty() {
			return
		}
		for i, line := range a.filteredView {
			if q.MatchLine(line) {
				matches = append(matches, i)
			}
		}
	} else if len(a.highlights) > 0 {
		for i, line := range a.filteredView {
			msg := line.Get(model.FieldMessage)
			for _, kw := range a.highlights {
				if strings.Contains(msg, kw) {
					matches = append(matches, i)
					break
				}
			}
		}
	}
	if len(matches) == 0 {
		return
	}
	cur := a.cursor
	idx := sort.Search(len(matches), func(i int) bool { return matches[i] >= cur })
	if dir > 0 {
		next := idx + 1
		if next >= len(matches) {
			next = 0
		}
		a.cursor = matches[next]
	} else {
		prev := idx - 1
		if prev < 0 {
			prev = len(matches) - 1
		}
		a.cursor = matches[prev]
	}
	a.autoscroll = false
	a.updateSearchStats()
}

func (a *App) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.searchHistMode {
		return a.handleSearchHistKeys(msg)
	}
	input := a.activeSearchInput()
	switch msg.Type {
	case tea.KeyEscape:
		a.closeSearchPopup()
	case tea.KeyEnter:
		a.confirmSearchTab()
	case tea.KeyTab:
		a.searchTab = (a.searchTab + 1) % 3
		a.activeSearchInput().clamp()
	case tea.KeyShiftTab:
		a.searchTab = (a.searchTab + 2) % 3
		a.activeSearchInput().clamp()
	default:
		if _, changed := input.handleEditKeys(msg); changed && a.searchTab == 0 {
			a.recomputeView()
		}
		switch msg.String() {
		case "ctrl+r":
			// 打开搜索历史列表（替换旧的循环切换）
			if a.searchTab == 0 && len(a.searchHistory) > 0 {
				a.searchHistMode = true
				a.searchHistCursor = 0
			}
		case "ctrl+j":
			if a.searchTab == 0 && len(a.starFields) > 0 {
				a.starCursor = (a.starCursor + 1) % len(a.starFields)
			}
		case "ctrl+k":
			if a.searchTab == 0 && len(a.starFields) > 0 {
				a.starCursor = (a.starCursor - 1 + len(a.starFields)) % len(a.starFields)
			}
		}
	}
	return a, nil
}

// handleSearchHistKeys 处理历史列表展开时的按键（导航/选中/关闭/续输）。
func (a *App) handleSearchHistKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(a.searchHistory)
	if n == 0 {
		// 无历史时不应进入列表（ctrl+r 打开有 len>0 守门），防御性关闭
		a.searchHistMode = false
		return a, nil
	}
	switch msg.Type {
	case tea.KeyEscape:
		a.searchHistMode = false
	case tea.KeyEnter:
		// 倒序：cursor=0 对应最新（searchHistory 末尾）
		a.applySearchHistory(a.searchHistory[n-1-a.searchHistCursor])
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
			// 其他字符：关闭列表，按正常逻辑进入搜索框
			a.searchHistMode = false
			return a.handleSearchKeys(msg)
		}
	case tea.KeyCtrlR:
		// 列表内再按 ctrl+r：无操作（避免误关）
	default:
		// 其他未识别键（Tab 等）：关闭列表，回正常搜索处理
		a.searchHistMode = false
		return a.handleSearchKeys(msg)
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

// activeSearchInput returns the input editor for the current search tab.
func (a *App) activeSearchInput() inputRef {
	switch a.searchTab {
	case 1:
		return inputRef{&a.highlightInput, &a.highlightCursor}
	case 2:
		return inputRef{&a.hideInput, &a.hideCursor}
	default:
		return inputRef{&a.searchInput, &a.searchCursor}
	}
}

func (a *App) closeSearchPopup() {
	a.searchMode = false
	a.starFields = nil
	a.starCursor = 0
}

// splitKeywords parses a comma-separated keyword string into a clean slice.
func splitKeywords(kw string) []string {
	parts := strings.Split(kw, ",")
	var clean []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			clean = append(clean, p)
		}
	}
	return clean
}

func (a *App) confirmHighlights() {
	kw := strings.TrimSpace(a.highlightInput)
	if kw != "" {
		a.highlights = splitKeywords(kw)
	} else {
		a.highlights = nil
	}
}

func (a *App) confirmHides() {
	kw := strings.TrimSpace(a.hideInput)
	if kw != "" {
		a.hides = splitKeywords(kw)
	} else {
		a.hides = nil
	}
	a.recomputeView()
}

// confirmSearchTab handles Enter based on the current search tab.
func (a *App) confirmSearchTab() {
	switch a.searchTab {
	case 1:
		a.confirmHighlights()
		a.closeSearchPopup()
	case 2:
		a.confirmHides()
		a.closeSearchPopup()
	default:
		if len(a.starFields) > 0 && a.starCursor < len(a.starFields) {
			sf := a.starFields[a.starCursor]
			if sf.Name != "" {
				term := sf.Name + ":" + sf.Value
				insert := term
				runes := []rune(a.searchInput)
				pos := a.searchCursor
				if a.searchInput != "" && pos > 0 && runes[pos-1] != ' ' {
					insert = " " + insert
				}
				if a.searchInput != "" && pos < len(runes) && runes[pos] != ' ' {
					insert = insert + " "
				}
				inputRef{&a.searchInput, &a.searchCursor}.insert(insert)
				return
			}
		}
		a.addSearchHistory(a.searchInput)
		a.recomputeView()
		a.closeSearchPopup()
	}
}

func (a *App) handleHighlightKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		a.highlightMode = false
	case tea.KeyEnter:
		a.confirmHighlights()
		a.highlightMode = false
	default:
		inputRef{&a.highlightInput, &a.highlightCursor}.handleEditKeys(msg)
	}
	return a, nil
}

func (a *App) handleHideKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		a.hideMode = false
	case tea.KeyEnter:
		a.confirmHides()
		a.hideMode = false
	default:
		inputRef{&a.hideInput, &a.hideCursor}.handleEditKeys(msg)
	}
	return a, nil
}

func (a *App) populateSearchFields() {
	if a.cursor < 0 || a.cursor >= len(a.filteredView) {
		a.starFields = nil
		return
	}
	line := a.filteredView[a.cursor]
	var fields []starField
	fields = append(fields, starField{Name: "", Value: ""})
	for _, f := range model.AllFields {
		val := line.Get(f)
		if val == "" {
			continue
		}
		if f == model.FieldMessage {
			for _, w := range strings.Fields(val) {
				if len(w) > 1 {
					fields = append(fields, starField{Name: string(f), Value: w})
				}
			}
			continue
		}
		fields = append(fields, starField{Name: string(f), Value: val})
	}
	a.starFields = fields
}
