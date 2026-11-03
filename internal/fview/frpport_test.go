package fview

import (
	"strings"
	"sync"
	"testing"

	"github.com/justfun/logview/internal/frp"
)

// TestFRPLookup 连接不存在/必需字段缺失时返回含连接名的明确错误。
func TestFRPLookup(t *testing.T) {
	st := &frp.Store{}
	_, err := FRPLookup(st, "edge-1")
	if err == nil || !strings.Contains(err.Error(), "edge-1") {
		t.Fatalf("连接不存在应报含连接名的错误，实际: %v", err)
	}
	// stcp 缺 serverName/secretKey
	st.UpsertConn(frp.Conn{Name: "edge-1", Type: frp.TypeSTCP, ServerAddr: "x:7000"})
	_, err = FRPLookup(st, "edge-1")
	if err == nil || !strings.Contains(err.Error(), "serverName/secretKey") {
		t.Fatalf("stcp 缺 serverName/secretKey 应报错，实际: %v", err)
	}
	// tcp 缺 remotePort
	st.UpsertConn(frp.Conn{Name: "edge-2", Type: frp.TypeTCP, ServerAddr: "x:7000"})
	_, err = FRPLookup(st, "edge-2")
	if err == nil || !strings.Contains(err.Error(), "remotePort") {
		t.Fatalf("tcp 缺 remotePort 应报错，实际: %v", err)
	}
	// 缺 serverAddr
	st.UpsertConn(frp.Conn{Name: "edge-3", Type: frp.TypeTCP, RemotePort: 6000})
	_, err = FRPLookup(st, "edge-3")
	if err == nil || !strings.Contains(err.Error(), "serverAddr") {
		t.Fatalf("缺 serverAddr 应报错，实际: %v", err)
	}
	// 完整 tcp/stcp 通过
	st.UpsertConn(frp.Conn{Name: "ok-tcp", Type: frp.TypeTCP, ServerAddr: "x:7000", RemotePort: 6000})
	if _, err := FRPLookup(st, "ok-tcp"); err != nil {
		t.Fatalf("完整 tcp 应通过: %v", err)
	}
	st.UpsertConn(frp.Conn{Name: "ok-stcp", Type: frp.TypeSTCP, ServerAddr: "x:7000", ServerName: "p", SecretKey: "sk"})
	if _, err := FRPLookup(st, "ok-stcp"); err != nil {
		t.Fatalf("完整 stcp 应通过: %v", err)
	}
}

// TestFRPBrowseTarget FRP 浏览的 SSH 目标必须带连接用户名（Final Review C1：
// 丢 user@ 会以本地用户名连远端 sshd，认证死循环）；stcp 走隧道端口，tcp 直连 remotePort。
func TestFRPBrowseTarget(t *testing.T) {
	if host, port := FRPBrowseTarget(frp.Conn{Type: frp.TypeSTCP, User: "root"}, 6022); host != "root@127.0.0.1" || port != 6022 {
		t.Errorf("stcp 目标 = %s:%d, 期望 root@127.0.0.1:6022", host, port)
	}
	if host, _ := FRPBrowseTarget(frp.Conn{Type: frp.TypeSTCP}, 6022); host != "root@127.0.0.1" {
		t.Errorf("空用户应兜底 root, 实际 %q", host)
	}
	if host, port := FRPBrowseTarget(frp.Conn{Type: frp.TypeTCP, ServerAddr: "frps.example.com", RemotePort: 6000}, 0); host != "root@frps.example.com" || port != 6000 {
		t.Errorf("tcp 目标 = %s:%d, 期望 root@frps.example.com:6000", host, port)
	}
}

// TestBrowseRegistryConcurrent 浏览注册表并发 Get/Release 不 panic（Final Review C2）。
func TestBrowseRegistryConcurrent(t *testing.T) {
	r := NewBrowseRegistry[*fakeT]()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = r.Get("c1", func() (*fakeT, error) { return &fakeT{}, nil }) }()
		go func() { defer wg.Done(); r.Release("c1", func(t *fakeT) { t.cleaned = true }) }()
	}
	wg.Wait()
}

type fakeT struct {
	cleaned bool
}
