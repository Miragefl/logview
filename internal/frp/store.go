package frp

// frp 连接参数持久化：~/.local/state/logview/frp.json（与 usage.json/session.json 同目录）。
// v2：单层平铺官方字段（type/serverAddr/serverPort/token/serverName/secretKey/remotePort/user），
// 所见即 frpc 参数；v1（servers+connections 两层引用）读时自动迁移。
// 文件缺失/损坏 → 空 store 降级，不报错。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// 连接类型：stcp=拉 visitor 隧道；tcp=frps remotePort 直连（零配置，不拉 frpc）。
const (
	TypeSTCP = "stcp"
	TypeTCP  = "tcp"
)

// Conn 一条 FRP 连接（字段名与 frp 官方 TOML 一致；Name 是 fview/TUI 记录名，不进 frpc）。
type Conn struct {
	Name       string `json:"name"`       // 记录名（主键：增删改查/密码 key/词频 Scope）
	Alias      string `json:"alias"`      // 展示别名（可选；列表/tab 标题优先它，空回落 Name——纯展示不进任何逻辑键）
	Type       string `json:"type"`       // stcp | tcp
	ServerAddr string `json:"serverAddr"` // frps 地址
	ServerPort int    `json:"serverPort"` // frps 端口（默认 7000）
	Token      string `json:"token"`      // auth.token（可选）
	ServerName string `json:"serverName"` // stcp：提供方 [[proxies]] name
	SecretKey  string `json:"secretKey"`  // stcp：预共享密钥
	RemotePort int    `json:"remotePort"` // tcp：frps 暴露端口
	User       string `json:"user"`       // SSH 用户名（空默认 root）
	Path       string `json:"path"`       // 远程日志路径（直达 tail 用）
}

type Store struct {
	Conns []Conn `json:"connections"`
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

// LoadStore 全局单例：读失败/损坏 → 空 store；v1 文件自动迁移为 v2 并写回。
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
	// 先按 v1 探测：servers 键非空 / 连接带 server 引用 → 迁移；否则 v2 直读
	var v1 v1File
	if json.Unmarshal(data, &v1) == nil && v1.isV1() {
		storeData.Conns = v1.migrate()
		_ = storeData.saveLocked() // 迁移结果写回（失败静默，下次读再迁）；LoadStore 已持锁，不可调 Save（锁不可重入）
	} else {
		json.Unmarshal(data, storeData) // 损坏 → 保持空 store
	}
	// 自愈：丢弃 name 为空的脏记录（2026-11-02 前端传参错位事故的遗留产物）
	if n := len(storeData.Conns); n > 0 {
		out := storeData.Conns[:0]
		for _, c := range storeData.Conns {
			if strings.TrimSpace(c.Name) != "" {
				out = append(out, c)
			}
		}
		storeData.Conns = out
	}
	return storeData
}

func (s *Store) Save() error {
	storeMu.Lock()
	defer storeMu.Unlock()
	return s.saveLocked()
}

// saveLocked 落盘（调用方必须已持有 storeMu）。
func (s *Store) saveLocked() error {
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

// UpsertConn 按 Name 去重覆盖（确认日志文件时更新 Path 也走这里）；入参归一（type/serverPort 默认值）。
func (s *Store) UpsertConn(c Conn) {
	c = normalizeConn(c)
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

func (s *Store) FindConn(name string) (Conn, bool) {
	for _, c := range s.Conns {
		if c.Name == name {
			return c, true
		}
	}
	return Conn{}, false
}

// normalizeConn 入参归一：type 空 → stcp（存量手改 json 场景），serverPort 0 → 7000。
func normalizeConn(c Conn) Conn {
	if c.Type != TypeTCP {
		c.Type = TypeSTCP
	}
	if c.ServerPort == 0 {
		c.ServerPort = 7000
	}
	return c
}

// ---- v1 迁移（servers+connections 两层 → 平铺；仅 LoadStore 内部使用）----

type v1Server struct {
	Name  string `json:"name"`
	Addr  string `json:"addr"` // host:port
	Token string `json:"token"`
}

type v1Conn struct {
	Name   string `json:"name"`
	Server string `json:"server"` // 引用 v1Server.Name
	SK     string `json:"sk"`
	Proxy  string `json:"proxy"`
	User   string `json:"user"`
	Path   string `json:"path"`
}

type v1File struct {
	Servers []v1Server `json:"servers"`
	Conns   []v1Conn   `json:"connections"`
}

// isV1 v1 特征：存在 servers 记录，或任一连接带 server 引用。
func (f *v1File) isV1() bool {
	if len(f.Servers) > 0 {
		return true
	}
	for _, c := range f.Conns {
		if c.Server != "" {
			return true
		}
	}
	return false
}

// migrate v1 → v2：server.addr 拆 host/port、token 平铺、proxy/sk 改官方名，全部 stcp。
func (f *v1File) migrate() []Conn {
	srvBy := make(map[string]v1Server, len(f.Servers))
	for _, s := range f.Servers {
		srvBy[s.Name] = s
	}
	out := make([]Conn, 0, len(f.Conns))
	for _, c := range f.Conns {
		srv := srvBy[c.Server]
		host, port := splitHostPort(srv.Addr)
		out = append(out, normalizeConn(Conn{
			Name:       c.Name,
			Type:       TypeSTCP,
			ServerAddr: host,
			ServerPort: port,
			Token:      srv.Token,
			ServerName: c.Proxy,
			SecretKey:  c.SK,
			User:       c.User,
			Path:       c.Path,
		}))
	}
	return out
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
