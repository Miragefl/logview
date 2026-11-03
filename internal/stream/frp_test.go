package stream

import (
	"context"
	"os"
	"testing"
	"time"
)

type fakeTunnel struct {
	port    int
	cleaned bool
}

func (f *fakeTunnel) LocalPort() int { return f.port }
func (f *fakeTunnel) Cleanup() error { f.cleaned = true; return nil }

// FRPDefaultUser：空用户兜底 root（FRP 场景约定）；非空原样。
// 回归（2026-11-02）：空 user 裸拼 user@host 产出 "@host" 非法目标，ssh 打 usage。
func TestFRPDefaultUser(t *testing.T) {
	if got := FRPDefaultUser(""); got != "root" {
		t.Errorf("空用户应兜底 root，实际 %q", got)
	}
	if got := FRPDefaultUser("ubuntu"); got != "ubuntu" {
		t.Errorf("非空用户应原样，实际 %q", got)
	}
}

// 两种形态的构造都必须兜底：inner host 不允许出现裸 "@host"。
func TestFRPSourceEmptyUserFallsBackRoot(t *testing.T) {
	s1 := NewFRPSource("a", &fakeTunnel{port: 6022}, "", "/x.log", 10)
	if got := s1.inner.host; got != "root@127.0.0.1" {
		t.Errorf("stcp 空 user 应拼 root@127.0.0.1，实际 %q", got)
	}
	s2 := NewFRPDirectSource("b", "frps.example.com", 6000, "", "/y.log", 10)
	if got := s2.inner.host; got != "root@frps.example.com" {
		t.Errorf("tcp 空 user 应拼 root@frps.example.com，实际 %q", got)
	}
}

func TestFRPSourceStreamsAndCleansTunnel(t *testing.T) {
	bin := fakeSSHScript(t, "frp tail line")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	tun := &fakeTunnel{port: 6022}
	src := NewFRPSource("client-a", tun, "root", "/var/log/a.log", 100)

	if got := src.Label(); got != "frp://client-a/var/log/a.log" {
		t.Fatalf("label 应为 frp://client-a/var/log/a.log，实际 %s", got)
	}
	if src.Host() != "frp:client-a" || src.Path() != "/var/log/a.log" {
		t.Fatalf("Host/Path 不符: %s %s", src.Host(), src.Path())
	}

	ch, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case batch, ok := <-ch:
		if !ok || len(batch) == 0 || batch[0].Text != "frp tail line" {
			t.Fatalf("应流出 frp tail line: %+v", batch)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}

	src.SetPassword("pw")
	if src.Password() != "pw" {
		t.Fatal("SetPassword 应委托 inner")
	}
	if err := src.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if !tun.cleaned {
		t.Fatal("Cleanup 应清理隧道")
	}
}
