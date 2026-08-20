package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// 源选择器（o 键）：K8s / 本地 / SSH 三 tab，选中后 ReplaceStream 热切换。
// 候选查询（kubectl）异步执行，结果经 candidatesMsg 回填，避免阻塞渲染。

type sourceCandidate struct {
	label string // 展示文本
	value string // 提交值（k8s: kind/name；本地：路径；ssh: host）
}

type candidatesMsg struct {
	tab   int
	items []sourceCandidate
	ns    string
	err   error
}

// sshHostCandidates 全局注入（loadConfig 时 set）。
var sshHostCandidates []string

// SetSSHHosts 注入 SSH 主机候选（rules.yaml ssh_hosts + ~/.ssh/config）。
func SetSSHHosts(hosts []string) {
	sshHostCandidates = hosts
}

// fetchK8sCandidatesCmd 异步拉取 namespace 下的 deploy/sts/pod 列表。
func fetchK8sCandidatesCmd(ns string) tea.Cmd {
	return func() tea.Msg {
		var items []sourceCandidate
		var lastErr error
		for _, kind := range []string{"deployment", "statefulset", "pod"} {
			args := []string{"get", kind, "-n", ns, "-o", "jsonpath={.items[*].metadata.name}"}
			out, err := exec.Command("kubectl", args...).Output()
			if err != nil {
				lastErr = err
				continue // 无该资源类型时跳过，其余 kind 仍可用
			}
			for _, n := range strings.Fields(string(out)) {
				items = append(items, sourceCandidate{
					label: fmt.Sprintf("%s/%s", shortKind(kind), n),
					value: fmt.Sprintf("%s/%s", kind, n),
				})
			}
		}
		return candidatesMsg{tab: 0, items: items, ns: ns, err: lastErr}
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

// loadLocalCandidates 列出 dir 下的日志文件，按名排序。
func loadLocalCandidates(dir string) []sourceCandidate {
	if dir == "" {
		cwd, _ := os.Getwd()
		dir = cwd
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var items []sourceCandidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.Contains(lower, "log") {
			items = append(items, sourceCandidate{label: e.Name(), value: filepath.Join(dir, e.Name())})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].label < items[j].label })
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
