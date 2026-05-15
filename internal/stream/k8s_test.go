package stream

import (
	"testing"
)

func TestK8sSourceLabel(t *testing.T) {
	src := NewK8sSource("deploy/parking-api", "default", nil)
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