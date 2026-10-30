package fview

import "testing"

// TestParseContexts kubectl config get-contexts 输出解析（Task 18；容错表头与警告行）。
func TestParseContexts(t *testing.T) {
	out := `CURRENT   NAME            CLUSTER        AUTHINFO
*          prod-cn         prod-cn        admin
           dev-local       dev-local      dev
warning: this is noise`
	got := ParseContexts(out)
	if len(got) != 2 || got[0] != "prod-cn" || got[1] != "dev-local" {
		t.Errorf("ParseContexts = %v", got)
	}
	if g := ParseContexts(""); len(g) != 0 {
		t.Errorf("空输出应返回空, 实际 %v", g)
	}
}

// TestParseNamespaces kubectl get ns 输出解析。
func TestParseNamespaces(t *testing.T) {
	out := `NAME              STATUS   AGE
default           Active   30d
kube-system       Active   30d
prod              Active   12d`
	got := ParseNamespaces(out)
	if len(got) != 3 || got[0] != "default" || got[2] != "prod" {
		t.Errorf("ParseNamespaces = %v", got)
	}
}

// TestParseResources kubectl get -o name 输出解析（kind/name 拆分）。
func TestParseResources(t *testing.T) {
	out := `deployment.apps/api-gateway
deployment.apps/order-service
statefulset.apps/mysql
pod/api-7d8f6-x8k2j`
	got := ParseResources(out)
	if len(got) != 4 {
		t.Fatalf("ParseResources 数量 = %d", len(got))
	}
	if got[0].Kind != "deployment" || got[0].Name != "api-gateway" {
		t.Errorf("首条 = %+v", got[0])
	}
	if got[3].Kind != "pod" || got[3].Name != "api-7d8f6-x8k2j" {
		t.Errorf("末条 = %+v", got[3])
	}
	if g := ParseResources("No resources found"); len(g) != 0 {
		t.Errorf("空资源提示应返回空, 实际 %v", g)
	}
}
