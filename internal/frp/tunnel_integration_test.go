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
	host, port := SplitServerAddr(os.Getenv("FRPS_ADDR"))
	c := Conn{Type: TypeSTCP, ServerAddr: host, ServerPort: port,
		Token: os.Getenv("FRPS_TOKEN"), ServerName: os.Getenv("FRP_PROXY"), SecretKey: os.Getenv("FRP_SK")}
	tun, err := StartTunnel(c)
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Cleanup()
	if tun.LocalPort() <= 0 {
		t.Fatalf("端口非法: %d", tun.LocalPort())
	}
}
