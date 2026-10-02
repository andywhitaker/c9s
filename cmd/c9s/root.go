package main

import (
	"context"
	"os"

	"c9s/pkg/ui"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"
)

var (
	topoFile   string
	kubeconfig string
	kubeCtx    string
	noColor    bool
)

var rootCmd = &cobra.Command{
	Use:           "c9s",
	Short:         "c9s - Containerlab on Kubernetes",
	Long:          `c9s is a command-line tool to deploy and manage Containerlab topologies natively on Kubernetes using Clabernetes.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if noColor {
			ui.DisableColors()
			_ = os.Setenv("NO_COLOR", "1")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		ui.PrintBanner()
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&topoFile, "topo", "t", "", "path to the topology definition file")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "path to the kubeconfig file")
	rootCmd.PersistentFlags().StringVar(&kubeCtx, "context", "", "name of the kubeconfig context to use")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(destroyCmd)
	if inspectCmd.Parent() == nil {
		rootCmd.AddCommand(inspectCmd)
	}
}

func Execute() {
	for _, arg := range os.Args[1:] {
		if arg == "--no-color" || arg == "--no-color=true" {
			noColor = true
			ui.DisableColors()
			_ = os.Setenv("NO_COLOR", "1")
			break
		}
	}

	if err := fang.Execute(context.Background(), rootCmd, fang.WithoutVersion(), fang.WithoutManpage()); err != nil {
		os.Exit(1)
	}
}

