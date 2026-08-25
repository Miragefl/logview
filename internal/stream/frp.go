package stream

// FRPSource：frp stcp 隧道 + SSH tail。隧道由 TUI 侧建好后移交进来，
// Cleanup 时连带杀 frpc visitor。认证/错误上屏逻辑全部复用 SSHSource。

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

type FRPSource struct {
	tunnel frpTunnelHandle
	inner  *SSHSource
	name   string // 记录名（label 与密码缓存 key）
}

func NewFRPSource(name string, t frpTunnelHandle, user, path string, tailLines int) *FRPSource {
	inner := NewSSHSource(user+"@127.0.0.1", path, tailLines)
	inner.SetPort(t.LocalPort())
	return &FRPSource{tunnel: t, inner: inner, name: name}
}

func (s *FRPSource) Label() string { return fmt.Sprintf("frp://%s%s", s.name, s.inner.path) }

func (s *FRPSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	return s.inner.Start(ctx)
}

func (s *FRPSource) Cleanup() error {
	s.inner.Cleanup()
	return s.tunnel.Cleanup()
}

func (s *FRPSource) SetPassword(pw string) { s.inner.SetPassword(pw) }
func (s *FRPSource) Password() string      { return s.inner.Password() }
func (s *FRPSource) Host() string          { return "frp:" + s.name }
func (s *FRPSource) Path() string          { return s.inner.Path() }
