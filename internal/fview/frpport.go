// FRP 一键连接编排支持：连接校验、浏览目标拼接、幂等隧道注册表。
// 引用计数语义（frp.Tunnel）：Acquire +1 / Cleanup -1，归零杀 frpc——
// 浏览侧经 BrowseRegistry 幂等持有一份，tab 侧由 FRPSource 自持（Final Review C1-C3）。
// tcp 直连（零配置，不拉 frpc）不走注册表/隧道。
package fview

import (
	"fmt"
	"sync"

	"github.com/Miragefl/logview/internal/frp"
	"github.com/Miragefl/logview/internal/stream"
)

// FRPBrowseTarget FRP 浏览的 SSH 目标与端口：必须带连接用户名（丢 user@ 会以本地
// 用户名连远端 sshd，认证死循环——Final Review C1）；空用户兜底 root（FRPDefaultUser 统一约定）。
// stcp 经隧道 127.0.0.1:localPort；tcp 直连 serverAddr:remotePort。
func FRPBrowseTarget(conn frp.Conn, localPort int) (host string, port int) {
	user := stream.FRPDefaultUser(conn.User)
	if conn.Type == frp.TypeTCP {
		return user + "@" + conn.ServerAddr, conn.RemotePort
	}
	return user + "@127.0.0.1", localPort
}

// FRPLookup 取连接并按类型校验必需字段（BrowseFRP/OpenFRPTab 公共段）。
func FRPLookup(st *frp.Store, connName string) (frp.Conn, error) {
	conn, ok := st.FindConn(connName)
	if !ok {
		return frp.Conn{}, fmt.Errorf("连接 %s 不存在", connName)
	}
	if conn.Type == frp.TypeTCP {
		if conn.RemotePort <= 0 {
			return frp.Conn{}, fmt.Errorf("连接 %s 缺少 remotePort", connName)
		}
	} else {
		if conn.ServerName == "" || conn.SecretKey == "" {
			return frp.Conn{}, fmt.Errorf("连接 %s 缺少 serverName/secretKey（stcp 必填）", connName)
		}
	}
	if conn.ServerAddr == "" {
		return frp.Conn{}, fmt.Errorf("连接 %s 缺少 serverAddr", connName)
	}
	return conn, nil
}

// BrowseRegistry 浏览会话的隧道注册表：锁保护并发（Final Review C2），
// 幂等持有——同连接重复浏览复用已有引用，不重复 Acquire（Final Review C3）。
type BrowseRegistry[T any] struct {
	mu      sync.Mutex
	tunnels map[string]T
}

// NewBrowseRegistry 构造空注册表。
func NewBrowseRegistry[T any]() *BrowseRegistry[T] {
	return &BrowseRegistry[T]{tunnels: make(map[string]T)}
}

// Get 幂等取引用：已有则复用；无则 acquire 新建并登记（失败不登记）。
func (r *BrowseRegistry[T]) Get(connName string, acquire func() (T, error)) (T, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tunnels[connName]; ok {
		return t, nil
	}
	t, err := acquire()
	if err != nil {
		var zero T
		return zero, err
	}
	r.tunnels[connName] = t
	return t, nil
}

// Release 释放并移除引用（cleanup 在锁外执行）。
func (r *BrowseRegistry[T]) Release(connName string, cleanup func(T)) {
	r.mu.Lock()
	t, ok := r.tunnels[connName]
	if ok {
		delete(r.tunnels, connName)
	}
	r.mu.Unlock()
	if ok && cleanup != nil {
		cleanup(t)
	}
}
