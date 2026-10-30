// k8s 浏览：exec kubectl 列 context/namespace/资源（--context 参数不动全局 kubeconfig）。
// 解析纯函数与命令执行分离——解析可 TDD，执行依赖本机 kubectl（手动验收覆盖）。
package fview

import (
	"os/exec"
	"strings"
)

// ResourceDTO 资源条目（Kind 归一化：deployment.apps → deployment）。
type ResourceDTO struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// ParseContexts 解析 `kubectl config get-contexts` 输出（跳过表头与警告行）。
func ParseContexts(output string) []string {
	var out []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "CURRENT") || strings.HasPrefix(line, "warning") {
			continue
		}
		// 去掉 CURRENT 列的 "*" 标记后取第一列
		fields := strings.Fields(strings.TrimPrefix(line, "*"))
		if len(fields) > 0 {
			out = append(out, fields[0])
		}
	}
	return out
}

// ParseNamespaces 解析 `kubectl get ns` 输出（跳过表头，取 NAME 列）。
func ParseNamespaces(output string) []string {
	var out []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "NAME") || strings.HasPrefix(line, "warning") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			out = append(out, fields[0])
		}
	}
	return out
}

// ParseResources 解析 `kubectl get deploy,sts,ds,po -o name` 输出（kind/name 拆分）。
func ParseResources(output string) []ResourceDTO {
	var out []ResourceDTO
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "No resources") || strings.HasPrefix(line, "warning") {
			continue
		}
		parts := strings.SplitN(line, "/", 2)
		if len(parts) != 2 {
			continue
		}
		kind := parts[0]
		if idx := strings.Index(kind, "."); idx > 0 {
			kind = kind[:idx] // deployment.apps → deployment
		}
		out = append(out, ResourceDTO{Kind: kind, Name: parts[1]})
	}
	return out
}

// runKubectl 执行 kubectl 并返回输出（--context 不改全局 kubeconfig）。
func runKubectl(kubeCtx string, args ...string) (string, error) {
	full := args
	if kubeCtx != "" {
		full = append([]string{"--context", kubeCtx}, args...)
	}
	// CombinedOutput：kubectl 失败时 stderr 携带真因（context 失效/集群不通/RBAC），
	// 解析函数的 warning/表头容错可消化混入行（Final Review I5）
	out, err := exec.Command("kubectl", full...).CombinedOutput()
	return string(out), err
}

// K8sContexts 列出全部 kubeconfig context。
func K8sContexts() ([]string, error) {
	out, err := runKubectl("", "config", "get-contexts")
	if err != nil {
		return nil, err
	}
	return ParseContexts(out), nil
}

// K8sNamespaces 列出指定 context 的 namespace。
func K8sNamespaces(kubeCtx string) ([]string, error) {
	out, err := runKubectl(kubeCtx, "get", "ns")
	if err != nil {
		return nil, err
	}
	return ParseNamespaces(out), nil
}

// K8sResources 列出指定 namespace 的工作负载资源（deployments/statefulsets/pods——
// 对齐 ParseK8sResource 支持面，daemonset 暂不支持）。
func K8sResources(kubeCtx, namespace string) ([]ResourceDTO, error) {
	out, err := runKubectl(kubeCtx, "-n", namespace, "get", "deployments,statefulsets,pods", "-o", "name")
	if err != nil {
		return nil, err
	}
	return ParseResources(out), nil
}
