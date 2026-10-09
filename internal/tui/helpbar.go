package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/justfun/logview/internal/model"
	"github.com/mattn/go-runewidth"
)

type helpItem struct {
	key  string
	desc string
}

// shortcutItems 返回当前模式的快捷键提示（学习用，可隐藏）。
func (a *App) shortcutItems() []helpItem {
	switch {
	case a.detailMode:
		return []helpItem{
			{"C-j/C-k", "上下换行"},
			{"y", "复制该行"},
			{"Esc/d", "关闭"},
		}
	case a.sourcePickerMode:
		return []helpItem{
			{"Enter", "确认"},
			{"Space", "勾选"},
			{"Backspace", "返回"},
			{"Tab", "切分类"},
			{"C-j/C-k", "移动"},
			{"Esc", "取消"},
		}
	case a.sshPwMode:
		return []helpItem{
			{"Enter", "重连"},
			{"Esc", "取消"},
		}
	case a.helpMode:
		return []helpItem{
			{"Esc/Enter", "关闭"},
		}
	case a.statsPanel:
		return []helpItem{
			{"S/Esc", "关闭"},
		}
	case a.searchMode:
		return []helpItem{
			{"Enter", "确认"},
			{"Tab", "切换分区"},
			{"C-t", "时间片"},
			{"C-j/C-k", "切换字段"},
			{"C-u", "清空输入"},
			{"C-r", "历史"},
			{"Esc", "取消"},
		}
	case a.visualMode:
		return []helpItem{
			{"j/k", "上下移动"},
			{"g/G", "顶/底"},
			{"y", "复制选中"},
			{"Esc", "退出选择"},
		}
	case a.panelFocus:
		return []helpItem{
			{"C-k/↑ C-j/↓", "移动"},
			{"Space/Enter", "切换显示"},
			{"Esc/q", "关闭"},
		}
	case a.exportMode:
		return []helpItem{
			{"C-k/↑ C-j/↓", "移动"},
			{"←/h →/l", "切换选项"},
			{"Enter", "导出"},
			{"Esc/q", "关闭"},
		}
	default:
		return []helpItem{
			{"/", "搜索"},
			{"o", "切换源"},
			{"F", "字段"},
			{"E/W/I/D/A", "级别"},
			{"s", "导出"},
			{"w", "换行"},
			{"d", "行详情"},
			{"e", "展开"},
			{"?", "帮助"},
			{"\\", "收起提示"},
		}
	}
}

// statusItems 返回常驻状态徽章（运行时反馈，不可隐藏）。
func (a *App) statusItems() []helpItem {
	var items []helpItem
	if a.searchMode && a.searchTab == 0 && a.searchInput != "" {
		items = append(items, helpItem{"", fmt.Sprintf("[匹配: %d条]", len(a.filteredView))})
	}
	if a.levelFilter != "" {
		items = append(items, helpItem{"", LevelStyle(a.levelFilter).Render(fmt.Sprintf("[过滤: %s]", a.levelFilter))})
	}
	if len(a.hides) > 0 {
		items = append(items, helpItem{"", HideMarkStyle.Render(fmt.Sprintf("[隐藏:%d词]", len(a.hides)))})
	}
	if !a.searchMode && a.searchInput != "" {
		items = append(items, helpItem{"", fmt.Sprintf("[搜索: %s]", runewidth.Truncate(a.searchInput, 20, "…"))})
	}
	if a.yankMsg != "" {
		items = append(items, helpItem{"", NewLogStyle.Render(a.yankMsg)})
	}
	if a.ctxN > 0 {
		items = append(items, helpItem{"", NewLogStyle.Render(fmt.Sprintf("混入 ×%d(Esc/-- 关闭)", a.ctxN))})
	}
	if a.memBytes > 0 {
		items = append(items, helpItem{"", HelpStyle.Render("[内存: " + humanBytes(a.memBytes) + "]")})
	}
	if lb := a.loadBadge(); lb != "" {
		items = append(items, helpItem{"", HelpStyle.Render(lb)})
	}
	return items
}

// loadBadge 加载时长徽章:流结束(EOF)后定格总时长;未结束时按最近一行间隔区分——
// 3s 内有新行为"加载中"(活跃接收),否则为"实时"(follow 类源存量读毕后的静默态)。
func (a *App) loadBadge() string {
	if a.firstLineAt.IsZero() {
		return "" // 首行未到,无从计起
	}
	if a.loadDur > 0 {
		return "[加载: " + fmtDur(a.loadDur) + "]"
	}
	span := a.lastLineAt.Sub(a.firstLineAt)
	if span <= 0 {
		return ""
	}
	if time.Since(a.lastLineAt) < 3*time.Second {
		return "[加载中: " + fmtDur(span) + "]"
	}
	return "[实时: " + fmtDur(span) + "]"
}

// fmtDur 时长展示:1 分钟内保留 1 位小数,超时切换 m/s 形式。
func fmtDur(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

// humanBytes 字节数自适应展示(MB/GB)。
func humanBytes(b uint64) string {
	const mb = 1 << 20
	if b < 1<<30 {
		return fmt.Sprintf("%dMB", b/mb)
	}
	return fmt.Sprintf("%.1fGB", float64(b)/(1<<30))
}

// renderFooter 渲染底部：状态栏常驻 + 快捷键栏（showKeyHints 控制显隐）。
func (a *App) renderFooter() string {
	status := a.joinHelpItems(a.statusItems())
	if !a.showKeyHints {
		if status == "" {
			return HelpStyle.Render(" \\显示快捷键")
		}
		return status + "  " + HelpStyle.Render("\\显示快捷键")
	}
	hints := a.joinHelpItems(a.shortcutItems())
	if status == "" {
		return hints
	}
	return status + "\n" + hints
}

func (a *App) joinHelpItems(items []helpItem) string {
	var parts []string
	for _, it := range items {
		if it.key == "" {
			parts = append(parts, it.desc)
		} else {
			parts = append(parts, fmt.Sprintf("%s %s", KeyCapStyle.Render(it.key), HelpStyle.Render(it.desc)))
		}
	}
	return strings.Join(parts, "  ")
}

// footerHeight 返回底部占用行数（状态栏 + 可选快捷键栏）。
func (a *App) footerHeight() int {
	h := 1 // status line (may be empty but reserve the line)
	if a.showKeyHints {
		h++
	}
	return h
}

func displayWidth(s string) int {
	return runewidth.StringWidth(model.StripANSI(s))
}
