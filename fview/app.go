package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/justfun/logview/internal/fview"
	"github.com/justfun/logview/internal/logcore"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stream"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// defaultBufSize 默认尾读行数（spec 4.2：10 万行）。
const defaultBufSize = 100000

// App fview 后端绑定根：tab 会话管理器 + 文件打开。
type App struct {
	ctx context.Context
	mgr *fview.Manager
}

// NewApp 构造后端绑定根（emit 直连 Wails EventsEmit）。
func NewApp() *App {
	a := &App{}
	a.mgr = fview.NewManager(func(event string, data any) {
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, event, data)
		}
	}, defaultBufSize)
	a.loadParsers()
	return a
}

// rulesPath 共享 rules.yaml 路径（与 TUI 同一份正本）。
func rulesPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "logview", "rules.yaml")
}

// loadParsers 加载共享 rules.yaml 解析规则；失败降级为全兜底行。
func (a *App) loadParsers() {
	res, err := parser.LoadRules(rulesPath())
	if err != nil {
		return
	}
	ps, err := parser.CompileRules(res.Rules)
	if err != nil {
		return
	}
	a.mgr.SetParsers(ps)
}

// startup Wails 启动回调。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// OpenFileTab 打开本地文件 tab（多路径聚合）；tailLines ≤0 取默认尾读行数；
// follow=true 走 tail 源实时追踪（.gz 归档由 stream 层降级为读满即止）。
func (a *App) OpenFileTab(paths []string, tailLines int, follow bool) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("未选择文件")
	}
	if tailLines <= 0 {
		tailLines = defaultBufSize
	}
	var src stream.LogStream
	if follow {
		src = stream.NewTailSource(paths, tailLines)
	} else {
		src = stream.NewFileSource(paths).WithTailLines(tailLines)
	}
	id, err := a.mgr.OpenTab(src)
	if err == nil && follow {
		err = a.mgr.SetFollow(id, true)
	}
	return id, err
}

// PickAndOpenFile 弹系统文件选择框（多选）并开 tab；follow 请求实时追踪；取消返回空串。
func (a *App) PickAndOpenFile(follow bool) (string, error) {
	paths, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "选择日志文件",
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	return a.OpenFileTab(paths, 0, follow)
}

// SetFollow 切换 tab 跟随态（前端跟随按钮）。
func (a *App) SetFollow(tabID string, follow bool) error {
	return a.mgr.SetFollow(tabID, follow)
}

// CloseTab 关闭 tab 并清理流资源。
func (a *App) CloseTab(tabID string) error {
	return a.mgr.CloseTab(tabID)
}

// Tabs 返回 tab 元信息列表。
func (a *App) Tabs() []fview.TabInfo {
	return a.mgr.Tabs()
}

// ParseQuery 解析搜索语法（logcore 单源；错误经 parseError 透出）。
func (a *App) ParseQuery(input string) fview.QueryMeta {
	return fview.ParseQuery(input)
}

// MatchLines 全量匹配 tab 行返回命中索引（Go 侧单源执行）。
func (a *App) MatchLines(tabID, query string) []int {
	return a.mgr.MatchLines(tabID, query)
}

// Lines 拉取 tab 行窗口（end<0 表示到末尾）。
func (a *App) Lines(tabID string, start, end int) []fview.LineDTO {
	return a.mgr.Lines(tabID, start, end)
}

// Rules 读共享 rules.yaml 解析规则列表（正则配置 UI 数据源）。
func (a *App) Rules() []parser.RuleConfig {
	if res, err := parser.LoadRules(rulesPath()); err == nil {
		return res.Rules
	}
	return nil
}

// SaveRules 写回解析规则并重载（已开 tab 不受影响，新 tab 生效）。
func (a *App) SaveRules(rules []parser.RuleConfig) error {
	if err := fview.UpdateRules(rulesPath(), rules); err != nil {
		return err
	}
	a.loadParsers()
	return nil
}

// Hides 读共享 rules.yaml 隐藏词。
func (a *App) Hides() []string {
	if res, err := parser.LoadRules(rulesPath()); err == nil {
		return res.Hides
	}
	return nil
}

// SaveHides 写回隐藏词（共享正本，TUI 下次启动生效）。
func (a *App) SaveHides(list []string) error {
	return fview.UpdateHides(rulesPath(), list)
}

// Fields 读字段显隐配置。
func (a *App) Fields() []parser.FieldConfig {
	if res, err := parser.LoadRules(rulesPath()); err == nil {
		return res.Fields
	}
	return nil
}

// SaveFields 写回字段显隐（共享正本）。
func (a *App) SaveFields(fields []parser.FieldConfig) error {
	return fview.UpdateFields(rulesPath(), fields)
}

// ThemeNames 内置主题名列表（logcore 单源）。
func (a *App) ThemeNames() []string {
	return logcore.ThemeNames()
}

// ThemeCSS 主题转 CSS 变量 KV（前端 :root 注入）。
func (a *App) ThemeCSS(name string) map[string]string {
	return fview.ThemeToCSS(logcore.ResolveTheme(name, nil))
}

// CurrentTheme 读共享 rules.yaml 当前主题名。
func (a *App) CurrentTheme() string {
	if res, err := parser.LoadRules(rulesPath()); err == nil {
		return res.Theme
	}
	return ""
}

// SetTheme 写回主题（共享正本，TUI 下次启动生效）。
func (a *App) SetTheme(name string) error {
	return fview.UpdateTheme(rulesPath(), name)
}

// GetLine 取单行详情（七标准字段 + 原文 + Unix 毫秒），idx 越界返回 nil。
func (a *App) GetLine(tabID string, idx int) map[string]string {
	ls := a.mgr.Lines(tabID, idx, idx+1)
	if len(ls) != 1 {
		return nil
	}
	ln := ls[0]
	return map[string]string{
		"seq":     fmt.Sprintf("%d", ln.Seq),
		"time":    ln.Time,
		"level":   ln.Level,
		"thread":  ln.Thread,
		"traceId": ln.TraceID,
		"logger":  ln.Logger,
		"message": ln.Message,
		"raw":     ln.Raw,
	}
}
