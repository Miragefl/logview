package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/justfun/logview/internal/frp"
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
	// browse FRP 浏览会话的幂等隧道注册表（Final Review C2/C3）
	browse *fview.BrowseRegistry[*frp.Tunnel]
	// storeMu 连接库 CRUD 串行化（frp.Store 的 Upsert/Delete/Find 裸操作非线程安全，Final Review I1）
	storeMu sync.Mutex
}

// NewApp 构造后端绑定根（emit 直连 Wails EventsEmit）。
func NewApp() *App {
	a := &App{browse: fview.NewBrowseRegistry[*frp.Tunnel]()}
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

// ---- 连接库（共享正本：frp.json Servers/Conns + rules.yaml ssh_hosts）----

// FrpServers frps 服务器列表（frp.json）。
func (a *App) FrpServers() []frp.Server { return frp.LoadStore().Servers }

// FrpConns frp 连接记录列表（frp.json）。
func (a *App) FrpConns() []frp.Conn { return frp.LoadStore().Conns }

// UpsertFrpServer 新增/更新 frps 服务器并落盘。
func (a *App) UpsertFrpServer(s frp.Server) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	st := frp.LoadStore()
	st.UpsertServer(s)
	return st.Save()
}

// UpsertFrpConn 新增/更新连接记录并落盘（确认日志路径也走这里）。
func (a *App) UpsertFrpConn(c frp.Conn) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	st := frp.LoadStore()
	st.UpsertConn(c)
	return st.Save()
}

// DeleteFrpServer 删除服务器（仍被连接引用时返回错误）。
func (a *App) DeleteFrpServer(name string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	st := frp.LoadStore()
	if err := st.DeleteServer(name); err != nil {
		return err
	}
	return st.Save()
}

// DeleteFrpConn 删除连接记录。
func (a *App) DeleteFrpConn(name string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	st := frp.LoadStore()
	st.DeleteConn(name)
	return st.Save()
}

// SSHHosts SSH 主机列表（rules.yaml ssh_hosts，TUI 同源）。
func (a *App) SSHHosts() []string {
	if res, err := parser.LoadRules(rulesPath()); err == nil {
		return res.SSHHosts
	}
	return nil
}

// AddSSHHost 新增 SSH 主机（去重）写回共享正本。
func (a *App) AddSSHHost(host string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	hosts := a.SSHHosts()
	for _, h := range hosts {
		if h == host {
			return nil
		}
	}
	return fview.UpdateSSHHosts(rulesPath(), append(hosts, host))
}

// RemoveSSHHost 移除 SSH 主机。
func (a *App) RemoveSSHHost(host string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	hosts := a.SSHHosts()
	out := hosts[:0]
	for _, h := range hosts {
		if h != host {
			out = append(out, h)
		}
	}
	return fview.UpdateSSHHosts(rulesPath(), out)
}

// ---- 远程目录浏览 ----

// DirEntryDTO 目录条目（前端树/列表渲染）。
type DirEntryDTO struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
}

// SSHBrowse 列出 SSH 主机远程目录（密码空串走密钥认证；认证失败返回明确错误供前端弹密码重试）。
func (a *App) SSHBrowse(host, path, password string) ([]DirEntryDTO, error) {
	entries, err := stream.SSHListDir(host, path, password)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntryDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, DirEntryDTO{Name: e.Name, IsDir: e.IsDir})
	}
	return out, nil
}

// OpenSSHTab 打开 SSH 远程日志 tab（远程 tail 恒 follow；密码会话态直传不落盘）。
func (a *App) OpenSSHTab(host, path string, tailLines int, password string) (string, error) {
	if tailLines <= 0 {
		tailLines = defaultBufSize
	}
	src := stream.NewSSHSource(host, path, tailLines)
	src.SetBufferLines(defaultBufSize)
	if password != "" {
		src.SetPassword(password)
	}
	id, err := a.mgr.OpenTab(src)
	if err != nil {
		return "", err
	}
	// 远程 tail 命令内建 follow，恒跟随
	if err := a.mgr.SetFollow(id, true); err != nil {
		return id, err
	}
	return id, nil
}

// ---- FRP 一键连接（AcquireTunnel 引用计数复用）----

// BrowseFRP 建隧道浏览远程目录（幂等持有一份引用，重复浏览复用不累加——
// Final Review C3；认证失败返回明确错误供前端弹密码）。
func (a *App) BrowseFRP(connName, path, password string) ([]DirEntryDTO, error) {
	st := frp.LoadStore()
	conn, srv, err := fview.FRPLookup(st, connName)
	if err != nil {
		return nil, err
	}
	t, err := a.browse.Get(connName, func() (*frp.Tunnel, error) {
		return frp.AcquireTunnel(srv, conn.SK, conn.Proxy)
	})
	if err != nil {
		return nil, fmt.Errorf("建立隧道失败: %w", err)
	}
	// 目标必须带连接用户名（Final Review C1：丢 user@ 会以本地用户名连远端 sshd）
	entries, err := stream.SSHListDirWithPort(fview.FRPBrowseHost(conn), t.LocalPort(), path, password)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntryDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, DirEntryDTO{Name: e.Name, IsDir: e.IsDir})
	}
	return out, nil
}

// ReleaseFRPBrowse 释放浏览侧隧道引用（浏览未转化为 tab 时不泄漏 frpc）。
func (a *App) ReleaseFRPBrowse(connName string) {
	a.browse.Release(connName, func(t *frp.Tunnel) { _ = t.Cleanup() })
}

// OpenFRPTab FRP 一键连接开 tab（同 key 隧道复用；tab 关闭时 FRPSource.Cleanup 减引用）。
func (a *App) OpenFRPTab(connName, path string, tailLines int, password string) (string, error) {
	if tailLines <= 0 {
		tailLines = defaultBufSize
	}
	st := frp.LoadStore()
	conn, srv, err := fview.FRPLookup(st, connName)
	if err != nil {
		return "", err
	}
	t, err := frp.AcquireTunnel(srv, conn.SK, conn.Proxy)
	if err != nil {
		return "", fmt.Errorf("建立隧道失败: %w", err)
	}
	src := stream.NewFRPSource(conn.Name, t, conn.User, path, tailLines)
	src.SetBufferLines(defaultBufSize)
	if password != "" {
		src.SetPassword(password)
	}
	id, err := a.mgr.OpenTab(src)
	if err != nil {
		_ = t.Cleanup() // 开 tab 失败回退本次引用
		return "", err
	}
	if err := a.mgr.SetFollow(id, true); err != nil {
		return id, err
	}
	return id, nil
}

// shutdown 退出兜底：杀全部 frpc visitor（漏网隧道零残留）。
func (a *App) shutdown(ctx context.Context) {
	frp.KillAllTunnels()
}

// ---- k8s 浏览绑定 ----

// K8sContexts 列出 kubeconfig 全部 context。
func (a *App) K8sContexts() ([]string, error) { return fview.K8sContexts() }

// K8sNamespaces 列出 context 的 namespace。
func (a *App) K8sNamespaces(kubeCtx string) ([]string, error) { return fview.K8sNamespaces(kubeCtx) }

// K8sResources 列出 namespace 的工作负载资源。
func (a *App) K8sResources(kubeCtx, namespace string) ([]fview.ResourceDTO, error) {
	return fview.K8sResources(kubeCtx, namespace)
}

// OpenK8sTab 打开 k8s 资源日志 tab（resource 格式 kind/name，支持 deploy/sts/pod 简称）。
func (a *App) OpenK8sTab(resource, namespace, kubeCtx string, tailLines int) (string, error) {
	if tailLines <= 0 {
		tailLines = defaultBufSize
	}
	if _, err := stream.ParseK8sResource(resource); err != nil {
		return "", err
	}
	src := stream.NewK8sSource(resource, namespace, nil, tailLines)
	src.SetContext(kubeCtx)
	id, err := a.mgr.OpenTab(src)
	if err != nil {
		return "", err
	}
	if err := a.mgr.SetFollow(id, true); err != nil {
		return id, err
	}
	return id, nil
}
