package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// 源选择器环境探测(包级函数变量,测试可注入;沿用 startFRPTunnel 等注入惯例)。
// tab 全局索引:0=K8s 1=本地 2=SSH 3=FRP。
// 可见性 = 纯能力探测,探测口径与执行层一比一(ssh: stream 层 exec 系统 ssh;
// frp: tunnel.go 拉起 frpc 前的 LookPath;k8s: kubectl+kubeconfig 双检)。
// 远端会话身份(SSH_TTY/SSH_CONNECTION)不参与判定——SSH 进服务器照样可看本机 K8s/SSH 跳板源。
var (
	// k8sProbe 本机是否可用 k8s:kubectl 在 PATH 且 kubeconfig 存在。
	k8sProbe = func() bool {
		if _, err := exec.LookPath("kubectl"); err != nil {
			return false
		}
		return kubeconfigExists()
	}
	// sshProbe 本机是否有 ssh 客户端。
	sshProbe = func() bool {
		_, err := exec.LookPath("ssh")
		return err == nil
	}
	// frpProbe 本机是否可建 FRP 隧道(frpc 缺席时砍 tab,不留点了就报错的死入口)。
	frpProbe = func() bool {
		_, err := exec.LookPath("frpc")
		return err == nil
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

// availableSourceTabs 能力驱动的源选择器可见 tab 集(全局索引,升序)。
// 本地恒在;kubectl+kubeconfig 有 K8s;ssh 客户端有 SSH;frpc 有 FRP。
func availableSourceTabs() []int {
	tabs := []int{1}
	if k8sProbe() {
		tabs = append(tabs, 0)
	}
	if sshProbe() {
		tabs = append(tabs, 2)
	}
	if frpProbe() {
		tabs = append(tabs, 3)
	}
	sort.Ints(tabs)
	return tabs
}
