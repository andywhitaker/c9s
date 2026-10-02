package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"c9s/pkg/clab"
	"c9s/pkg/k8s"
	"c9s/pkg/ui"

	"github.com/spf13/cobra"
)

var labNameFlag string

func init() {
	destroyCmd.Flags().StringVarP(&labNameFlag, "name", "n", "", "name of the lab to destroy")
}

var destroyCmd = &cobra.Command{
	Use:   "destroy [lab-name]",
	Short: "destroy a containerlab topology on Kubernetes",
	Long: `destroy removes a deployed lab from Kubernetes by deleting the Clabernetes Topology resource
and tearing down the dedicated lab-{name_of_lab} namespace. The lab can be specified by topology file,
--name flag, or positional argument.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.PrintBanner()

		labName := labNameFlag
		if labName == "" && len(args) > 0 {
			labName = args[0]
		}

		if labName == "" {
			// Find Topology file to determine lab name
			resolvedPath, err := clab.FindTopologyFile(topoFile)
			if err != nil {
				return err
			}
			ui.Info("Parsing containerlab topology file %q to resolve lab name...", filepath.Base(resolvedPath))

			parsed, _, err := clab.ParseTopologyFile(resolvedPath)
			if err != nil {
				return fmt.Errorf("topology parsing error: %w", err)
			}
			labName = parsed.Name
		}

		// 2. Connect to Kubernetes (with safety checks)
		client, err := k8s.NewClient(kubeconfig, kubeCtx)
		if err != nil {
			return err
		}
		ui.Info("Connected to cluster %q (context: %q)", client.ClusterName, client.ContextName)

		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()

		// 3. Delete Lab
		ui.Info("Destroying lab %q (namespace lab-%s)...", labName, labName)
		if err := client.DeleteLab(ctx, labName); err != nil {
			return err
		}

		ui.Success("Lab %q destroyed successfully!", labName)
		return nil
	},
}
