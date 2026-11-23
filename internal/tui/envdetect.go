package tui

import (
	"os"
	"os/exec"
	"path/filepath"
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

// kubeconfigExists KUBECONFIG(冒号分隔多路径)非空则任一路径存在即可——kubectl 语义:
// 设了 KUBECONFIG 就不用默认文件,全不存在不回落;未设才查 ~/.kube/config。
// HOME 不可解析兜底 false。
func kubeconfigExists() bool {
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		for _, p := range filepath.SplitList(kc) {
			if p == "" {
				continue
			}
			if _, err := os.Stat(p); err == nil {
				return true
			}
		}
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".kube", "config"))
	return err == nil
}

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
