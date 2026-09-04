package stream

import "testing"

// Scope() 返回源的词频隔离域:FRP=连接名、SSH=主机、k8s="k8s"、本地/管道=全局("")。
func TestScopeDomains(t *testing.T) {
	cases := []struct {
		name string
		src  LogStream
		want string
	}{
		{"frp 连接名", &FRPSource{name: "frp1"}, "frp1"},
		{"ssh 主机", &SSHSource{host: "srv1"}, "srv1"},
		{"k8s 统一域", &K8sSource{}, "k8s"},
		{"多 k8s 聚合", &MultiK8sSource{}, "k8s"},
		{"本地文件全局", &FileSource{}, ""},
		{"tail 全局", &TailSource{}, ""},
		{"管道全局", &PipeSource{}, ""},
	}
	for _, c := range cases {
		if got := c.src.Scope(); got != c.want {
			t.Errorf("%s: Scope() = %q, want %q", c.name, got, c.want)
		}
	}
}
