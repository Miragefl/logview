// Package usage 热点数据共享包：记录用户在源选择中的操作频次（SSH 主机、
// k8s 资源、FRP 连接、本地文件、高亮/隐藏词条），候选列表按频次降序。
// 计数带时间衰减（frecent）：每次使用先按距上次使用的间隔给旧计数打折再 +1，
// 最近常用的排最前，长期不用的自然沉底。半衰期 7 天。
// 持久化到 ~/.local/state/logview/usage.json，TUI 与 fview 桌面端共用同一份。
package usage

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Entry 单项计数：衰减后的频次 + 最后使用时间（Unix 秒）。
// LastUsed 截断到整秒是有意行为：Bump 落盘后读回的惰性衰减带亚秒，
// 刚用一次读回 0.9999x，★ 门槛 0.99 依赖这个语义（见 tui sortCandidatesHot）。
type Entry struct {
	Count    float64 `json:"count"`
	LastUsed int64   `json:"lastUsed"`
}

var (
	mu    sync.Mutex
	data  map[string]Entry
	dirty bool
)

// halfLife 计数半衰期：间隔一个半衰期，旧计数衰减一半。
const halfLife = 7 * 24 * time.Hour

// 各候选域的 key 前缀。KeyFile 为 fview 桌面端本地文件新增；
// KeyHighlight/KeyHide 是词条历史（TUI C-r），不属于源列表。
const (
	KeySSHHost   = "ssh:"
	KeyK8sCtx    = "k8sctx:"
	KeyK8sNS     = "k8sns:"
	KeyK8sRes    = "k8sres:"
	KeyFRPConn   = "frp:"
	KeyFile      = "file:"
	KeyHighlight = "hl:"
	KeyHide      = "hide:"
)

// sourcePrefixes 源类 key 前缀集合（RecentSourceKeys 的过滤白名单）。
var sourcePrefixes = []string{KeySSHHost, KeyK8sCtx, KeyK8sNS, KeyK8sRes, KeyFRPConn, KeyFile}

// Path 返回 usage.json 落盘路径（HOME 不可得时返回空串）。
func Path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".local", "state", "logview")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "usage.json")
}

// Load 返回热点数据 map（首次调用时从磁盘加载并缓存）。
func Load() map[string]Entry {
	mu.Lock()
	defer mu.Unlock()
	return loadLocked()
}

// loadLocked 加载/返回缓存 map（调用方持 mu）。
func loadLocked() map[string]Entry {
	if data != nil {
		return data
	}
	data = make(map[string]Entry)
	p := Path()
	if p == "" {
		return data
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return data
	}
	// 兼容旧格式（纯数字累计计数）：当作 count 迁移，LastUsed 取文件修改时间
	var legacy map[string]float64
	if err := json.Unmarshal(raw, &legacy); err == nil {
		mtime := time.Now()
		if st, err := os.Stat(p); err == nil {
			mtime = st.ModTime()
		}
		for k, v := range legacy {
			data[k] = Entry{Count: v, LastUsed: mtime.Unix()}
		}
		return data
	}
	json.Unmarshal(raw, &data)
	return data
}

// Decayed 返回按时间衰减后的计数（读取时惰性衰减，不写回）。
func (e Entry) Decayed(now time.Time) float64 {
	elapsed := now.Sub(time.Unix(e.LastUsed, 0))
	if elapsed <= 0 {
		return e.Count
	}
	// 半衰指数衰减：count × 0.5^(elapsed/halfLife)
	halves := elapsed.Seconds() / halfLife.Seconds()
	return e.Count * math.Pow(0.5, halves)
}

// Bump 记录一次使用：旧计数先按间隔衰减，再 +1，刷新 LastUsed。
func Bump(key string) {
	if key == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	m := loadLocked()
	now := time.Now()
	old := m[key]
	m[key] = Entry{
		Count:    old.Decayed(now) + 1,
		LastUsed: now.Unix(),
	}
	dirty = true
	saveLocked()
}

// Score 返回 key 的当前（衰减后）频次。
func Score(key string) float64 {
	mu.Lock()
	defer mu.Unlock()
	return loadLocked()[key].Decayed(time.Now())
}

// RecentSourceKeys 源类 key 按衰减分降序返回（同分 LastUsed 新→旧、再词序升序），
// 截前 limit；高亮/隐藏词条前缀被排除。遍历与排序全程持锁——
// Wails 绑定多 goroutine 并发调用（Bump 写 map 与本遍历重叠会 fatal）。
func RecentSourceKeys(limit int) []string {
	mu.Lock()
	defer mu.Unlock()
	m := loadLocked()
	now := time.Now()
	type ranked struct {
		key   string
		score float64
		last  int64
	}
	var rs []ranked
	for k, e := range m {
		if !isSourceKey(k) {
			continue
		}
		rs = append(rs, ranked{key: k, score: e.Decayed(now), last: e.LastUsed})
	}
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].score != rs[j].score {
			return rs[i].score > rs[j].score
		}
		if rs[i].last != rs[j].last {
			return rs[i].last > rs[j].last
		}
		return rs[i].key < rs[j].key
	})
	if len(rs) > limit {
		rs = rs[:limit]
	}
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.key
	}
	return out
}

// isSourceKey 判断 key 是否源类（前缀白名单内）。
func isSourceKey(k string) bool {
	for _, p := range sourcePrefixes {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

func saveLocked() {
	if !dirty {
		return
	}
	p := Path()
	if p == "" {
		return
	}
	raw, err := json.MarshalIndent(loadLocked(), "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(p, raw, 0644)
	dirty = false
}

// ResetForTest 清内存缓存（磁盘数据按 HOME 重新加载）——仅测试用。
func ResetForTest() {
	mu.Lock()
	data = nil
	dirty = false
	mu.Unlock()
}
