package fview

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justfun/logview/internal/frp"
	"github.com/justfun/logview/internal/parser"
)

// TestUpdateSSHHosts ssh_hosts 写回共享 rules.yaml（Task 13 Step 1）。
func TestUpdateSSHHosts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	if err := UpdateSSHHosts(path, []string{"web-1", "db-1"}); err != nil {
		t.Fatalf("UpdateSSHHosts: %v", err)
	}
	res, err := parser.LoadRules(path)
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	if len(res.SSHHosts) != 2 || res.SSHHosts[0] != "web-1" {
		t.Errorf("写回后 ssh_hosts = %v", res.SSHHosts)
	}
	if len(res.Rules) == 0 {
		t.Error("其余键被破坏：默认规则丢失")
	}
}

// TestConnLibNoPasswordLeak 连接库序列化产物不含任何密码字段（Review Focus 3）。
func TestConnLibNoPasswordLeak(t *testing.T) {
	server := frp.Conn{Name: "s1", Type: frp.TypeSTCP, ServerAddr: "x", ServerPort: 7000, Token: "tok", ServerName: "p", SecretKey: "sk"}
	conn := frp.Conn{Name: "c1", Type: frp.TypeSTCP, ServerAddr: "x", ServerPort: 7000, Token: "tok", ServerName: "p", SecretKey: "sk", User: "root", Path: "/var/log/a.log"}
	for name, v := range map[string]any{"server": server, "conn": conn} {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s marshal: %v", name, err)
		}
		lower := strings.ToLower(string(data))
		if strings.Contains(lower, "password") || strings.Contains(lower, "passwd") {
			t.Errorf("%s 序列化产物含密码键: %s", name, lower)
		}
	}
}
