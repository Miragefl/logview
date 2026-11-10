// 共享 rules.yaml 读改写（spec 4.5 共享正本：fview 写回，TUI 下次启动生效）。
// 写回策略：yaml.Node 局部替换目标键，保留其余结构与注释；
// 两 App 同开的并发写容忍 last-write-wins（整体读改写，无文件锁）。
package fview

import (
	"fmt"
	"os"

	"github.com/Miragefl/logview/internal/parser"
	"gopkg.in/yaml.v3"
)

// ensureRulesFile rules.yaml 不存在时落盘内置默认（纯 fview 新用户开箱即用，Final Review #11）。
func ensureRulesFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(parser.DefaultRulesYAML()), 0644)
}

// updateRulesKey 读改写 rules.yaml 顶层键：命中则替换值节点（保留原 key 节点注释），
// 未命中则追加；其余键原样保留。
func updateRulesKey(path, key string, value any) error {
	if err := ensureRulesFile(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析 %s: %w", path, err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%s 非映射结构", path)
	}
	mapping := root.Content[0]

	valNode := yaml.Node{}
	if err := valNode.Encode(value); err != nil {
		return err
	}

	replaced := false
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = &valNode
			replaced = true
			break
		}
	}
	if !replaced {
		keyNode := yaml.Node{Kind: yaml.ScalarNode, Value: key}
		mapping.Content = append(mapping.Content, &keyNode, &valNode)
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// UpdateHides 写回共享 rules.yaml 的 hides 键。
func UpdateHides(path string, hides []string) error {
	return updateRulesKey(path, "hides", hides)
}

// UpdateFields 写回共享 rules.yaml 的 fields 键（字段显隐配置）。
func UpdateFields(path string, fields []parser.FieldConfig) error {
	return updateRulesKey(path, "fields", fields)
}

// UpdateRules 写回共享 rules.yaml 的解析规则列表（正则配置 UI 持久化）。
func UpdateRules(path string, rules []parser.RuleConfig) error {
	return updateRulesKey(path, "rules", rules)
}

// UpdatePatterns 写回共享 rules.yaml 的 patterns 键（字段正则片段，规则 {name} 引用）。
func UpdatePatterns(path string, patterns map[string]string) error {
	return updateRulesKey(path, "patterns", patterns)
}
