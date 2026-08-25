package tui

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// 热点数据：记录用户在源选择器中的操作频次（SSH 主机、k8s context/namespace/资源），
// 候选列表按频次降序排列（常用在前），同频次按名称序。
// 计数带时间衰减（frecent）：每次使用先按距上次使用的间隔给旧计数打折再 +1，
// 最近常用的排最前，长期不用的自然沉底。半衰期 7 天。
// 持久化到 ~/.local/state/logview/usage.json（与 session.json 同目录）。

// usageEntry 单项计数：衰减后的频次 + 最后使用时间（Unix 秒）。
type usageEntry struct {
	Count    float64 `json:"count"`
	LastUsed int64   `json:"lastUsed"`
}

var (
	usageMu    sync.Mutex
	usageData  map[string]usageEntry
	usageDirty bool
)

// usageHalfLife 计数半衰期：间隔一个半衰期，旧计数衰减一半。
const usageHalfLife = 7 * 24 * time.Hour

func usagePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".local", "state", "logview")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "usage.json")
}

func loadUsage() map[string]usageEntry {
	if usageData != nil {
		return usageData
	}
	usageData = make(map[string]usageEntry)
	p := usagePath()
	if p == "" {
		return usageData
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return usageData
	}
	// 兼容旧格式（纯数字累计计数）：当作 count 迁移，LastUsed 取文件修改时间
	var legacy map[string]float64
	if err := json.Unmarshal(data, &legacy); err == nil {
		mtime := time.Now()
		if st, err := os.Stat(p); err == nil {
			mtime = st.ModTime()
		}
		for k, v := range legacy {
			usageData[k] = usageEntry{Count: v, LastUsed: mtime.Unix()}
		}
		return usageData
	}
	json.Unmarshal(data, &usageData)
	return usageData
}

// decayed 返回按时间衰减后的计数（读取时惰性衰减，不写回）。
func (e usageEntry) decayed(now time.Time) float64 {
	elapsed := now.Sub(time.Unix(e.LastUsed, 0))
	if elapsed <= 0 {
		return e.Count
	}
	// 半衰指数衰减：count × 0.5^(elapsed/halfLife)
	halves := elapsed.Seconds() / usageHalfLife.Seconds()
	return e.Count * math.Pow(0.5, halves)
}

// BumpUsage 记录一次使用：旧计数先按间隔衰减，再 +1，刷新 LastUsed。
func BumpUsage(key string) {
	if key == "" {
		return
	}
	usageMu.Lock()
	defer usageMu.Unlock()
	m := loadUsage()
	now := time.Now()
	old := m[key]
	m[key] = usageEntry{
		Count:    old.decayed(now) + 1,
		LastUsed: now.Unix(),
	}
	usageDirty = true
	saveUsageLocked()
}

// usageScore 返回 key 的当前（衰减后）频次。
func usageScore(key string) float64 {
	usageMu.Lock()
	defer usageMu.Unlock()
	return loadUsage()[key].decayed(time.Now())
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
	usageFRPConn = "frp:"
)

// sortCandidatesHot 候选按（衰减频次降序, 名称升序）排序；带 hot 标记（★ 前缀给常用项 label）。
func sortCandidatesHot(items []sourceCandidate, keyPrefix string, mark bool) []sourceCandidate {
	if len(items) == 0 {
		return items
	}
	usageMu.Lock()
	m := loadUsage()
	usageMu.Unlock()
	now := time.Now()
	type rank struct {
		item  sourceCandidate
		score float64
	}
	ranks := make([]rank, len(items))
	for i, it := range items {
		ranks[i] = rank{item: it, score: m[keyPrefix+it.value].decayed(now)}
	}
	sort.SliceStable(ranks, func(i, j int) bool {
		if ranks[i].score != ranks[j].score {
			return ranks[i].score > ranks[j].score
		}
		return ranks[i].item.label < ranks[j].item.label
	})
	out := make([]sourceCandidate, len(ranks))
	for i, r := range ranks {
		it := r.item
		if mark && r.score >= 1 {
			it.label = "★ " + it.label
		}
		out[i] = it
	}
	return out
}
