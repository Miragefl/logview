package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/justfun/logview/internal/model"
)

type K8sResource struct {
	Kind string
	Name string
}

func ParseK8sResource(s string) (K8sResource, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[1] == "" {
		return K8sResource{}, fmt.Errorf("invalid resource format: %q (expected kind/name)", s)
	}
	kind := strings.ToLower(parts[0])
	switch kind {
	case "deploy", "deployment":
		kind = "deployment"
	case "sts", "statefulset":
		kind = "statefulset"
	case "po", "pod":
		kind = "pod"
	default:
		return K8sResource{}, fmt.Errorf("unsupported resource kind: %q", parts[0])
	}
	return K8sResource{Kind: kind, Name: parts[1]}, nil
}

type K8sSource struct {
	resource  K8sResource
	namespace string
	podNames  []string
	seq       atomic.Uint64
}

func NewK8sSource(resource, namespace string, podNames []string) *K8sSource {
	res, _ := ParseK8sResource(resource)
	return &K8sSource{resource: res, namespace: namespace, podNames: podNames}
}

func (k *K8sSource) Label() string {
	return fmt.Sprintf("k8s/%s/%s", k.resource.Kind, k.resource.Name)
}

func (k *K8sSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	pods := k.podNames
	var err error

	if len(pods) == 0 && k.resource.Kind != "pod" {
		pods, err = k.discoverPods(ctx)
		if err != nil {
			return nil, fmt.Errorf("discover pods: %w", err)
		}
	} else if k.resource.Kind == "pod" {
		pods = []string{k.resource.Name}
	}

	if len(pods) == 0 {
		return nil, fmt.Errorf("no pods found for %s", k.resource.Name)
	}

	ch := make(chan model.RawLine, 256)
	go func() {
		defer close(ch)
		var wg sync.WaitGroup
		for _, pod := range pods {
			wg.Add(1)
			go func(podName string) {
				defer wg.Done()
				k.streamPod(ctx, ch, podName)
			}(pod)
		}
		wg.Wait()
	}()

	return ch, nil
}

func (k *K8sSource) discoverPods(ctx context.Context) ([]string, error) {
	// get selector labels from the deployment/statefulset
	selectorArgs := []string{"get", k.resource.Kind, k.resource.Name,
		"-n", k.namespace,
		"-o", "jsonpath={.spec.selector.matchLabels}",
	}
	out, err := exec.CommandContext(ctx, "kubectl", selectorArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("get selector: %w", err)
	}
	var labels map[string]string
	if err := json.Unmarshal(out, &labels); err != nil || len(labels) == 0 {
		return nil, fmt.Errorf("parse selector labels: %w", err)
	}
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	selector := strings.Join(parts, ",")

	args := []string{"get", "pods",
		"-l", selector,
		"-n", k.namespace,
		"-o", "jsonpath={.items[*].metadata.name}",
	}
	out, err = exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, fmt.Errorf("no pods found for %s (selector: %s)", k.resource.Name, selector)
	}
	return strings.Fields(raw), nil
}

func (k *K8sSource) streamPod(ctx context.Context, ch chan<- model.RawLine, podName string) {
	args := []string{"logs", "-f", podName, "-n", k.namespace}
	cmd := exec.CommandContext(ctx, "kubectl", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer cmd.Wait()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := model.RawLine{
			Text:   scanner.Text(),
			Source: podName,
			Seq:    k.seq.Add(1),
		}
		select {
		case ch <- line:
		case <-ctx.Done():
			return
		}
	}
}

func (k *K8sSource) Cleanup() error { return nil }