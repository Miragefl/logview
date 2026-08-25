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
	case l, ok := <-ch:
		if !ok || l.Text != "frp tail line" {
			t.Fatalf("应流出 frp tail line: %+v", l)
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
