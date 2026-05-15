# LogView

终端实时日志查看器，支持实时搜索、字段过滤、多Pod聚合。

## 安装

### Homebrew（推荐）

```bash
brew tap Miragefl/logview
brew install logview
```

### 手动编译

```bash
git clone https://github.com/Miragefl/logview.git
cd logview
go build -o logview .
```

## 使用

```bash
# Kubernetes
logview k8s deploy/parking-api
logview k8s deploy/parking-api -n production
logview k8s pod/billing-rule-59fd8b85cf-xnn24 -n parking-release

# 本地文件
logview tail /var/log/app.log

# 管道
kubectl logs -f deploy/parking-api | logview pipe
```

## 自定义解析规则

创建 `~/.logview/rules.yaml`：

```yaml
rules:
  - name: java-logback
    pattern: '(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)'
  - name: json-log
    pattern: '(?P<raw>.*)'
    parse: json
  - name: plain-text
    pattern: '(?P<message>.*)'

fields:
  - name: time
    visible: true
  - name: level
    visible: true
  - name: thread
    visible: false
  - name: traceId
    visible: false
  - name: logger
    visible: false
  - name: message
    visible: true
  - name: source
    visible: true
```

## 搜索语法

支持关键词、字段前缀、AND/OR 布尔运算、时间范围：

```
ERROR                           关键词搜索
traceId:42980fadf7bc48c8        按字段精确匹配
level:ERROR thread:exec-3       多字段 AND
ERROR OR WARN                   OR 匹配
after:09:00 before:10:00        时间范围过滤
after:09:00 ERROR AND WARN      混合使用
```

## 快捷键

| 按键 | 功能 |
|------|------|
| `/` | 搜索 |
| `n` / `N` | 下一个/上一个匹配 |
| `Esc` | 取消搜索（保留当前行） |
| `f` | 字段显示设置 |
| `s` | 导出日志 |
| `g` | 跳到顶部 |
| `G` | 跳到底部（自动滚动） |
| `↑` / `k` | 上移一行 |
| `↓` / `j` | 下移一行 |
| `C-u` | 上半页 |
| `C-d` | 下半页 |
| `C-b` | 整页上翻 |
| `C-f` | 整页下翻 |
| `q` | 退出 |
