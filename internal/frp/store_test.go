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
	st.UpsertServer(Server{Name: "prod", Addr: "frps.example.com:7000", Token: "tk"})
	st.UpsertServer(Server{Name: "prod", Addr: "frps2.example.com:7000", Token: "tk2"}) // 覆盖同名
	st.UpsertConn(Conn{Name: "a", Server: "prod", SK: "sk1", Proxy: "ssh-a", User: "root", Path: "/var/log/a.log"})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	ResetStoreForTest()
	SetStoreFileForTest(p)
	st2 := LoadStore()
	if len(st2.Servers) != 1 || st2.Servers[0].Addr != "frps2.example.com:7000" {
		t.Fatalf("UpsertServer 应按 Name 覆盖，实际 %+v", st2.Servers)
	}
	c, ok := st2.FindConn("a")
	if !ok || c.Proxy != "ssh-a" || c.Path != "/var/log/a.log" {
		t.Fatalf("FindConn 应取回记录，实际 %+v ok=%v", c, ok)
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
	st.UpsertServer(Server{Name: "prod", Addr: "frps.example.com:7000"})
	st.UpsertConn(Conn{Name: "a", Server: "prod", Proxy: "ssh-a"})
	st.UpsertConn(Conn{Name: "b", Server: "prod", Proxy: "ssh-b"})

	// 服务器被引用时不可删
	if err := st.DeleteServer("prod"); err == nil {
		t.Fatal("被引用的服务器删除应报错")
	}
	// 删除全部引用后可删
	if !st.DeleteConn("a") || !st.DeleteConn("b") {
		t.Fatal("DeleteConn 应返回 true")
	}
	if _, ok := st.FindConn("a"); ok {
		t.Fatal("a 应已删除")
	}
	if st.DeleteConn("a") {
		t.Fatal("重复删除应返回 false")
	}
	if err := st.DeleteServer("prod"); err != nil {
		t.Fatalf("无引用时删除服务器应成功: %v", err)
	}
	if err := st.DeleteServer("prod"); err == nil {
		t.Fatal("删除不存在的服务器应报错")
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	st2 := LoadStore()
	if len(st2.Conns) != 0 || len(st2.Servers) != 0 {
		t.Fatalf("删除后落盘应为空，实际 conns=%d servers=%d", len(st2.Conns), len(st2.Servers))
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
	if len(st.Servers) != 0 || len(st.Conns) != 0 {
		t.Fatalf("损坏文件应降级为空 store，实际 %+v", st)
	}
}
