package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"c9s/pkg/clab"
	"c9s/pkg/converter"
	"c9s/pkg/k8s"
	"c9s/pkg/ui"

	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy a containerlab topology on Kubernetes",
	Long: `deploy parses a containerlab topology definition file, copies startup configs into ConfigMaps,
and deploys the lab as a Clabernetes Topology Custom Resource inside a dedicated lab-{name_of_lab} namespace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.PrintBanner()

		// 1. Find Topology file
		resolvedPath, err := clab.FindTopologyFile(topoFile)
		if err != nil {
			return err
		}
		ui.Info("Parsing & validating containerlab topology file %q...", filepath.Base(resolvedPath))

		// 2. Parse & Validate
		parsed, rawYaml, err := clab.ParseTopologyFile(resolvedPath)
		if err != nil {
			return fmt.Errorf("topology validation error: %w", err)
		}
		ui.Info("Topology %q validated successfully (%d node(s), %d link(s))",
			parsed.Name, len(parsed.Topology.Nodes), len(parsed.Topology.Links))

		// 3. Convert to Kubernetes resources
		topoDir := filepath.Dir(resolvedPath)
		res, err := converter.Convert(parsed, topoDir, rawYaml)
		if err != nil {
			return fmt.Errorf("conversion error: %w", err)
		}

		// 4. Connect to Kubernetes (with safety checks)
		client, err := k8s.NewClient(kubeconfig, kubeCtx)
		if err != nil {
			return err
		}
		ui.Info("Connected to cluster %q (context: %q)", client.ClusterName, client.ContextName)

		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()

		// 5. Apply Namespace
		ui.Info("Ensuring lab namespace %q...", res.Namespace.Name)
		if err := client.ApplyNamespace(ctx, res.Namespace); err != nil {
			return err
		}

		// 6. Apply ConfigMaps for startup configs
		for _, cm := range res.ConfigMaps {
			for filename := range cm.Data {
				ui.Info("Creating ConfigMap %q in %q (key: %s)...", cm.Name, cm.Namespace, filename)
			}
			if err := client.ApplyConfigMap(ctx, cm); err != nil {
				return err
			}
		}

		// 7. Apply Clabernetes Topology Custom Resource
		ui.Info("Deploying Clabernetes Topology CR %q in namespace %q...", res.TopologyCR.Name, res.TopologyCR.Namespace)
		if err := client.ApplyTopology(ctx, res.TopologyCR); err != nil {
			return err
		}

		ui.Success("Topology %q submitted to Clabernetes controller successfully!", parsed.Name)

		// 8. Poll briefly for controller compilation and display node summary table
		ui.Info("Querying node states...")
		expectedNodes := make(map[string]string)
		for nodeName, node := range parsed.Topology.Nodes {
			expectedNodes[nodeName] = node.Kind
		}

		// Wait 3 seconds for controller to create child CRs
		time.Sleep(3 * time.Second)

		rows, err := client.GetSummaryRows(ctx, parsed.Name, expectedNodes)
		if err == nil && len(rows) > 0 {
			fmt.Println()
			ui.PrintTable(rows)
			fmt.Println()
		}

		ui.Success("Lab %q deployed successfully in namespace %q!", parsed.Name, res.Namespace.Name)
		return nil
	},
}
