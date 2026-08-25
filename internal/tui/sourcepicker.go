package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/frp"
	"github.com/justfun/logview/internal/stream"
)

// 源选择器（o 键）：K8s / 本地 / SSH 三 tab，选中后 ReplaceStream 热切换。
// 候选查询（kubectl）异步执行，结果经 candidatesMsg 回填，避免阻塞渲染。

type sourceCandidate struct {
	label string // 展示文本
	value string // 提交值（k8s: kind/name；本地：路径；ssh: host）
	dir   bool   // 目录条目（浏览态 Enter 进入而非打开）
}

type candidatesMsg struct {
	tab   int
	items []sourceCandidate
	ns    string
	kind  string // contexts / namespaces / 空=资源列表 / sshdir
	err   error
}

// sshHostCandidates 全局注入（loadConfig 时 set）。
var sshHostCandidates []string

// SetSSHHosts 注入 SSH 主机候选（rules.yaml ssh_hosts + ~/.ssh/config）。
func SetSSHHosts(hosts []string) {
	sshHostCandidates = hosts
}

// kubectlTimeout 防止 RBAC/网络问题导致候选查询挂起。
func kubectlOutput(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "kubectl", args...).Output()
}

// withKubeCtx 前插 --context（空 context 原样返回）。
func withKubeCtx(ctxName string, args ...string) []string {
	if ctxName == "" {
		return args
	}
	return append([]string{"--context", ctxName}, args...)
}

// fetchK8sContextsCmd 异步拉取全部 context 名。
func fetchK8sContextsCmd() tea.Cmd {
	return func() tea.Msg {
		out, err := kubectlOutput("config", "get-contexts", "-o", "name")
		if err != nil {
			return candidatesMsg{tab: 0, kind: "contexts", err: err}
		}
		var items []sourceCandidate
		for _, name := range strings.Fields(string(out)) {
			items = append(items, sourceCandidate{label: name, value: name})
		}
		return candidatesMsg{tab: 0, kind: "contexts", items: items}
	}
}

// currentK8sContext 返回当前 context（kubectl 不可用时为空）。
func currentK8sContext() string {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// useK8sContext 切换默认 context（全局生效）。
func useK8sContext(name string) error {
	return exec.Command("kubectl", "config", "use-context", name).Run()
}

// fetchK8sNamespacesCmd 异步拉取 namespace 列表（ctxName 空=当前 context）。
func fetchK8sNamespacesCmd(ctxName string) tea.Cmd {
	return func() tea.Msg {
		out, err := kubectlOutput(withKubeCtx(ctxName, "get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}")...)
		if err != nil {
			return candidatesMsg{tab: 0, kind: "namespaces", err: err}
		}
		var items []sourceCandidate
		for _, name := range strings.Fields(string(out)) {
			items = append(items, sourceCandidate{label: name, value: name})
		}
		return candidatesMsg{tab: 0, kind: "namespaces", items: items}
	}
}

// fetchK8sCandidatesCmd 异步拉取 namespace 下的 deploy/sts/pod 列表。
func fetchK8sCandidatesCmd(ctxName, ns string) tea.Cmd {
	return func() tea.Msg {
		var items []sourceCandidate
		var lastErr error
		for _, kind := range []string{"deployment", "statefulset", "pod"} {
			out, err := kubectlOutput(withKubeCtx(ctxName, "get", kind, "-n", ns, "-o", "jsonpath={.items[*].metadata.name}")...)
			if err != nil {
				lastErr = err
				continue // 无该资源类型/无权限时跳过，其余 kind 仍可用
			}
			for _, n := range strings.Fields(string(out)) {
				items = append(items, sourceCandidate{
					label: fmt.Sprintf("%s/%s", shortKind(kind), n),
					value: fmt.Sprintf("%s/%s", kind, n),
				})
			}
		}
		return candidatesMsg{tab: 0, kind: "resources", items: items, ns: ns, err: lastErr}
	}
}

func shortKind(kind string) string {
	switch kind {
	case "deployment":
		return "deploy"
	case "statefulset":
		return "sts"
	}
	return kind
}

// localEntry 本地目录条目（目录或日志文件）。
type localEntry struct {
	name  string
	isDir bool
}

// listLocalDir 列出 dir 的内容：目录在前（/ 后缀）、文件在后，按名排序。
// 显示全部文件（日志文件排最前，其余文件靠后），showHidden=false 时隐藏 . 开头条目。
func listLocalDir(dir string, showHidden bool) []localEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs, logs, others []localEntry
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, localEntry{name: name, isDir: true})
			continue
		}
		if strings.Contains(strings.ToLower(name), "log") {
			logs = append(logs, localEntry{name: name})
		} else {
			others = append(others, localEntry{name: name})
		}
	}
	sortBy := func(s []localEntry) {
		sort.Slice(s, func(i, j int) bool { return s[i].name < s[j].name })
	}
	sortBy(dirs)
	sortBy(logs)
	sortBy(others)
	return append(append(dirs, logs...), others...)
}

// loadLocalCandidates 旧接口保留：供输入框路径过滤场景（非浏览态）。
func loadLocalCandidates(dir string) []sourceCandidate {
	var items []sourceCandidate
	for _, e := range listLocalDir(dir, false) {
		if e.isDir {
			continue
		}
		items = append(items, sourceCandidate{label: e.name, value: filepath.Join(dir, e.name)})
	}
	return items
}

// sshCandidates 合并全局主机候选为选择器条目（按使用频次降序，常用在前）。
func sshCandidates() []sourceCandidate {
	var items []sourceCandidate
	for _, h := range sshHostCandidates {
		items = append(items, sourceCandidate{label: h, value: h})
	}
	return sortCandidatesHot(items, usageSSHHost, true)
}

// fetchSSHDirCmd 异步拉取远程目录列表（password 非空走密码认证）。
func fetchSSHDirCmd(host, path, password string) tea.Cmd {
	return func() tea.Msg {
		entries, err := stream.SSHListDir(host, path, password)
		if err != nil {
			return candidatesMsg{tab: 2, kind: "sshdir", ns: host + ":" + path, err: err}
		}
		var items []sourceCandidate
		for _, e := range entries {
			if e.IsDir {
				items = append(items, sourceCandidate{label: e.Name + "/", value: e.Name, dir: true})
			} else {
				items = append(items, sourceCandidate{label: e.Name, value: e.Name})
			}
		}
		return candidatesMsg{tab: 2, kind: "sshdir", ns: host + ":" + path, items: items}
	}
}

// frpStoreRef 全局注入（loadConfig 时 set；测试用 SetFRPStore 覆盖）。
var frpStoreRef *frp.Store

// SetFRPStore 注入 frp 连接存储。
func SetFRPStore(s *frp.Store) { frpStoreRef = s }

func frpStore() *frp.Store {
	if frpStoreRef == nil {
		return frp.LoadStore()
	}
	return frpStoreRef
}

// frpTunnelHandle tui 侧隧道句柄（*frp.Tunnel / 测试 fake 均实现）。
type frpTunnelHandle interface {
	LocalPort() int
	Cleanup() error
}

// startFRPTunnel 可注入的隧道启动（测试替换；生产包装 frp.StartTunnel）。后续任务使用。
var startFRPTunnel = func(server frp.Server, sk, proxy string) (frpTunnelHandle, error) {
	return frp.StartTunnel(server, sk, proxy)
}

// frpConnCandidates FRP L0 候选：+ 新建连接 恒在首位，其后已存记录（频次降序）。
func frpConnCandidates(filter string) []sourceCandidate {
	var conns []sourceCandidate
	for _, c := range frpStore().Conns {
		conns = append(conns, sourceCandidate{
			label: c.Name + "  " + c.Proxy + " " + c.Path,
			value: c.Name,
		})
	}
	conns = sortCandidatesHot(conns, usageFRPConn, true)
	out := []sourceCandidate{{label: "+ 新建连接", value: "+new"}}
	f := strings.ToLower(strings.TrimSpace(filter))
	for _, c := range conns {
		if f == "" || strings.Contains(strings.ToLower(c.label), f) {
			out = append(out, c)
		}
	}
	return out
}

// frpServerCandidates 表单 step0 候选：手动输入 恒在首位，其后已存服务器。
func frpServerCandidates(filter string) []sourceCandidate {
	out := []sourceCandidate{{label: "手动输入新服务器…", value: "+manual"}}
	f := strings.ToLower(strings.TrimSpace(filter))
	for _, s := range frpStore().Servers {
		label := s.Name + "  " + s.Addr
		if f == "" || strings.Contains(strings.ToLower(label), f) {
			out = append(out, sourceCandidate{label: label, value: s.Name})
		}
	}
	return out
}
