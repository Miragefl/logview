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

// fetchK8sNamespacesCmd 异步拉取 namespace 列表。
func fetchK8sNamespacesCmd() tea.Cmd {
	return func() tea.Msg {
		out, err := kubectlOutput("get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}")
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
func fetchK8sCandidatesCmd(ns string) tea.Cmd {
	return func() tea.Msg {
		var items []sourceCandidate
		var lastErr error
		for _, kind := range []string{"deployment", "statefulset", "pod"} {
			out, err := kubectlOutput("get", kind, "-n", ns, "-o", "jsonpath={.items[*].metadata.name}")
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

// listLocalDir 列出 dir 的内容：目录在前（/ 后缀）、日志类文件在后，按名排序。
// showHidden=false 时隐藏 . 开头条目。
func listLocalDir(dir string, showHidden bool) []localEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs, logs []localEntry
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
		lower := strings.ToLower(name)
		if strings.Contains(lower, "log") {
			logs = append(logs, localEntry{name: name})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(logs, func(i, j int) bool { return logs[i].name < logs[j].name })
	return append(dirs, logs...)
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

// sshCandidates 合并全局主机候选为选择器条目。
func sshCandidates() []sourceCandidate {
	var items []sourceCandidate
	for _, h := range sshHostCandidates {
		items = append(items, sourceCandidate{label: h, value: h})
	}
	return items
}

// fetchSSHDirCmd 异步拉取远程目录列表。
func fetchSSHDirCmd(host, path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := stream.SSHListDir(host, path)
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
