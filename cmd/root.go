package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	followOnly bool
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
	Use:   "k8s <resource> [resource...] [flags]",
	Short: "View logs from Kubernetes pods",
	Args:  cobra.MinimumNArgs(1),
	ValidArgsFunction: completeK8sResource,
	RunE: func(cmd *cobra.Command, args []string) error {
		parsers, err := loadParsers()
		if err != nil {
			return err
		}
		namespaces, _ := cmd.Flags().GetStringArray("namespace")

		if len(namespaces) > 1 && len(namespaces) != len(args) {
			return fmt.Errorf("namespace count (%d) must match resource count (%d), or provide exactly 1 namespace for all resources",
				len(namespaces), len(args))
		}

		var src stream.LogStream
		if len(args) == 1 {
			ns := resolveNamespace(namespaces, 0)
			src = stream.NewK8sSource(args[0], ns, nil)
		} else {
			sources := make([]*stream.K8sSource, len(args))
			for i, res := range args {
				ns := resolveNamespace(namespaces, i)
				sources[i] = stream.NewK8sSource(res, ns, nil)
			}
			src = stream.NewMultiK8sSource(sources)
		}

		app := tui.NewApp(src, parsers, bufferSize)
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
		parsers, err := loadParsers()
		if err != nil {
			return err
		}
		src := stream.NewTailSource(args, followOnly)
		app := tui.NewApp(src, parsers, bufferSize)
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

var pipeCmd = &cobra.Command{
	Use:   "pipe",
	Short: "View logs from stdin (pipe)",
	RunE: func(cmd *cobra.Command, args []string) error {
		parsers, err := loadParsers()
		if err != nil {
			return err
		}
		src := stream.NewPipeSource(os.Stdin)
		app := tui.NewApp(src, parsers, bufferSize)
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	k8sCmd.Flags().StringArrayP("namespace", "n", []string{"default"}, "Kubernetes namespace (one for all, or one per resource)")
	k8sCmd.RegisterFlagCompletionFunc("namespace", completeK8sNamespace)
	rootCmd.PersistentFlags().StringVar(&ruleName, "rule", "", "parser rule name (auto-detect if empty)")
	rootCmd.PersistentFlags().IntVar(&bufferSize, "buffer-size", 100000, "ring buffer capacity")
	rootCmd.PersistentFlags().StringVar(&configDir, "config", "", "config directory (default: ~/.config/logview)")
	tailCmd.Flags().BoolVarP(&followOnly, "follow", "f", false, "follow mode: skip existing content, only show new lines")
	rootCmd.AddCommand(k8sCmd)
	rootCmd.AddCommand(tailCmd)
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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getConfigDir() string {
	if configDir != "" {
		return configDir
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "logview")
}

func loadParsers() (*parser.AutoDetect, error) {
	rulesPath := filepath.Join(getConfigDir(), "rules.yaml")

	var rules []parser.RuleConfig
	var fieldConfigs []parser.FieldConfig
	if _, err := os.Stat(rulesPath); err == nil {
		rules, fieldConfigs, _ = parser.LoadRules(rulesPath)
	}
	if len(rules) == 0 {
		rules = defaultRules()
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
	return parser.NewAutoDetect(parsers), nil
}

func defaultRules() []parser.RuleConfig {
	return []parser.RuleConfig{
		{
			Name:    "java-logback",
			Pattern: `(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[.,]\d{3})\s+\[(?P<thread>[^\]]+)\]\s+\[(?P<traceId>[^\]]+)\]\s+(?P<level>\w+)\s+(?P<logger>\S+)\s+-\s+(?P<message>.*)`,
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
