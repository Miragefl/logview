# LogView

终端实时日志查看器，支持实时搜索、高亮、字段过滤、多资源聚合。

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
# Kubernetes - 单个资源
logview k8s deploy/parking-api
logview k8s deploy/parking-api -n production
logview k8s pod/billing-rule-59fd8b85cf-xnn24 -n parking-release

# Kubernetes - 多个资源（同 namespace）
logview k8s -n parking deploy/api deploy/billing

# Kubernetes - 多个资源（跨 namespace）
logview k8s -n parking deploy/api -n billing deploy/billing-rule

# 本地文件
logview tail /var/log/app.log

# 管道
kubectl logs -f deploy/parking-api | logview pipe

# 查看版本
logview version

# 指定配置文件目录
logview --config ~/.config/logview k8s deploy/parking-api
```

## 配置

默认配置目录：`~/.config/logview/`

可通过 `--config` 指定其他目录：

```bash
logview --config /path/to/config k8s deploy/app
```

配置文件 `rules.yaml`：

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

## 命令补全

支持 bash / zsh / fish，安装后输入命令按 Tab 自动补全子命令、k8s 资源和 namespace。

```bash
# zsh（推荐加入 ~/.zshrc）
logview completion zsh > ~/.zfunc/_logview

# bash
logview completion bash > /etc/bash_completion.d/logview

# fish
logview completion fish > ~/.config/fish/completions/logview.fish
```

补全效果：

```
logview <tab>                     # 提示子命令: k8s, tail, pipe, version, completion
logview k8s -n <tab>              # 提示集群中的 namespace
logview k8s <tab>                 # 提示资源类型: pod/, deploy/, sts/
logview k8s pod/<tab>             # 提示该 namespace 下的 Pod
logview k8s deploy/<tab>          # 提示该 namespace 下的 Deployment
```

## 快捷键

按 `?` 打开帮助面板。帮助栏会根据当前模式自动切换显示内容。

### 导航

| 按键 | 功能 |
|------|------|
| `↑` / `k` | 上移一行 |
| `↓` / `j` | 下移一行 |
| `g` | 跳到顶部 |
| `G` | 跳到底部（自动滚动） |
| `C-u` | 上移半页 |
| `C-d` | 下移半页 |
| `C-b` | 整页上翻 |
| `C-f` | 整页下翻 |
| `PgUp` / `PgDn` | 翻页 |
| `H` | 跳到屏顶 |
| `M` | 跳到屏中 |
| `L` | 跳到屏底 |
| `zt` | 当前行置顶 |
| `zz` | 当前行居中 |
| `zb` | 当前行置底 |

### 搜索

| 按键 | 功能 |
|------|------|
| `f` / `/` | 打开搜索（弹窗显示当前行字段） |
| `Esc` | 清除搜索 |

搜索弹窗内支持左右方向键移动光标、中间插入/删除、Home/End 跳转。

| 按键 | 功能 |
|------|------|
| `Tab` | 下一个字段 |
| `S-Tab` | 上一个字段 |
| `C-j` / `C-k` | 上下切换字段 |
| `Enter` | 确认（选字段则插入，默认确认搜索） |
| `C-u` | 清空输入 |
| `Esc` | 取消搜索 |

### 高亮

按 `h` 打开高亮弹窗，输入关键词并用逗号分隔，每个关键词以不同颜色高亮显示。再次打开会保留上次的关键词。

```
ERROR, WARN, timeout    # 三个关键词分别以黄、青、品红高亮
```

清空输入后确认即可取消高亮。

### 选择与复制（Vim 风格）

| 按键 | 功能 |
|------|------|
| `v` | 进入可视化选择 |
| `V` | 进入可视化选择 |
| `y` | 复制选中 / 复制当前行 |
| `Esc` | 退出选择模式 |

### 日志级别过滤

| 按键 | 功能 |
|------|------|
| `E` | 仅 ERROR |
| `W` | ERROR + WARN |
| `I` | 去掉 DEBUG |
| `D` | 全部级别 |
| `A` | 取消过滤 |

### 其他

| 按键 | 功能 |
|------|------|
| `F` | 字段显示设置 |
| `s` | 导出日志 |
| `e` | 展开/折叠堆栈 |
| `h` | 高亮关键词 |
| `?` | 快捷键帮助 |
| `q` / `C-c` | 退出 |

## 特性

- **光标行自动展开**：光标所在行自动换行显示完整内容，无需横向滚动
- **上下文帮助栏**：帮助栏根据当前模式（搜索/选择/面板/导出）显示对应快捷键
- **多资源聚合**：同时查看多个 k8s Deployment / Pod 的日志
- **智能解析**：自动识别 JSON、Logback 等日志格式
- **字段配置**：按 `F` 自定义显示哪些字段
- **搜索语法**：支持 `field:value`、`AND/OR`、`after:/before:` 时间范围
