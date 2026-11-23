package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// withProbes 覆盖环境探测变量(测试后恢复),避免真机 kubectl 有无影响测试结果。
func withProbes(t *testing.T, k8s, remote bool) {
	t.Helper()
	oldK8s, oldRemote := k8sProbe, remoteProbe
	k8sProbe, remoteProbe = func() bool { return k8s }, func() bool { return remote }
	t.Cleanup(func() { k8sProbe, remoteProbe = oldK8s, oldRemote })
}

func TestAvailableSourceTabsAll(t *testing.T) {
	withProbes(t, true, false)
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{0, 1, 2, 3}) {
		t.Fatalf("全可用环境可见集 = %v, want [0 1 2 3]", got)
	}
}

func TestAvailableSourceTabsNoKubectl(t *testing.T) {
	withProbes(t, false, false)
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("无 kubectl 可见集 = %v, want [1 2 3]", got)
	}
}

func TestAvailableSourceTabsRemoteOverridesK8s(t *testing.T) {
	withProbes(t, true, true) // 远端优先:即使本机有 kubectl 也只留本地
	if got := availableSourceTabs(); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("远端会话可见集 = %v, want [1]", got)
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
