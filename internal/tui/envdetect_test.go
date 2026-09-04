package tui

import (
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
