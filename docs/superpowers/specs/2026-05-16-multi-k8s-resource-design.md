# Multi-K8s-Resource 日志聚合

## 目标

支持 `logview k8s` 同时查看多个 deploy/pod 的日志，混合显示，用 `source:` 搜索语法过滤来源。

## CLI 语法

```
# 同 namespace，共享 -n
logview k8s -n parking deploy/api deploy/billing

# 跨 namespace，-n 按顺序配对
logview k8s -n parking deploy/api -n billing deploy/billing

# 单个资源，向后兼容
logview k8s deploy/api
logview k8s deploy/api -n parking
```

### -n 配对规则

| -n 数量 | 资源数量 | 行为 |
|---------|---------|------|
| 0 | 1 | 默认 namespace "default" |
| 1 | N | 所有资源共享该 namespace |
| N | N | 按顺序 1:1 配对 |
| N | M (N≠M, N>1) | 报错：namespace 数量与资源数量不匹配 |

### CLI 参数改动

- `k8sCmd.Args`：从 `cobra.ExactArgs(1)` 改为 `cobra.MinimumNArgs(1)`
- `-n` flag：从 `StringP` 改为 `StringArrayP`，支持多次指定
- `ValidArgsFunction`：支持补全所有资源参数，根据当前参数位置取对应的 namespace 查询 k8s 资源名

### Shell Completion 改动

`completeK8sResource` 不再限制 `len(args) > 0` 时返回空，而是为每个位置的资源参数都提供补全：

1. 已输入的资源参数个数 = `len(args)`，当前补全的是第 `len(args)` 个资源
2. 从 `-n` flag 取 namespace 列表：
   - 0 个 `-n`：当前资源用默认 namespace "default"
   - 1 个 `-n`：当前资源用该 namespace
   - 多个 `-n`：当前资源用 `nsList[len(args)]`（按索引配对）
3. 用对应 namespace 查询 k8s 资源名，返回补全列表

补全效果：
```
logview k8s -n parking deploy/<tab>       # 补全 parking ns 下的 deployment
logview k8s -n parking deploy/api <tab>   # 继续补全，仍用 parking ns
logview k8s -n parking deploy/api -n billing deploy/<tab>  # 用 billing ns 补全
```

## Stream 层：MultiK8sSource

新增 `MultiK8sSource` struct，实现 `LogStream` 接口。

```go
type MultiK8sSource struct {
    sources []*K8sSource
}
```

### Start(ctx) 逻辑

1. 为每个 `K8sSource` 调用 `Start(ctx)`，各返回一个 `<-chan RawLine`
2. 启动一个合并 goroutine，用 `reflect.Select` 或 N 个 goroutine 把所有 channel 的数据汇入一个输出 channel
3. 输出 channel 容量 512（资源多了需要更大缓冲）
4. 每条 `RawLine` 的 `Source` 字段已经是 pod 名，无需额外处理

### Label() 逻辑

多个资源用 `+` 连接：`k8s/deployment/api+deployment/billing`

### Cleanup() 逻辑

遍历调用每个 source 的 Cleanup()。

## TUI 层

**不改。** 现有架构天然支持：

- `Source` 字段标识每条日志来自哪个 pod，渲染为 `[pod-name]`
- `source:xxx` 搜索语法过滤指定 pod 的日志
- Title 栏显示 MultiK8sSource 的 Label

## 文件改动

| 文件 | 改动 |
|------|------|
| `cmd/root.go` | k8sCmd.Args 改 MinimumNArgs(1)，-n 改 StringArrayP，RunE 构造 MultiK8sSource 或单 K8sSource，completeK8sResource 支持多资源补全 |
| `internal/stream/k8s.go` | 新增 MultiK8sSource struct 及 Start/Label/Cleanup 方法 |
| `README.md` | 补充多资源用法示例 |

## 不做的事

- 不做分组/分栏显示（混合就够了）
- 不做按资源快捷键切换（source: 搜索语法过滤）
- 不做跨 ns 的 inline 语法（ns/deploy/name）
- 不做跨 ns 的 inline 语法（ns/deploy/name）
