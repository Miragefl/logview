package frp

// frp stcp 连接参数持久化：~/.local/state/logview/frp.json（与 usage.json/session.json 同目录）。
// 存两类数据：frps 服务器（地址+token）、连接记录（frps 引用+sk+proxy+用户+日志路径）。
// 文件缺失/损坏 → 空 store 降级，不报错。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Server struct {
	Name  string `json:"name"`
	Addr  string `json:"addr"` // host:port
	Token string `json:"token"`
}

type Conn struct {
	Name   string `json:"name"` // 记录名（默认 = proxy 名）
	Server string `json:"server"`
	SK     string `json:"sk"`
	Proxy  string `json:"proxy"`
	User   string `json:"user"`
	Path   string `json:"path"` // 远程日志路径（直达 tail 用）
}

type Store struct {
	Servers []Server `json:"servers"`
	Conns   []Conn   `json:"connections"`
}

var (
	storeMu   sync.Mutex
	storeData *Store
	storeFile string // 测试可覆盖
)

func storePath() string {
	if storeFile != "" {
		return storeFile
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".local", "state", "logview")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "frp.json")
}

// LoadStore 全局单例：读失败/损坏 → 空 store。
func LoadStore() *Store {
	storeMu.Lock()
	defer storeMu.Unlock()
	if storeData != nil {
		return storeData
	}
	storeData = &Store{}
	p := storePath()
	if p == "" {
		return storeData
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return storeData
	}
	json.Unmarshal(data, storeData) // 损坏 → 保持空 store
	return storeData
}

func (s *Store) Save() error {
	storeMu.Lock()
	defer storeMu.Unlock()
	p := storePath()
	if p == "" {
		return os.ErrNotExist
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// UpsertServer 按 Name 去重覆盖。
func (s *Store) UpsertServer(v Server) {
	for i := range s.Servers {
		if s.Servers[i].Name == v.Name {
			s.Servers[i] = v
			return
		}
	}
	s.Servers = append(s.Servers, v)
}

// UpsertConn 按 Name 去重覆盖（确认日志文件时更新 Path 也走这里）。
func (s *Store) UpsertConn(c Conn) {
	for i := range s.Conns {
		if s.Conns[i].Name == c.Name {
			s.Conns[i] = c
			return
		}
	}
	s.Conns = append(s.Conns, c)
}

// DeleteConn 按 Name 删除连接记录，返回是否存在。
func (s *Store) DeleteConn(name string) bool {
	for i := range s.Conns {
		if s.Conns[i].Name == name {
			s.Conns = append(s.Conns[:i], s.Conns[i+1:]...)
			return true
		}
	}
	return false
}

// DeleteServer 按 Name 删除服务器（仍被连接引用时不删，返回错误）。
func (s *Store) DeleteServer(name string) error {
	for _, c := range s.Conns {
		if c.Server == name {
			return fmt.Errorf("服务器 %s 仍被连接 %s 引用，先删连接", name, c.Name)
		}
	}
	for i := range s.Servers {
		if s.Servers[i].Name == name {
			s.Servers = append(s.Servers[:i], s.Servers[i+1:]...)
			return nil
		}
	}
	return os.ErrNotExist
}

func (s *Store) FindServer(name string) (Server, bool) {
	for _, v := range s.Servers {
		if v.Name == name {
			return v, true
		}
	}
	return Server{}, false
}

func (s *Store) FindConn(name string) (Conn, bool) {
	for _, c := range s.Conns {
		if c.Name == name {
			return c, true
		}
	}
	return Conn{}, false
}

// SetStoreFileForTest / ResetStoreForTest 测试专用（同包测试直接调用）。
func SetStoreFileForTest(p string) {
	storeMu.Lock()
	defer storeMu.Unlock()
	storeFile = p
	storeData = nil
}

func ResetStoreForTest() {
	storeMu.Lock()
	defer storeMu.Unlock()
	storeFile = ""
	storeData = nil
}
