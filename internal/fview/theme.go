// 主题绑定：logcore.ThemeConfig → 前端 CSS 变量（单源色值，spec 决策表）。
package fview

import (
	"github.com/Miragefl/logview/internal/logcore"
)

// ThemeToCSS 主题色转 CSS 变量 KV（前端 :root 注入；--level-* 为各级别色）。
// 空值回落：TUI 终端主题的 Bg/Fg 为空是合法状态（终端透明），桌面端必须有底色，
// 回落到 fview 默认深色。
func ThemeToCSS(cfg logcore.ThemeConfig) map[string]string {
	bg, fg := cfg.Bg, cfg.Fg
	if bg == "" {
		bg = "#10161f"
	}
	if fg == "" {
		fg = "#e6e6e6"
	}
	return map[string]string{
		"--bg":           bg,
		"--fg":           fg,
		"--level-error":  cfg.LevelError,
		"--level-warn":   cfg.LevelWarn,
		"--level-info":   cfg.LevelInfo,
		"--level-debug":  cfg.LevelDebug,
		"--time":         cfg.Time,
		"--source":       cfg.Source,
		"--trace":        cfg.TraceID,
		"--thread":       cfg.Thread,
		"--selected":     cfg.Selected,
		"--visual":       cfg.Visual,
		"--accent":       cfg.Accent,
		"--dim":          cfg.Dim,
		"--highlight":    cfg.Highlight,
		"--popup-border": cfg.PopupBorder,
		"--popup-bg":     cfg.PopupBg,
	}
}

// UpdateTheme 写回共享 rules.yaml 的 theme 键。
func UpdateTheme(path, name string) error {
	return updateRulesKey(path, "theme", name)
}
