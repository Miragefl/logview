package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	ruleName   string
	bufferSize int
)

var rootCmd = &cobra.Command{
	Use:   "logview",
	Short: "Terminal log viewer with real-time search and filtering",
}

var k8sCmd = &cobra.Command{
	Use:   "k8s <resource> [flags]",
	Short: "View logs from Kubernetes pods",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resource := args[0]
		namespace, _ := cmd.Flags().GetString("namespace")
		fmt.Printf("k8s: resource=%s namespace=%s\n", resource, namespace)
		// TUI wiring will be added in Task 13
		return nil
	},
}

var tailCmd = &cobra.Command{
	Use:   "tail <file> [file...] [flags]",
	Short: "View logs from local files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("tail: files=%v\n", args)
		// TUI wiring will be added in Task 13
		return nil
	},
}

var pipeCmd = &cobra.Command{
	Use:   "pipe",
	Short: "View logs from stdin (pipe)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("pipe: reading from stdin")
		// TUI wiring will be added in Task 13
		return nil
	},
}

func init() {
	k8sCmd.Flags().StringP("namespace", "n", "default", "Kubernetes namespace")
	rootCmd.PersistentFlags().StringVar(&ruleName, "rule", "", "parser rule name (auto-detect if empty)")
	rootCmd.PersistentFlags().IntVar(&bufferSize, "buffer-size", 100000, "ring buffer capacity")
	rootCmd.AddCommand(k8sCmd)
	rootCmd.AddCommand(tailCmd)
	rootCmd.AddCommand(pipeCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}