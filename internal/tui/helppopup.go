package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpSection struct {
	title string
	items []helpItem
}

func helpSections() []helpSection {
	return []helpSection{
		{"导航", []helpItem{
			{"↑/k", "上移"},
			{"↓/j", "下移"},
			{"g", "顶部"},
			{"G", "底部"},
			{"C-u/C-d", "上/下半页"},
			{"C-b/C-f", "上/下翻页"},
			{"H/M/L", "屏顶/屏中/屏底"},
			{"zt/zz/zb", "当前行置顶/居中/置底"},
		}},
		{"搜索", []helpItem{
			{"f或/", "打开搜索"},
			{"n/N", "下一个/上一个匹配"},
			{"C-r", "搜索历史"},
			{"Tab/S-Tab", "切换分区/字段"},
			{"C-j/C-k", "上下字段"},
			{"Enter", "确认"},
			{"Esc", "取消"},
		}},
		{"选择与复制", []helpItem{
			{"v", "可视化选择"},
			{"y", "复制"},
		}},
		{"日志级别", []helpItem{
			{"E", "仅ERROR"},
			{"W", "ERROR+WARN"},
			{"I", "去掉DEBUG"},
			{"D", "全部级别"},
			{"A", "取消过滤"},
		}},
		{"标记与工具", []helpItem{
			{"m", "标记/取消标记"},
			{"'", "跳转标记"},
			{"#", "切换行号"},
			{"S", "统计面板"},
		}},
		{"其他", []helpItem{
			{"F", "字段设置"},
			{"s", "导出日志"},
			{"e", "展开/折叠堆栈"},
			{"o", "切换日志源(k8s/本地/SSH)"},
			{"h", "高亮关键词"},
			{"x", "隐藏关键词"},
			{"w", "换行"},
			{"\\", "收起/展开提示栏"},
			{"?", "帮助"},
			{"q", "打开源选择器/退出"}, {"C-c", "退出"},
		}},
	}
}

// sectionHeight 一个分组渲染后的行数（标题 + 条目 + 空行）。
func sectionHeight(sec helpSection) int {
	return 1 + len(sec.items) + 1
}

func renderHelpSection(sec helpSection) string {
	var b strings.Builder
	b.WriteString(DetailLabelStyle.Render(sec.title) + "\n")
	for _, it := range sec.items {
		b.WriteString(fmt.Sprintf("  %s %s\n", HelpKeyStyle.Render(it.key), HelpStyle.Render(it.desc)))
	}
	b.WriteString("\n")
	return b.String()
}

// buildHelpPopup 宽终端（≥100 列）双栏减半高度；高度不足时按分组边界截断并提示。
func (a *App) buildHelpPopup(vl int) []string {
	sections := helpSections()

	var columns []string
	if a.width >= 100 {
		// 按累计高度均分到两栏，保持分组完整
		total := 0
		for _, sec := range sections {
			total += sectionHeight(sec)
		}
		left, right := strings.Builder{}, strings.Builder{}
		acc := 0
		for _, sec := range sections {
			if acc < total/2 {
				left.WriteString(renderHelpSection(sec))
				acc += sectionHeight(sec)
			} else {
				right.WriteString(renderHelpSection(sec))
			}
		}
		columns = []string{left.String(), right.String()}
	} else {
		var single strings.Builder
		for _, sec := range sections {
			single.WriteString(renderHelpSection(sec))
		}
		columns = []string{single.String()}
	}

	maxRows := vl - 4 // 边框(2) + hint(1) + 余量(1)
	if maxRows < 5 {
		maxRows = 5
	}

	// 高度不足时按分组边界截断（单栏模式）
	if len(columns) == 1 {
		var b strings.Builder
		acc := 0
		for _, sec := range sections {
			h := sectionHeight(sec)
			if acc+h > maxRows {
				break
			}
			b.WriteString(renderHelpSection(sec))
			acc += h
		}
		if acc == 0 {
			b.WriteString(renderHelpSection(sections[0]))
			acc = sectionHeight(sections[0])
		}
		if acc+2 > maxRows {
			b.WriteString(PopupTabStyle.Render(" 放大终端查看更多") + "\n")
		}
		columns[0] = b.String()
	}

	content := columns[0]
	if len(columns) == 2 {
		c1 := strings.TrimRight(columns[0], "\n")
		c2 := strings.TrimRight(columns[1], "\n")
		content = lipgloss.JoinHorizontal(lipgloss.Top, c1, "  ", c2) + "\n"
	}
	content += PopupTabStyle.Render(" Esc/回车 关闭")

	boxW := min(52, a.width-4)
	if len(columns) == 2 {
		boxW = min(104, a.width-4)
	}
	box := PopupBoxStyle.Width(boxW).Render(content)
	return a.overlayToVL(box, vl)
}
