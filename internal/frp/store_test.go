package frp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundtripAndUpsert(t *testing.T) {
	p := filepath.Join(t.TempDir(), "frp.json")
	SetStoreFileForTest(p)
	defer ResetStoreForTest()

	st := LoadStore()
	st.UpsertConn(Conn{Name: "a", Type: TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7000, ServerName: "ssh-a", SecretKey: "sk1", User: "root", Path: "/var/log/a.log"})
	st.UpsertConn(Conn{Name: "a", ServerAddr: "frps2.example.com"}) // 覆盖同名（type 空归一为 stcp）
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	ResetStoreForTest()
	SetStoreFileForTest(p)
	st2 := LoadStore()
	if len(st2.Conns) != 1 {
		t.Fatalf("同名覆盖应只有 1 条，实际 %d", len(st2.Conns))
	}
	c, ok := st2.FindConn("a")
	if !ok || c.ServerAddr != "frps2.example.com" || c.Type != TypeSTCP || c.ServerPort != 7000 {
		t.Fatalf("FindConn 应取回归一后的记录，实际 %+v ok=%v", c, ok)
	}
	if _, ok := st2.FindConn("nope"); ok {
		t.Fatal("不存在的记录应返回 false")
	}
}

func TestStoreDelete(t *testing.T) {
	p := filepath.Join(t.TempDir(), "frp.json")
	SetStoreFileForTest(p)
	defer ResetStoreForTest()

	st := LoadStore()
	st.UpsertConn(Conn{Name: "a", ServerName: "ssh-a"})
	st.UpsertConn(Conn{Name: "b", ServerName: "ssh-b"})
	if !st.DeleteConn("a") {
		t.Fatal("DeleteConn 应返回 true")
	}
	if _, ok := st.FindConn("a"); ok {
		t.Fatal("a 应已删除")
	}
	if st.DeleteConn("a") {
		t.Fatal("重复删除应返回 false")
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	ResetStoreForTest()
	SetStoreFileForTest(p)
	st2 := LoadStore()
	if len(st2.Conns) != 1 || st2.Conns[0].Name != "b" {
		t.Fatalf("删除后落盘应只剩 b，实际 %+v", st2.Conns)
	}
}

func TestStoreCorruptFileDegradesToEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "frp.json")
	SetStoreFileForTest(p)
	defer ResetStoreForTest()
	if err := os.WriteFile(p, []byte("not json {"), 0644); err != nil {
		t.Fatal(err)
	}
	st := LoadStore()
	if len(st.Conns) != 0 {
		t.Fatalf("损坏文件应降级为空 store，实际 %+v", st)
	}
}

// LoadStore 自愈：name 为空的脏记录（前端传参错位事故产物）读时丢弃。
func TestStoreLoadDropsEmptyName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "frp.json")
	dirty := `{"connections":[{"name":"","type":"stcp","serverPort":7000},{"name":"ok","type":"stcp","serverAddr":"x"}]}`
	if err := os.WriteFile(p, []byte(dirty), 0644); err != nil {
		t.Fatal(err)
	}
	SetStoreFileForTest(p)
	defer ResetStoreForTest()
	st := LoadStore()
	if len(st.Conns) != 1 || st.Conns[0].Name != "ok" {
		t.Fatalf("空 name 脏记录应被丢弃，实际 %+v", st.Conns)
	}
}

// v1（servers+connections 两层）读时自动迁移为 v2 平铺并写回。
func TestStoreMigrateV1(t *testing.T) {
	p := filepath.Join(t.TempDir(), "frp.json")
	v1 := `{
  "servers": [
    {"name": "prod", "addr": "frps.example.com:7000", "token": "tk"},
    {"name": "home", "addr": "nas.example.com", "token": "tk2"}
  ],
  "connections": [
    {"name": "a", "server": "prod", "sk": "sk1", "proxy": "ssh-a", "user": "root", "path": "/var/log/a.log"},
    {"name": "b", "server": "home", "sk": "sk2", "proxy": "ssh-b", "user": "", "path": ""}
  ]
}`
	if err := os.WriteFile(p, []byte(v1), 0644); err != nil {
		t.Fatal(err)
	}
	SetStoreFileForTest(p)
	defer ResetStoreForTest()

	st := LoadStore()
	if len(st.Conns) != 2 {
		t.Fatalf("迁移后应 2 条连接，实际 %d", len(st.Conns))
	}
	c, _ := st.FindConn("a")
	if c.Type != TypeSTCP || c.ServerAddr != "frps.example.com" || c.ServerPort != 7000 ||
		c.Token != "tk" || c.ServerName != "ssh-a" || c.SecretKey != "sk1" || c.Path != "/var/log/a.log" {
		t.Fatalf("a 迁移字段错误: %+v", c)
	}
	c2, _ := st.FindConn("b")
	if c2.ServerAddr != "nas.example.com" || c2.ServerPort != 7000 { // 无端口地址默认 7000
		t.Fatalf("b 迁移端口归一错误: %+v", c2)
	}

	// 迁移结果已写回 v2：重读走 v2 直读路径且字段一致
	ResetStoreForTest()
	SetStoreFileForTest(p)
	st2 := LoadStore()
	if len(st2.Conns) != 2 {
		t.Fatalf("写回后重读应保持 2 条，实际 %d", len(st2.Conns))
	}
	if c3, _ := st2.FindConn("a"); c3 != c {
		t.Fatalf("写回重读应与迁移结果一致: %+v vs %+v", c3, c)
	}
}
