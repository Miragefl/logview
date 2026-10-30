package fview

import (
	"strings"
	"testing"

	"github.com/justfun/logview/internal/frp"
)

// TestFRPBrowsePlan 连接引用缺失服务器时返回含连接名的明确错误（Task 17 Step 1）。
func TestFRPBrowsePlan(t *testing.T) {
	_, _, err := frpBrowsePlan(
		frp.Conn{Name: "edge-1", Server: "missing", SK: "sk", Proxy: "p", User: "root", Path: ""},
		map[string]frp.Server{},
	)
	if err == nil || !strings.Contains(err.Error(), "edge-1") {
		t.Fatalf("引用缺失服务器应报含连接名的错误，实际: %v", err)
	}

	host, port, err := frpBrowsePlan(
		frp.Conn{Name: "edge-1", Server: "srv", SK: "sk", Proxy: "p", User: "root", Path: ""},
		map[string]frp.Server{"srv": {Name: "srv", Addr: "x:7000", Token: "t"}},
	)
	if err != nil {
		t.Fatalf("正常计划: %v", err)
	}
	if host != "127.0.0.1" || port != 0 {
		t.Errorf("计划应指向本地隧道端口（port 由隧道实例注入），实际 %s:%d", host, port)
	}
}
