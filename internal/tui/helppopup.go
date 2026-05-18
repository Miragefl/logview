package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) buildHelpPopup(vl int) []string {
	lines := make([]string, vl)

	sections := []struct {
		title string
		items []helpItem
	}{
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
			{"Tab/S-Tab", "切换字段"},
			{"C-j/C-k", "上下字段"},
			{"Enter", "确认"},
			{"Esc", "取消搜索"},
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
		{"其他", []helpItem{
			{"F", "字段设置"},
			{"s", "导出日志"},
			{"e", "展开/折叠堆栈"},
			{"h", "高亮关键词"},
			{"x", "隐藏关键词"},
			{"?", "帮助"},
			{"q/C-c", "退出"},
		}},
	}

	var content strings.Builder
	for _, sec := range sections {
		content.WriteString(DetailLabelStyle.Render(sec.title) + "\n")
		for _, it := range sec.items {
			content.WriteString(fmt.Sprintf("  %s %s\n", HelpKeyStyle.Render(it.key), HelpStyle.Render(it.desc)))
		}
		content.WriteString("\n")
	}
	hint := PopupTabStyle.Render(" Esc/回车 关闭")
	content.WriteString(hint)

	boxW := min(52, a.width-4)
	box := PopupBoxStyle.Width(boxW).Render(content.String())

	overlay := lipgloss.NewStyle().Width(a.width).Height(vl).
		Render(lipgloss.Place(a.width, vl,
			lipgloss.Center, lipgloss.Center,
			box))

	parts := strings.Split(overlay, "\n")
	for i := 0; i < vl; i++ {
		if i < len(parts) {
			lines[i] = parts[i]
		}
	}
	return lines
}
