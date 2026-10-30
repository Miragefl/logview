package fview

import (
	"strings"
	"sync"
	"testing"

	"github.com/justfun/logview/internal/frp"
)

// TestFRPLookup 连接引用缺失服务器时返回含连接名的明确错误（Task 17 Step 1）。
func TestFRPLookup(t *testing.T) {
	st := &frp.Store{}
	_, _, err := FRPLookup(st, "edge-1")
	if err == nil || !strings.Contains(err.Error(), "edge-1") {
		t.Fatalf("连接不存在应报含连接名的错误，实际: %v", err)
	}
	st.UpsertConn(frp.Conn{Name: "edge-1", Server: "missing"})
	_, _, err = FRPLookup(st, "edge-1")
	if err == nil || !strings.Contains(err.Error(), "edge-1") {
		t.Fatalf("引用缺失服务器应报含连接名的错误，实际: %v", err)
	}
	st.UpsertServer(frp.Server{Name: "srv", Addr: "x:7000", Token: "t"})
	st.UpsertConn(frp.Conn{Name: "edge-1", Server: "srv", SK: "sk", Proxy: "p", User: "root"})
	conn, srv, err := FRPLookup(st, "edge-1")
	if err != nil || conn.Name != "edge-1" || srv.Addr != "x:7000" {
		t.Fatalf("正常查找: conn=%v srv=%v err=%v", conn, srv, err)
	}
}

// TestFRPBrowseHost FRP 浏览的 SSH 目标必须带连接用户名（Final Review C1：
// 丢 user@ 会以本地用户名连远端 sshd，认证死循环）。
func TestFRPBrowseHost(t *testing.T) {
	if got := FRPBrowseHost(frp.Conn{User: "root"}); got != "root@127.0.0.1" {
		t.Errorf("FRPBrowseHost = %q, 期望 root@127.0.0.1", got)
	}
	if got := FRPBrowseHost(frp.Conn{User: ""}); got != "root@127.0.0.1" {
		t.Errorf("空用户应兜底 root, 实际 %q", got)
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
