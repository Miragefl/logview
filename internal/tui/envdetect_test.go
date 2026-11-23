package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// withProbes 覆盖环境探测变量(测试后恢复),避免真机 kubectl/ssh/frpc 有无影响测试结果。
func withProbes(t *testing.T, k8s, ssh, frp bool) {
	t.Helper()
	oldK8s, oldSSH, oldFRP := k8sProbe, sshProbe, frpProbe
	k8sProbe, sshProbe, frpProbe = func() bool { return k8s }, func() bool { return ssh }, func() bool { return frp }
	t.Cleanup(func() { k8sProbe, sshProbe, frpProbe = oldK8s, oldSSH, oldFRP })
}

func TestAvailableSourceTabsAll(t *testing.T) {
	withProbes(t, true, true, true)
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{0, 1, 2, 3}) {
		t.Fatalf("全可用环境可见集 = %v, want [0 1 2 3]", got)
	}
}

func TestAvailableSourceTabsNoKubectl(t *testing.T) {
	withProbes(t, false, true, true)
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("无 kubectl 可见集 = %v, want [1 2 3]", got)
	}
}

func TestAvailableSourceTabsNoSsh(t *testing.T) {
	withProbes(t, true, false, true)
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{0, 1, 3}) {
		t.Fatalf("无 ssh 客户端可见集 = %v, want [0 1 3]", got)
	}
}

func TestAvailableSourceTabsNoFrp(t *testing.T) {
	withProbes(t, true, true, false)
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("无 frpc 可见集 = %v, want [0 1 2]", got)
	}
}

func TestAvailableSourceTabsRemoteIgnored(t *testing.T) {
	// 能力驱动翻案:SSH 会话身份变量不再裁剪 tab,能力齐则全量。
	t.Setenv("SSH_CONNECTION", "10.0.0.1 1 10.0.0.2 22")
	t.Setenv("SSH_TTY", "/dev/pts/0")
	withProbes(t, true, true, true)
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{0, 1, 2, 3}) {
		t.Fatalf("SSH 会话内能力齐全可见集 = %v, want [0 1 2 3](远端身份不参与判定)", got)
	}
}

// kubeconfigExists 用例:HOME/KUBECONFIG 全部 t.Setenv 隔离(TestMain 的 HOME 是进程级共享目录),不碰真实环境。

func TestKubeconfigExistsDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KUBECONFIG", "")
	if err := os.MkdirAll(filepath.Join(home, ".kube"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".kube", "config")
	if err := os.WriteFile(cfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !kubeconfigExists() {
		t.Fatalf("存在 ~/.kube/config 时 kubeconfigExists = false, want true")
	}
	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}
	if kubeconfigExists() {
		t.Fatalf("~/.kube/config 已删后 kubeconfigExists = true, want false")
	}
}

func TestKubeconfigExistsMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KUBECONFIG", "")
	if kubeconfigExists() {
		t.Fatalf("无 kubeconfig 时 kubeconfigExists = true, want false")
	}
}

func TestKubeconfigExistsEnvHit(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // HOME 下不造 config,证明命中的是 KUBECONFIG
	kc := filepath.Join(t.TempDir(), "kc.yaml")
	if err := os.WriteFile(kc, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kc)
	if !kubeconfigExists() {
		t.Fatalf("KUBECONFIG 指向存在文件时 kubeconfigExists = false, want true")
	}
}

func TestKubeconfigExistsEnvMultiPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	kc := filepath.Join(t.TempDir(), "kc.yaml")
	if err := os.WriteFile(kc, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing.yaml")+":"+kc)
	if !kubeconfigExists() {
		t.Fatalf("KUBECONFIG 多路径任一存在时 kubeconfigExists = false, want true")
	}
}

func TestKubeconfigExistsEnvSkipsEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	kc := filepath.Join(t.TempDir(), "kc.yaml")
	if err := os.WriteFile(kc, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", "::"+kc)
	if !kubeconfigExists() {
		t.Fatalf("KUBECONFIG 空段跳过后 kubeconfigExists = false, want true")
	}
}

func TestKubeconfigExistsEnvMissNoFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".kube"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".kube", "config"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	if kubeconfigExists() {
		t.Fatalf("KUBECONFIG 设了但不存在时不应回落 ~/.kube/config: got true, want false")
	}
}

func TestKubeconfigExistsNoHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("KUBECONFIG", "")
	if kubeconfigExists() {
		t.Fatalf("HOME 不可解析时 kubeconfigExists = true, want false")
	}
}
