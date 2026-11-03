package stream

// FRPSource：frp 访问形态的 SSH tail。两种形态：
//   - stcp：隧道由调用方建好后移交进来，Cleanup 时连带杀 frpc visitor
//   - tcp：frps remotePort 直连（零配置），无隧道，Cleanup 只清 inner
//
// 认证/错误上屏逻辑全部复用 SSHSource。

import (
	"context"
	"fmt"

	"github.com/justfun/logview/internal/model"
)

// frpTunnelHandle 隧道能力（*frp.Tunnel 实现；测试用 fake）。
type frpTunnelHandle interface {
	LocalPort() int
	Cleanup() error
}

// FRPDefaultUser FRP 场景的 SSH 用户约定：空默认 root（远端设备账号）。
// user@host 拼接必须先过它——空 user 裸拼会产出 "@host" 非法目标，
// ssh 打 usage（2026-11-02 空用户开流事故）；浏览/开流/TUI/fview 四路统一。
func FRPDefaultUser(user string) string {
	if user == "" {
		return "root"
	}
	return user
}

type FRPSource struct {
	tunnel frpTunnelHandle // tcp 直连形态为 nil
	inner  *SSHSource
	name   string // 记录名（label 与密码缓存 key）
}

func NewFRPSource(name string, t frpTunnelHandle, user, path string, tailLines int) *FRPSource {
	inner := NewSSHSource(FRPDefaultUser(user)+"@127.0.0.1", path, tailLines)
	inner.SetPort(t.LocalPort())
	return &FRPSource{tunnel: t, inner: inner, name: name}
}

// NewFRPDirectSource tcp 直连形态：frps remotePort 暴露，SSH 直连 serverAddr:remotePort，不拉 frpc。
func NewFRPDirectSource(name, serverAddr string, remotePort int, user, path string, tailLines int) *FRPSource {
	inner := NewSSHSource(FRPDefaultUser(user)+"@"+serverAddr, path, tailLines)
	inner.SetPort(remotePort)
	return &FRPSource{inner: inner, name: name}
}

func (s *FRPSource) Label() string { return fmt.Sprintf("frp://%s%s", s.name, s.inner.path) }

// Scope 词频隔离域=FRP 连接名(同连接多条路径共享词频历史)。
func (s *FRPSource) Scope() string { return s.name }

// SetBufferLines 转发 gz 归档传回行数上限(装配处传 ring 容量,与 SSH 直连同语义)。
func (s *FRPSource) SetBufferLines(n int) { s.inner.SetBufferLines(n) }

func (s *FRPSource) Start(ctx context.Context) (<-chan []model.RawLine, error) {
	return s.inner.Start(ctx)
}

func (s *FRPSource) Cleanup() error {
	s.inner.Cleanup()
	if s.tunnel != nil { // tcp 直连形态无隧道
		return s.tunnel.Cleanup()
	}
	return nil
}

func (s *FRPSource) SetPassword(pw string) { s.inner.SetPassword(pw) }
func (s *FRPSource) Password() string      { return s.inner.Password() }
func (s *FRPSource) Host() string          { return "frp:" + s.name }
func (s *FRPSource) Path() string          { return s.inner.Path() }
