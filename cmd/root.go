package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stream"
	"github.com/justfun/logview/internal/tui"
	"github.com/spf13/cobra"
)

var (
	ruleName   string
	bufferSize int
	configDir  string
)

var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

func SetVersion(v, c, d string) {
	buildVersion = v
	buildCommit = c
	buildDate = d
}

var rootCmd = &cobra.Command{
	Use:   "logview",
	Short: "Terminal log viewer with real-time search and filtering",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version info",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("logview %s (commit: %s, built: %s)\n", buildVersion, buildCommit, buildDate)
	},
}

var k8sCmd = &cobra.Command{
	Use:              "k8s <resource> [resource...] [flags]",
	Short:            "View logs from Kubernetes pods",
	Args:             cobra.MinimumNArgs(1),
	ValidArgsFunction: completeK8sResource,
	RunE: func(cmd *cobra.Command, args []string) error {
		parsers, history, defaultHides, rulesPath, _, err := loadParsers()
		if err != nil {
			return err
		}
		namespaces, _ := cmd.Flags().GetStringArray("namespace")
		k8sFollow, _ := cmd.Flags().GetBool("follow")
		k8sTail, _ := cmd.Flags().GetInt("tail")

		if k8sFollow && k8sTail == 0 {
			k8sTail = history
		}

		if len(namespaces) > 1 && len(namespaces) != len(args) {
			return fmt.Errorf("namespace count (%d) must match resource count (%d), or provide exactly 1 namespace for all resources",
				len(namespaces), len(args))
		}

		var src stream.LogStream
		if len(args) == 1 {
			ns := resolveNamespace(namespaces, 0)
			src = stream.NewK8sSource(args[0], ns, nil, k8sTail)
		} else {
			sources := make([]*stream.K8sSource, len(args))
			for i, res := range args {
				ns := resolveNamespace(namespaces, i)
				sources[i] = stream.NewK8sSource(res, ns, nil, k8sTail)
			}
			src = stream.NewMultiK8sSource(sources)
		}

		app := tui.NewApp(src, parsers, bufferSize, defaultHides)
		app.SetRulesPath(rulesPath)
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func resolveNamespace(namespaces []string, idx int) string {
	if len(namespaces) == 0 {
		return "default"
	}
	if len(namespaces) == 1 {
		return namespaces[0]
	}
	return namespaces[idx]
}

func completeK8sResource(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	namespaces, _ := cmd.Flags().GetStringArray("namespace")
	ns := resolveNamespace(namespaces, len(args))

	kinds := []struct{ prefix, kind string }{
		{"pod/", "pod"}, {"po/", "pod"},
		{"deploy/", "deployment"}, {"deployment/", "deployment"},
		{"sts/", "statefulset"}, {"statefulset/", "statefulset"},
	}
	for _, k := range kinds {
		if strings.HasPrefix(toComplete, k.prefix) {
			names := kubectlGetNames(k.kind, ns)
			var completions []string
			for _, n := range names {
				completions = append(completions, k.prefix+n)
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		}
	}

	var completions []string
	for _, k := range kinds {
		completions = append(completions, k.prefix)
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func kubectlGetNames(kind, namespace string) []string {
	args := []string{"get", kind, "-n", namespace, "-o", "jsonpath={.items[*].metadata.name}"}
	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return nil
	}
	return strings.Fields(strings.TrimSpace(string(out)))
}

func completeK8sNamespace(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	out, err := exec.Command("kubectl", "get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}").Output()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return strings.Fields(strings.TrimSpace(string(out))), cobra.ShellCompDirectiveNoFileComp
}

var tailCmd = &cobra.Command{
	Use:   "tail <file> [file...] [flags]",
	Short: "View logs from local files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parsers, history, defaultHides, rulesPath, _, err := loadParsers()
		if err != nil {
			return err
		}
		followMode, _ := cmd.Flags().GetBool("follow")
		readOnly, _ := cmd.Flags().GetBool("read-only")
		tailLines, _ := cmd.Flags().GetInt("tail")
		if readOnly {
			src := stream.NewFileSource(args)
			app := tui.NewApp(src, parsers, bufferSize, defaultHides)
			app.SetRulesPath(rulesPath)
			resume, _ := cmd.Flags().GetBool("resume")
			if resume {
				if s, err := tui.LoadSession(); err == nil {
					app.ApplySession(s)
				}
			}
			p := tea.NewProgram(app, tea.WithAltScreen())
			_, err = p.Run()
			return err
		}
		followLines := 0
		if followMode {
			if tailLines > 0 {
				followLines = tailLines
			} else {
				followLines = history
			}
		}
		src := stream.NewTailSource(args, followLines)
		app := tui.NewApp(src, parsers, bufferSize, defaultHides)
		app.SetRulesPath(rulesPath)
		resume, _ := cmd.Flags().GetBool("resume")
		if resume {
			if s, err := tui.LoadSession(); err == nil {
				app.ApplySession(s)
			}
		}
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

var pipeCmd = &cobra.Command{
	Use:   "pipe",
	Short: "View logs from stdin (pipe)",
	RunE: func(cmd *cobra.Command, args []string) error {
		parsers, _, defaultHides, rulesPath, _, err := loadParsers()
		if err != nil {
			return err
		}
		src := stream.NewPipeSource(os.Stdin)
		app := tui.NewApp(src, parsers, bufferSize, defaultHides)
		app.SetRulesPath(rulesPath)
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

var fileCmd = &cobra.Command{
	Use:   "file <file> [file...]",
	Short: "Open log file(s) in read-only mode",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parsers, _, defaultHides, rulesPath, _, err := loadParsers()
		if err != nil {
			return err
		}
		src := stream.NewFileSource(args)
		app := tui.NewApp(src, parsers, bufferSize, defaultHides)
		app.SetRulesPath(rulesPath)
		resume, _ := cmd.Flags().GetBool("resume")
		if resume {
			if s, err := tui.LoadSession(); err == nil {
				app.ApplySession(s)
			}
		}
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	k8sCmd.Flags().StringArrayP("namespace", "n", []string{"default"}, "Kubernetes namespace (one for all, or one per resource)")
	k8sCmd.Flags().BoolVarP(new(bool), "follow", "f", false, "follow mode: show last N lines then tail new content")
	k8sCmd.Flags().Int("tail", 0, "number of trailing lines in follow mode (default: config history)")
	k8sCmd.RegisterFlagCompletionFunc("namespace", completeK8sNamespace)
	rootCmd.PersistentFlags().StringVar(&ruleName, "rule", "", "parser rule name (auto-detect if empty)")
	rootCmd.PersistentFlags().IntVar(&bufferSize, "buffer-size", 100000, "ring buffer capacity")
	rootCmd.PersistentFlags().StringVar(&configDir, "config", "", "config directory (default: ~/.config/logview)")
	tailCmd.Flags().BoolP("follow", "f", false, "follow mode: show last N lines then tail new content")
	tailCmd.Flags().IntP("tail", "n", 0, "number of trailing lines in follow mode (default: config history)")
	tailCmd.Flags().BoolP("read-only", "r", false, "read-only mode: load file without following")
	tailCmd.Flags().BoolP("resume", "R", false, "restore last session state")
	fileCmd.Flags().BoolP("resume", "R", false, "restore last session state")
	rootCmd.AddCommand(k8sCmd)
	rootCmd.AddCommand(tailCmd)
	rootCmd.AddCommand(fileCmd)
	rootCmd.AddCommand(pipeCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd())
}

func completionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			default:
				return fmt.Errorf("unsupported shell: %s (use bash, zsh, or fish)", args[0])
			}
		},
	}
	return cmd
}

var tailNumFollowRe = regexp.MustCompile(`^-(\d+)f$`)

func Execute() {
	args := expandTailArgs(os.Args[1:])
	// auto-detect stdin pipe: if no subcommand and stdin is not a terminal, use pipe mode
	if len(args) == 0 || !isSubcommand(args[0]) {
		if info, _ := os.Stdin.Stat(); info.Mode()&os.ModeNamedPipe != 0 || !isTerminal(info) {
			args = append([]string{"pipe"}, args...)
		}
	}
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func isSubcommand(arg string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == arg {
			return true
		}
	}
	return false
}

func isTerminal(info os.FileInfo) bool {
	return info.Mode()&os.ModeCharDevice != 0
}

// expand -100f / -200f into -n 100 -f for cobra compatibility
func expandTailArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if m := tailNumFollowRe.FindStringSubmatch(a); m != nil {
			out = append(out, "--tail", m[1], "-f")
		} else {
			out = append(out, a)
		}
	}
	return out
}

func getConfigDir() string {
	if configDir != "" {
		return configDir
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "logview")
}

func loadParsers() (*parser.AutoDetect, int, []string, string, map[string]string, error) {
	cfgDir := getConfigDir()
	rulesPath := filepath.Join(cfgDir, "rules.yaml")

	var rules []parser.RuleConfig
	var fieldConfigs []parser.FieldConfig
	var history int
	var themeName string
	var themeColors map[string]string
	var defaultHides []string
	var keyBindings map[string]string
	if _, err := os.Stat(rulesPath); err == nil {
		rules, fieldConfigs, history, themeName, themeColors, defaultHides, keyBindings, _ = parser.LoadRules(rulesPath)
	} else {
		os.MkdirAll(cfgDir, 0755)
		os.WriteFile(rulesPath, []byte(defaultRulesYAML), 0644)
		rules, fieldConfigs, history, themeName, themeColors, defaultHides, keyBindings, _ = parser.LoadRules(rulesPath)
	}
	if history <= 0 {
		history = 5000
	}
	if len(rules) == 0 {
		rules = defaultFallbackRules()
	}

	if len(fieldConfigs) > 0 {
		var fields []model.Field
		var entries []model.FieldConfigEntry
		aliases := make(map[string]string)
		standardFields := []string{"time", "level", "thread", "traceId", "logger", "message", "source"}
		for _, fc := range fieldConfigs {
			f := model.Field(fc.Name)
			fields = append(fields, f)
			entries = append(entries, model.FieldConfigEntry{Name: fc.Name, Visible: fc.Visible})
			isStandard := false
			for _, std := range standardFields {
				if fc.Name == std {
					isStandard = true
					break
				}
			}
			if !isStandard {
				for _, std := range standardFields {
					if strings.HasPrefix(std, fc.Name) || strings.Contains(std, fc.Name) {
						aliases[fc.Name] = std
						break
					}
				}
			}
		}
		model.SetAllFields(fields)
		tui.SetFieldMask(model.NewFieldMaskFromConfig(entries))
		tui.SetFieldAlias(aliases)
	}

	parsers := parser.MustCompileRules(rules)
	cfg := tui.ResolveTheme(themeName, themeColors)
	tui.ApplyTheme(cfg)
	return parser.NewAutoDetect(parsers), history, defaultHides, rulesPath, keyBindings, nil
}

const defaultRulesYAML = `# ============================================================
# LogView 配置文件
# 修改后重新打开 logview 即可生效
# ============================================================

# patterns: 可复用的正则片段，在 rules 中用 {name} 引用
patterns:
  time: '(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[.,]\d{3})'
  thread: '(?P<thread>[^\]]+)'
  traceId: '(?P<traceId>[^\]]+)'
  level: '(?P<level>\w+)'
  logger: '(?P<logger>\S+)'
  message: '(?P<message>.*)'

# rules: 日志解析规则，按顺序匹配，每个来源只选第一条命中的规则
#   - pattern: 正则表达式，支持 {name} 引用 patterns
#   - parse: 可选，设为 json 则按 JSON 解析
rules:
  - name: java-logback
    pattern: '{time} \[{thread}\] \[{traceId}\] {level}\s+{logger} - ?{message}'
  - name: json-log
    pattern: '(?P<raw>.*)'
    parse: json
  - name: plain-text
    pattern: '{message}'

# history: -f 模式默认加载的尾行数
history: 5000

# theme: 配色主题，可选 dark / light
theme: dark

# theme_colors: 覆盖主题中的具体颜色（十六进制色码）
# 可配置项:
#   title.fg / title.bg      标题栏 前景/背景
#   level.debug               DEBUG 级别色
#   level.info                INFO 级别色
#   level.warn                WARN 级别色
#   level.error               ERROR 级别色
#   time / source / traceId / thread    字段颜色
#   error_line_bg / warn_line_bg        ERROR/WARN 行背景
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

func defaultFallbackRules() []parser.RuleConfig {
	return []parser.RuleConfig{
		{
			Name:    "java-logback",
			Pattern: `(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[.,]\d{3})\s+\[(?P<thread>[^\]]+)\]\s+\[(?P<traceId>[^\]]+)\]\s+(?P<level>\w+)\s+(?P<logger>\S+)\s+-\s*(?P<message>.*)`,
		},
		{
			Name:  "json-log",
			Parse: "json",
		},
		{
			Name:    "plain-text",
			Pattern: `(?P<message>.*)`,
		},
	}
}
