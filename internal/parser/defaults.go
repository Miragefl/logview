package parser

// DefaultRulesYAML 返回内置默认 rules.yaml 模板（首跑落盘/文件缺失时使用）。
const defaultRulesYAML = `# ============================================================
# LogView 配置文件
# 修改后重新打开 logview 即可生效
# ============================================================

# patterns: 可复用的正则片段，在 rules 中用 {name} 引用
patterns:
  time: '(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[.,]\d{3})'
  hmsTime: '(?P<time>\d{2}:\d{2}(?::\d{2})?(?:[.,]\d{1,3})?)'
  thread: '(?P<thread>[^\]]+)'
  traceId: '(?P<traceId>[^\]]+)'
  level: '(?P<level>\w+)'
  simpleLevel: '(?P<level>TRACE|DEBUG|INFO|WARN|ERROR|FATAL)'
  logger: '(?P<logger>\S+)'
  message: '(?P<message>.*)'

# rules: 日志解析规则，按顺序匹配，每个来源只选第一条命中的规则
#   - pattern: 正则表达式，支持 {name} 引用 patterns
#   - parse: 可选，设为 json 则按 JSON 解析
rules:
  - name: java-logback
    # logger 列右补齐空格不定长,{logger}\s+- 容忍多空格(第三轮验收③)
    pattern: '{time} \[{thread}\] \[{traceId}\] {level}\s+{logger}\s+-\s+?{message}'
  # 无日期时间戳的简单日志（HH:mm:ss[.SSS] 级别 消息）：time: 搜索按当日时分窗口
  - name: simple-time-level
    pattern: '{hmsTime}\s+{simpleLevel}\s+{message}'
  - name: json-log
    pattern: '(?P<raw>.*)'
    parse: json
  - name: plain-text
    pattern: '{message}'

# history: -f 模式默认加载的尾行数
history: 5000

# theme: 配色主题
#   dark 系: dark / dracula / tokyo-storm / nord / gruvbox / catppuccin / one-dark
#   light系: light / gruvbox-light / catppuccin-latte / nord-light / tokyo-day / solarized-light
theme: dark

# theme_colors: 覆盖主题中的具体颜色（十六进制色码）
# 可配置项:
#   title.fg / title.bg      标题栏 前景/背景
#   level.debug               DEBUG 级别色
#   level.info                INFO 级别色
#   level.warn                WARN 级别色
#   level.error               ERROR 级别色
#   time / source / traceId / thread    字段颜色
#   highlight                 搜索高亮背景色
#   selected                  选中项背景色
#   visual                    可视选择背景色
#   popup.border / popup.bg   弹窗 边框/背景
#   dim                       暗淡文字色
#   accent                    强调色（标签、按键）
# theme_colors:
#   level.error: "#FF0000"
#   highlight: "#FFFF00"

# hides: 默认隐藏包含这些关键词的日志行，按 x 可管理
# hides:
#   - health check
#   - heartbeat

# ssh_hosts: 源选择器(o键) SSH tab 的常用主机候选（与 ~/.ssh/config 自动发现合并）
# ssh_hosts:
#   - web1
#   - deploy@10.0.0.5

# fields: 字段显示/隐藏，visible: false 隐藏但搜索和过滤仍可用
fields:
  - name: time
    visible: true
  - name: source
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
`

// DefaultRulesYAML 内置默认 rules.yaml 模板（cmd 装配与 fview 写回共用单源）。
func DefaultRulesYAML() string { return defaultRulesYAML }
