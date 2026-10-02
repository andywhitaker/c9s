package main

import (
	"errors"
	"fmt"
	"strings"

	"c9s/pkg/connect"
	"c9s/pkg/k8s"

	"github.com/spf13/cobra"
)

var (
	connectNamespace      string
	connectUsername       string
	connectSSH            bool
	connectExec           bool
	connectClientOverride *k8s.Client
)

func init() {
	connectCmd.Flags().StringVarP(&connectNamespace, "namespace", "n", "", "kubernetes namespace of the lab node")
	connectCmd.Flags().StringVarP(&connectUsername, "username", "u", "", "username for SSH connection")
	connectCmd.Flags().BoolVar(&connectSSH, "ssh", false, "force SSH connection method")
	connectCmd.Flags().BoolVar(&connectExec, "exec", false, "force kubectl exec connection method")

	if connectCmd.Parent() == nil {
		rootCmd.AddCommand(connectCmd)
	}
}

var connectCmd = &cobra.Command{
	Use:   "connect [flags] [user@]node [-- [command...]]",
	Short: "connect to a lab node via SSH or kubectl exec",
	Long: `connect connects directly to a deployed lab node via SSH or kubectl exec.
By default, connection method and username are automatically determined based on the node kind.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if connectSSH && connectExec {
			return errors.New("cannot specify both --ssh and --exec")
		}

		dashIdx := cmd.ArgsLenAtDash()
		var nodeTarget string
		var customCommand []string

		if dashIdx != -1 {
			if dashIdx == 0 {
				return errors.New("node name is required before '--'")
			}
			nodeTarget = args[0]
			customCommand = args[dashIdx:]
			if dashIdx > 1 {
				return fmt.Errorf("unexpected argument %q; only [user@]node allowed before '--'", args[1])
			}
		} else {
			if len(args) == 0 {
				return errors.New("node name is required; usage: c9s connect [flags] [user@]node [-- [command...]]")
			}
			nodeTarget = args[0]
			if len(args) > 1 {
				customCommand = args[1:]
			}
		}

		topo := topoFile
		if f := cmd.Flags().Lookup("topo"); f != nil && f.Value.String() != "" {
			topo = f.Value.String()
		}

		opts := connect.ConnectOptions{
			Namespace:     strings.TrimSpace(connectNamespace),
			TopoFile:      strings.TrimSpace(topo),
			NodeTarget:    strings.TrimSpace(nodeTarget),
			Username:      strings.TrimSpace(connectUsername),
			ForceSSH:      connectSSH,
			ForceExec:     connectExec,
			Kubeconfig:    strings.TrimSpace(kubeconfig),
			KubeContext:   strings.TrimSpace(kubeCtx),
			CustomCommand: customCommand,
			Client:        connectClientOverride,
		}

		return connect.RunConnect(cmd.Context(), opts)
	},
}
