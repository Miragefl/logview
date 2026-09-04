package tui

import (
	"os"
	"os/exec"
)

// 源选择器环境探测(包级函数变量,测试可注入;沿用 startFRPTunnel 等注入惯例)。
// tab 全局索引:0=K8s 1=本地 2=SSH 3=FRP。
var (
	// k8sProbe 本机是否可用 k8s(kubectl 在 PATH 即视为可用)。
	k8sProbe = func() bool {
		_, err := exec.LookPath("kubectl")
		return err == nil
	}
	// remoteProbe 是否处于 ssh/frp 进入的远端会话(隧道本质也是 ssh,SSH_TTY 天然覆盖)。
	remoteProbe = func() bool {
		return os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != ""
	}
)

// availableSourceTabs 环境感知的源选择器可见 tab 集(全局索引,升序)。
// 远端会话只留本地;本机无 kubectl 去 K8s;其余全量。
func availableSourceTabs() []int {
	if remoteProbe() {
		return []int{1}
	}
	if !k8sProbe() {
		return []int{1, 2, 3}
	}
	return []int{0, 1, 2, 3}
}
