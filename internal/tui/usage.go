package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// 热点数据：记录用户在源选择器中的操作频次（SSH 主机、k8s context/namespace/资源），
// 候选列表按频次降序排列（常用在前），同频次按名称序。
// 持久化到 ~/.local/state/logview/usage.json（与 session.json 同目录）。

var (
	usageMu    sync.Mutex
	usageData  map[string]int
	usageDirty bool
)

func usagePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".local", "state", "logview")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "usage.json")
}

func loadUsage() map[string]int {
	if usageData != nil {
		return usageData
	}
	usageData = make(map[string]int)
	if p := usagePath(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			json.Unmarshal(data, &usageData)
		}
	}
	return usageData
}

// BumpUsage 记录一次使用（key 形如 "ssh:lane" / "k8sctx:x" / "k8sns:y" / "k8sres:deployment/z"）。
func BumpUsage(key string) {
	if key == "" {
		return
	}
	usageMu.Lock()
	defer usageMu.Unlock()
	m := loadUsage()
	m[key]++
	usageDirty = true
	saveUsageLocked()
}

// UsageCount 返回 key 的使用频次。
func UsageCount(key string) int {
	usageMu.Lock()
	defer usageMu.Unlock()
	return loadUsage()[key]
}

func saveUsageLocked() {
	if !usageDirty {
		return
	}
	p := usagePath()
	if p == "" {
		return
	}
	data, err := json.MarshalIndent(loadUsage(), "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(p, data, 0644)
	usageDirty = false
}

// usageKey 各候选域的 key 前缀。
const (
	usageSSHHost = "ssh:"
	usageK8sCtx  = "k8sctx:"
	usageK8sNS   = "k8sns:"
	usageK8sRes  = "k8sres:"
)

// sortCandidatesHot 候选按（频次降序, 名称升序）排序；带 hot 标记（★ 前缀给常用项 label）。
func sortCandidatesHot(items []sourceCandidate, keyPrefix string, mark bool) []sourceCandidate {
	if len(items) == 0 {
		return items
	}
	usageMu.Lock()
	m := loadUsage()
	usageMu.Unlock()
	type rank struct {
		item  sourceCandidate
		count int
	}
	ranks := make([]rank, len(items))
	for i, it := range items {
		ranks[i] = rank{item: it, count: m[keyPrefix+it.value]}
	}
	sort.SliceStable(ranks, func(i, j int) bool {
		if ranks[i].count != ranks[j].count {
			return ranks[i].count > ranks[j].count
		}
		return ranks[i].item.label < ranks[j].item.label
	})
	out := make([]sourceCandidate, len(ranks))
	for i, r := range ranks {
		it := r.item
		if mark && r.count > 0 {
			it.label = "★ " + it.label
		}
		out[i] = it
	}
	return out
}
