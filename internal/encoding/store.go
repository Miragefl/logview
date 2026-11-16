package encoding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// storeMaxEntries 记忆上限(用户拍板):只记非 UTF-8 的手动选择,GBK 源不一定
// 常看,LRU 截 50 条防膨胀。
const storeMaxEntries = 50

// storeEntry 单条编码记忆。TS 用纳秒精度:同秒内多次写入仍可按时间排序,
// 相同 TS 时按 key 字典序兜底,保证 LRU 淘汰的确定性。
type storeEntry struct {
	Enc string `json:"enc"`
	TS  int64  `json:"ts"`
}

// Store 编码按源记忆(encoding.json)。key 精确到路径(用户拍板):
// "ssh:"+host+path / "frp:"+name+path / "k8sres:"+kind/name / "file:"+绝对路径。
// 持久化姿势对齐 internal/usage(MarshalIndent + os.WriteFile 直写)。
type Store struct {
	mu      sync.Mutex
	path    string
	entries map[string]storeEntry
	loaded  bool
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

// DefaultStorePath 状态目录下的 encoding.json 路径(HOME 不可得返回空串)。
func DefaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "logview", "encoding.json")
}

// LoadFor 查询源记忆;无记录/坏 JSON/解析失败一律 EncAuto(容错,不报错)。
func (s *Store) LoadFor(key string) Encoding {
	if key == "" {
		return EncAuto
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	e, ok := s.entries[key]
	if !ok {
		return EncAuto
	}
	enc, ok := ParseEncoding(e.Enc)
	if !ok {
		return EncAuto // 手动改坏文件:回落 auto 而非报错
	}
	return enc
}

// Remember 记录手动选择:GBK/GB18030 落盘(LRU 截断);UTF-8/Auto 删除条目
// (切回默认即清记忆);EncApp 忽略。key 为空(管道等无身份源)忽略。
func (s *Store) Remember(key string, enc Encoding) {
	if key == "" || enc == EncApp {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	if s.entries == nil {
		s.entries = map[string]storeEntry{}
	}
	if enc == EncUTF8 || enc == EncAuto {
		delete(s.entries, key)
	} else {
		s.entries[key] = storeEntry{Enc: enc.String(), TS: time.Now().UnixNano()}
		s.evictLocked()
	}
	s.saveLocked()
}

// evictLocked 超 storeMaxEntries 时淘汰最旧(TS 升序,同 TS 按 key 字典序)。
func (s *Store) evictLocked() {
	if len(s.entries) <= storeMaxEntries {
		return
	}
	type kv struct {
		k string
		e storeEntry
	}
	all := make([]kv, 0, len(s.entries))
	for k, e := range s.entries {
		all = append(all, kv{k, e})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].e.TS != all[j].e.TS {
			return all[i].e.TS < all[j].e.TS
		}
		return all[i].k < all[j].k
	})
	for _, x := range all[:len(all)-storeMaxEntries] {
		delete(s.entries, x.k)
	}
}

// loadLocked 惰性读盘:坏 JSON 容错为空 map(写盘时覆盖恢复)。
func (s *Store) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var disk struct {
		Entries map[string]storeEntry `json:"entries"`
	}
	if json.Unmarshal(raw, &disk) != nil {
		return
	}
	if len(disk.Entries) > 0 {
		s.entries = disk.Entries
	}
}

// saveLocked 直写落盘(对齐 usage 包姿势;条目 ≤50,直写风险可忽略)。
func (s *Store) saveLocked() {
	if s.path == "" {
		return
	}
	disk := struct {
		Entries map[string]storeEntry `json:"entries"`
	}{Entries: s.entries}
	raw, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(s.path), 0755)
	os.WriteFile(s.path, raw, 0644)
}
