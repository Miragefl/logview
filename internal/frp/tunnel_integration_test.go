// internal/frp/tunnel_integration_test.go
//go:build frpintegration

package frp

// 真实 frpc 集成测试（需本机 frpc + 可达 frps）：
//   go test -tags frpintegration ./internal/frp/ -run TestStartTunnelReal -v
// 环境变量：FRPS_ADDR（host:port）、FRPS_TOKEN、FRP_SK、FRP_PROXY。
import (
	"os"
	"testing"
)

func TestStartTunnelReal(t *testing.T) {
	sv := Server{Addr: os.Getenv("FRPS_ADDR"), Token: os.Getenv("FRPS_TOKEN")}
	tun, err := StartTunnel(sv, os.Getenv("FRP_SK"), os.Getenv("FRP_PROXY"))
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Cleanup()
	if tun.LocalPort() <= 0 {
		t.Fatalf("端口非法: %d", tun.LocalPort())
	}
}
