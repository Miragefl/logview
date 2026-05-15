# LogView

终端实时日志查看器，支持实时搜索、字段过滤、多Pod聚合。

## 安装

```bash
go build -o logview .
```

## 使用

```bash
# Kubernetes
logview k8s deploy/parking-api
logview k8s deploy/parking-api -n production

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
```

## 快捷键

| 按键 | 功能 |
|------|------|
| `/` | 搜索 |
| `Tab` | 切换面板 |
| `Enter` | 提取traceId/线程 |
| `f` | 字段显示控制 |
| `e` | 折叠/展开堆栈 |
| `s` | 导出 |
| `g` | 跳转顶/底 |
| `q` | 退出 |