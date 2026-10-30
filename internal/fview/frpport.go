// FRP 一键连接编排：AcquireTunnel（引用计数复用）→ 本地端口 SSH 浏览/tail。
// 浏览临时隧道未转化为 tab 时由 ReleaseFRPBrowse 显式回收（Review Focus 2），
// 进程退出 KillAllTunnels 兜底（main.go OnShutdown）。
package fview

import (
	"fmt"

	"github.com/justfun/logview/internal/frp"
	"github.com/justfun/logview/internal/stream"
)

// frpBrowsePlan 校验连接并给出浏览目标（host 恒 127.0.0.1，port 由隧道实例注入）。
func frpBrowsePlan(conn frp.Conn, servers map[string]frp.Server) (host string, port int, err error) {
	srv, ok := servers[conn.Server]
	if !ok {
		return "", 0, fmt.Errorf("连接 %s 引用的服务器 %s 不存在，先在连接管理中配置", conn.Name, conn.Server)
	}
	_ = srv // 端口来自 AcquireTunnel 返回的 Tunnel（测试经 port 参数注入）
	return "127.0.0.1", 0, nil
}

// BrowseFRPFRPSource 建隧道浏览的装配入口在 app.go（需 Manager/emit 上下文）；
// 此处仅提供纯计划逻辑与隧道释放。
func ReleaseTunnelIfIdle(t *frp.Tunnel) {
	if t != nil {
		_ = t.Cleanup()
	}
}

// openFRPSource 供 app.go 调用：AcquireTunnel + NewFRPSource 组装（user 默认 root）。
func openFRPSource(conn frp.Conn, server frp.Server, path string, tailLines int) (*stream.FRPSource, *frp.Tunnel, error) {
	if _, _, err := frpBrowsePlan(conn, map[string]frp.Server{server.Name: server}); err != nil {
		return nil, nil, err
	}
	t, err := frp.AcquireTunnel(server, conn.SK, conn.Proxy)
	if err != nil {
		return nil, nil, fmt.Errorf("建立隧道失败: %w", err)
	}
	src := stream.NewFRPSource(conn.Name, t, conn.User, path, tailLines)
	src.SetBufferLines(100000)
	return src, t, nil
}
