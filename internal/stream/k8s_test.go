package stream

import (
	"testing"
)

func TestK8sSourceLabel(t *testing.T) {
	src := NewK8sSource("deploy/parking-api", "default", nil, 0)
	label := src.Label()
	if label != "k8s/deployment/parking-api" {
		t.Errorf("Label() = %q, want 'k8s/deployment/parking-api'", label)
	}
}

func TestParseK8sResource(t *testing.T) {
	tests := []struct {
		input   string
		want    K8sResource
		wantErr bool
	}{
		{"deploy/parking-api", K8sResource{Kind: "deployment", Name: "parking-api"}, false},
		{"pod/api-7d8f6-x9k2j", K8sResource{Kind: "pod", Name: "api-7d8f6-x9k2j"}, false},
		{"sts/data-store", K8sResource{Kind: "statefulset", Name: "data-store"}, false},
		{"invalid", K8sResource{}, true},
	}
	for _, tt := range tests {
		got, err := ParseK8sResource(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseK8sResource(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseK8sResource(%q) = %+v, want %+v", tt.input, got, tt.want)
		}
	}
}
// SetContext 后 kubectl 参数带 --context。
func TestK8sSourceContextArgs(t *testing.T) {
	k := NewK8sSource("deploy/x", "default", nil, 10)
	if got := k.kubectlArgs("get", "pods"); len(got) != 2 || got[0] != "get" {
		t.Fatalf("无 context 时参数不应变化: %v", got)
	}
	k.SetContext("uat-context")
	got := k.kubectlArgs("get", "pods")
	want := []string{"--context", "uat-context", "get", "pods"}
	if len(got) != len(want) {
		t.Fatalf("带 context 参数: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("参数 %d: %q want %q", i, got[i], want[i])
		}
	}
}
