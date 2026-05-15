# LogView — 终端实时日志查看器

## 概述

一个 Go 语言编写的终端 TUI 工具，用于实时查看、搜索、过滤日志流。支持 k8s 多 Pod 聚合、本地文件 tail、管道输入三种数据源。通过自定义正则解析日志格式，支持字段级别的显示控制和两步筛选工作流。

## 架构

四层结构，单向数据流：

```
Log Stream (数据源) → Log Parser (正则解析) → Log Buffer (环形缓冲) → TUI Layer (渲染)
```

### Log Stream 接口

```go
type LogStream interface {
    Start(ctx context.Context) (<-chan RawLine, error)
    Label() string    // 显示用，如 "pod/parking-api-7d8f6"
    Close() error
}
```

三种内置实现：
- **K8sSource** — 执行 `kubectl logs -f`，支持 Deployment/StatefulSet/Pod，自动发现多 Pod 并聚合
- **TailSource** — `tail -f` 本地文件，支持多文件
- **PipeSource** — 从 stdin 读取

### 多 Pod 聚合

- k8s 模式下自动发现 Deployment/StatefulSet 下所有 Pod
- 每个 Pod 独立 goroutine 读日志，汇入同一 channel
- 每条 RawLine 携带来源标识（Pod 名）
- 按日志时间戳排序展示
- 来源标识列可通过字段控制隐藏

## 正则解析

### 配置文件

路径：`~/.logview/rules.yaml`

```yaml
rules:
  - name: java-logback
    pattern: '(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)'

  - name: json-log
    pattern: '(?P<raw>.*)'
    parse: json

  - name: plain-text
    pattern: '(?P<message>.*)'
```

### 匹配逻辑

- 启动时自动尝试所有规则，首条命中即使用
- 可通过 `--rule <name>` 参数手动指定
- 解析失败的原样显示，不丢弃日志
- 用命名捕获组提取字段，解析后的字段：time、level、thread、traceId、logger、message

## UI 设计

### 布局

```
┌─ LogView ─ pod/parking-api-7d8f6 [java-logback] ─ 12,847条 ──────────┐
│ 09:27:01 INFO  ==========deliveryComplete=true==========              │
│ 09:27:02 WARN  connection timeout, retrying...                       │
│ 09:27:02 ERROR java.lang.NullPointerException                        │
│           at com.ydcloud.smart.parking...                             │
│           at com.ydcloud.smart.parking...  (3 lines folded) [→展开]   │
│ 09:27:03 INFO  heartbeat ok                                          │
├───────────────────────────────────────────────────────────────────────┤
│ 搜索: deliveryComplete                                                │
├── 字段 ──┬── 级别 ──┬── 筛选 ────────────────────────────────────────┤
│ ☑ time   │ ☑ INFO  │ traceId: (空)                                   │
│ ☐ thread │ ☑ WARN  │ thread:  (空)                                   │
│ ☐ traceId│ ☑ ERROR │ 级别:    (空)                                    │
│ ☑ level  │ ☐ DEBUG │                                                 │
│ ☐ logger │         │                                                 │
│ ☑ message│         │                                                 │
├─────────┴─────────┴─────────────────────────────────────────────────┤
│ /搜索  Tab面板  Enter提取  f字段  e堆栈  s导出  g跳转  q退出          │
└──────────────────────────────────────────────────────────────────────┘
```

### 快捷键

| 按键 | 功能 |
|------|------|
| `/` | 进入搜索模式，实时模糊匹配 |
| `Tab` | 切换底部面板：字段显示 / 级别过滤 / 精准筛选 |
| `Enter` | 在日志行上按回车，自动提取当前行的 traceId/线程填入筛选框 |
| `f` | 切换字段显示面板，勾选/取消勾选 |
| `e` | 折叠/展开异常堆栈 |
| `s` | 导出当前过滤后的日志到文件 |
| `g` | 跳转到缓冲区顶部/底部 |
| `q` | 退出 |

### 两步筛选工作流

1. `/` 输入关键词搜索消息 → 高亮匹配行
2. 光标移到目标行 → `Enter` → 自动提取该行 traceId/线程填入筛选框 → 日志流按此精准过滤

### 异常堆栈折叠

- 解析时检测连续的 `at xxx.xxx(` 行或 `Caused by:` 行
- 渲染时合并为折叠块，显示 `(N lines folded)`
- 按 `e` 或点击展开

## 性能策略

| 策略 | 说明 |
|------|------|
| 虚拟滚动 | 只渲染可视区域日志行，不渲染屏幕外内容 |
| 环形缓冲区 | 固定容量默认 10 万条，`--buffer-size` 可配置，满则覆盖最旧 |
| 搜索索引 | 维护 keyword → []bufferIndex 的 map，入队时同步更新，O(1) 查找 |
| 渲染节流 | 30fps（33ms 刷新一次），日志再快也按此频率刷屏 |
| 回看暂停 | 用户上滚查看历史时暂停自动滚动，右下角提示 `[新日志: N条] ↓按跳转` |

## 日志导出

按 `s` 弹出导出面板：

- **范围**：当前筛选结果 / 全部缓冲区
- **格式**：原始文本 / 结构化 JSON（解析后字段）
- **路径**：默认 `./logview-export-{日期}.log`，可编辑

导出异步执行，不阻塞日志流，完成后底部提示条数和路径。

## 命令行用法

```bash
# k8s
logview k8s deploy/parking-api
logview k8s deploy/parking-api -n production
logview k8s pod/parking-api-7d8f6-x9k2j

# 本地文件
logview tail /var/log/app.log
logview tail /var/log/app.log /var/log/another.log

# 管道
kubectl logs -f deploy/parking-api | logview pipe

# 选项
logview k8s deploy/parking-api --rule java-logback --buffer-size 50000
```

## 技术栈

- Go 1.22+
- bubbletea — TUI 框架
- lipgloss — 样式
- bubbles — 通用组件（输入框、列表等）
- client-go — 可选，未来直接调 k8s API 替代 kubectl 包装
