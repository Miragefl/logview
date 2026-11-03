package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/justfun/logview/internal/frp"
	"github.com/justfun/logview/internal/fview"
	"github.com/justfun/logview/internal/logcore"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stream"
	"github.com/justfun/logview/internal/usage"
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
	// passes 密码仓（keyring 钥匙串，测试可注入 fake，passstore.go）
	passes passStore
	// storeMu 连接库 CRUD 串行化（frp.Store 的 Upsert/Delete/Find 裸操作非线程安全，Final Review I1）
	storeMu sync.Mutex
}

// NewApp 构造后端绑定根（emit 直连 Wails EventsEmit）。
func NewApp() *App {
	a := &App{browse: fview.NewBrowseRegistry[*frp.Tunnel](), passes: newKeyringPassStore()}
	a.mgr = fview.NewManager(func(event string, data any) {
		a.safeEmit(event, data)
	}, defaultBufSize)
	a.loadParsers()
	return a
}

// safeEmit Wails 事件广播（ctx 未注入时静默丢弃；Manager 流与菜单回调共用出口）。
// data 必须带 ... 展开传入 EventsEmit：漏展开会把整个切片当单个 optionalData
// 元素，前端回调收到 [X] 而非 X，tab 事件 payload 全嵌套变形（Final Review C1）。
func (a *App) safeEmit(event string, data ...any) {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, event, data...)
	}
}

// shortcutCloseTab mac 菜单 ⌘W 回调：广播 shortcut:closeTab，
// 前端 App.vue 订阅后关闭当前激活 tab（与 ctrl+w / 点 X 同路，spec 2026-11-03）。
func (a *App) shortcutCloseTab() {
	a.safeEmit("shortcut:closeTab")
}

// menuQuit mac 菜单 ⌘Q 回调：退出应用（shutdown 钩子兜底杀全部 frpc visitor）。
func (a *App) menuQuit() {
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
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
	// 展示名=文件名(多文件 +N);源信息条=全路径逗号拼接(验收反馈⑥)
	display := filepath.Base(paths[0])
	if len(paths) > 1 {
		display = fmt.Sprintf("%s+%d", display, len(paths)-1)
	}
	id, err := a.mgr.OpenTab(src, display, strings.Join(paths, ", "))
	if err == nil && follow {
		err = a.mgr.SetFollow(id, true)
	}
	if err == nil {
		// 本地文件热点（file: 前缀，与 TUI 共享 usage.json）——打开即计频，
		// 保证"最近文件"冷启动有数据（PickAndOpenFile 亦经此路径）
		for _, p := range paths {
			usage.Bump(usage.KeyFile + p)
		}
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

// GetSavedPass 取已存密码（空串=无；key 约定 ssh:{host} / frp:{connName}）。
func (a *App) GetSavedPass(key string) string {
	return a.passes.Get(key)
}

// SavePass 存密码（空密码不入库——密钥认证/免密不产生垃圾条目；改密重写同 key 覆盖）。
func (a *App) SavePass(key, pass string) error {
	if pass == "" {
		return nil
	}
	return a.passes.Set(key, pass)
}

// ClearTab 清屏（C 键:后端清会话缓冲并广播 tab:cleared,验收反馈④）。
func (a *App) ClearTab(tabID string) error {
	return a.mgr.ClearTab(tabID)
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

// Rules 读模板版规则列表（{name} 引用保留——展开正则写回会固化引用，编辑走模板）。
func (a *App) Rules() []parser.RuleConfig {
	if res, err := parser.LoadRules(rulesPath()); err == nil {
		return res.RawRules
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

// Patterns 读共享 rules.yaml 的 patterns（字段正则片段，规则 {name} 引用）。
func (a *App) Patterns() map[string]string {
	if res, err := parser.LoadRules(rulesPath()); err == nil {
		return res.Patterns
	}
	return map[string]string{}
}

// SavePatterns 写回 patterns 键并重载解析器。
func (a *App) SavePatterns(p map[string]string) error {
	if err := fview.UpdatePatterns(rulesPath(), p); err != nil {
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

// ---- 连接库（共享正本：frp.json 平铺官方字段 Conns + rules.yaml ssh_hosts）----

// FrpConns frp 连接记录列表（frp.json，v2 单层平铺官方字段）。
func (a *App) FrpConns() []frp.Conn { return frp.LoadStore().Conns }

// UpsertFrpConn 新增/更新连接记录并落盘（确认日志路径也走这里；入参归一 type/serverPort 默认值）。
// name 是主键：空值拒绝落盘（前端传参错位时全字段 undefined，防再出空记录）。
func (a *App) UpsertFrpConn(c frp.Conn) error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("连接 name 不能为空")
	}
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	st := frp.LoadStore()
	st.UpsertConn(c)
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

// SSHHostDTO SSH 主机条目：name + 是否来自 ~/.ssh/config 自动发现
//（前端据此隐藏删除键——发现主机删了也会被重新发现）。
type SSHHostDTO struct {
	Name          string `json:"name"`
	FromSSHConfig bool   `json:"fromSshConfig"`
}

// SSHHosts SSH 主机列表：rules.yaml ssh_hosts 与 ~/.ssh/config 发现结果合并
//（MergeSSHHosts 配置优先、去重，与 TUI cmd/root.go 装配同源同序）。
func (a *App) SSHHosts() []SSHHostDTO {
	discovered := stream.ParseSSHConfigHosts()
	discSet := make(map[string]bool, len(discovered))
	for _, h := range discovered {
		discSet[h] = true
	}
	merged := stream.MergeSSHHosts(a.rulesSSHHosts(), discovered)
	out := make([]SSHHostDTO, 0, len(merged))
	for _, h := range merged {
		out = append(out, SSHHostDTO{Name: h, FromSSHConfig: discSet[h]})
	}
	return out
}

// rulesSSHHosts rules.yaml ssh_hosts 正本（Add/Remove 只操作手动登记，不掺发现结果）。
func (a *App) rulesSSHHosts() []string {
	if res, err := parser.LoadRules(rulesPath()); err == nil {
		return res.SSHHosts
	}
	return nil
}

// AddSSHHost 新增 SSH 主机（去重）写回共享正本。
func (a *App) AddSSHHost(host string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	hosts := a.rulesSSHHosts()
	for _, h := range hosts {
		if h == host {
			return nil
		}
	}
	return fview.UpdateSSHHosts(rulesPath(), append(hosts, host))
}

// RemoveSSHHost 移除手动登记的 SSH 主机（~/.ssh/config 发现的主机不在此列，删不掉属预期）。
func (a *App) RemoveSSHHost(host string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	hosts := a.rulesSSHHosts()
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
	id, err := a.mgr.OpenTab(src, filepath.Base(path), host+":"+path)
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

// BrowseFRP 浏览远程目录：stcp 建隧道（幂等持有一份引用，重复浏览复用不累加——
// Final Review C3）；tcp 零配置直连 serverAddr:remotePort（无隧道无注册表）。
// 认证失败返回明确错误供前端弹密码。
func (a *App) BrowseFRP(connName, path, password string) ([]DirEntryDTO, error) {
	conn, err := fview.FRPLookup(frp.LoadStore(), connName)
	if err != nil {
		return nil, err
	}
	host, port := "", 0
	if conn.Type == frp.TypeTCP {
		host, port = fview.FRPBrowseTarget(conn, 0)
	} else {
		t, err := a.browse.Get(connName, func() (*frp.Tunnel, error) {
			return frp.AcquireTunnel(conn)
		})
		if err != nil {
			return nil, fmt.Errorf("建立隧道失败: %w", err)
		}
		host, port = fview.FRPBrowseTarget(conn, t.LocalPort())
	}
	entries, err := stream.SSHListDirWithPort(host, port, path, password)
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

// OpenFRPTab FRP 开 tab：stcp 建 visitor 隧道（同 key 复用；tab 关闭时
// FRPSource.Cleanup 减引用）；tcp 零配置直连（FRPDirectSource，无隧道）。
func (a *App) OpenFRPTab(connName, path string, tailLines int, password string) (string, error) {
	if tailLines <= 0 {
		tailLines = defaultBufSize
	}
	conn, err := fview.FRPLookup(frp.LoadStore(), connName)
	if err != nil {
		return "", err
	}
	// tab 标题用展示别名（空回落 name）；主键/密码 key 仍走 conn.Name
	disp := conn.Alias
	if disp == "" {
		disp = conn.Name
	}
	label := disp + " → " + conn.ServerAddr
	if conn.Type == frp.TypeTCP {
		src := stream.NewFRPDirectSource(conn.Name, conn.ServerAddr, conn.RemotePort, conn.User, path, tailLines)
		src.SetBufferLines(defaultBufSize)
		if password != "" {
			src.SetPassword(password)
		}
		id, err := a.mgr.OpenTab(src, filepath.Base(path), label)
		if err != nil {
			return "", err
		}
		if err := a.mgr.SetFollow(id, true); err != nil {
			return id, err
		}
		return id, nil
	}
	t, err := frp.AcquireTunnel(conn)
	if err != nil {
		return "", fmt.Errorf("建立隧道失败: %w", err)
	}
	src := stream.NewFRPSource(conn.Name, t, conn.User, path, tailLines)
	src.SetBufferLines(defaultBufSize)
	if password != "" {
		src.SetPassword(password)
	}
	id, err := a.mgr.OpenTab(src, filepath.Base(path), label)
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

// ---- 源热点/内存绑定（与 TUI 共享 usage.json） ----

// UsageRecent 源热点 key 列表（file:/ssh: 等前缀，衰减分降序），本地面板数据源。
func (a *App) UsageRecent() []string { return fview.UsageRecent() }

// UsageTouch 记录一次源使用（key 由前端按前缀规范拼接，如 file:/path/app.log）。
func (a *App) UsageTouch(key string) { fview.UsageTouch(key) }

// UsageKeywords 高亮/隐藏词频 top20（共享 usage.json，kind ∈ hl|hide，scope 空即全局）。
func (a *App) UsageKeywords(scope, kind string) []string { return fview.Keywords(scope, kind) }

// BumpKeyword 记一次关键词使用（与 TUI 同一份词频，验收反馈③）。
func (a *App) BumpKeyword(scope, kind, word string) { fview.BumpKeyword(scope, kind, word) }

// FoldRanges 堆栈折叠组快照（seq 域,e 键折叠渲染数据源,验收反馈④）。
func (a *App) FoldRanges(tabID string) []fview.FoldRange { return a.mgr.FoldRanges(tabID) }

// GetMemStats 状态栏内存徽章：Sys-HeapReleased 字节数（口径对齐 TUI）。
func (a *App) GetMemStats() uint64 { return fview.MemStatsBytes() }

// SystemFonts 系统字体名列表（外观页字体下拉数据源）。
func (a *App) SystemFonts() []string { return fview.SystemFonts() }

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
	id, err := a.mgr.OpenTab(src, resource, kubeCtx+" · "+namespace+"/"+resource)
	if err != nil {
		return "", err
	}
	if err := a.mgr.SetFollow(id, true); err != nil {
		return id, err
	}
	return id, nil
}
