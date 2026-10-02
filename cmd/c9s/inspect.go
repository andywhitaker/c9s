package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"c9s/pkg/clab"
	"c9s/pkg/k8s"
	"c9s/pkg/safeguards"
	"c9s/pkg/ui"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	inspectLabName string
	inspectAll     bool
)

func init() {
	inspectCmd.Flags().StringVarP(&inspectLabName, "name", "n", "", "name of the lab to inspect")
	inspectCmd.Flags().BoolVarP(&inspectAll, "all", "a", false, "inspect all deployed labs")
	if inspectCmd.Parent() == nil {
		rootCmd.AddCommand(inspectCmd)
	}
}

var inspectCmd = &cobra.Command{
	Use:   "inspect [lab-name]",
	Short: "inspect lab details",
	Long: `inspect inspects deployed lab resources and displays a summary table of nodes,
containers, readiness states, and assigned IP addresses.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.PrintBanner()

		var labName string
		expectedNodes := make(map[string]string)

		// 1. If -n, --name is set, use it as labName.
		if inspectLabName != "" {
			labName = strings.TrimSpace(inspectLabName)
		// 2. Else if len(args) > 0, use args[0] as labName.
		} else if len(args) > 0 {
			labName = strings.TrimSpace(args[0])
		}

		// 3. Else if topoFile is set or if a topology file is found in current directory via clab.FindTopologyFile(topoFile):
		//    Parse the topology file via clab.ParseTopologyFile(...) -> resolve labName = parsed.Name, and populate expectedNodes from parsed.Topology.Nodes.
		// 4. If a topology file is explicitly specified via -t / --topo, parse it even if labName was provided via --name or arg, to enrich expectedNodes.
		if topoFile != "" {
			resolvedPath, err := clab.FindTopologyFile(topoFile)
			if err != nil {
				return err
			}
			parsed, _, err := clab.ParseTopologyFile(resolvedPath)
			if err != nil {
				return fmt.Errorf("topology parsing error: %w", err)
			}
			if labName == "" {
				labName = parsed.Name
			}
			for nodeName, node := range parsed.Topology.Nodes {
				expectedNodes[nodeName] = node.Kind
			}
		} else if labName == "" && !inspectAll {
			resolvedPath, err := clab.FindTopologyFile("")
			if err == nil {
				parsed, _, err := clab.ParseTopologyFile(resolvedPath)
				if err != nil {
					return fmt.Errorf("topology parsing error: %w", err)
				}
				labName = parsed.Name
				for nodeName, node := range parsed.Topology.Nodes {
					expectedNodes[nodeName] = node.Kind
				}
			} else if strings.Contains(err.Error(), "multiple topology files found") {
				return err
			}
		}

		// 5. If no lab name is specified and no topology file found:
		//    If --all is set, list and inspect all deployed labs.
		//    Else return a clear error instructing the user to provide a lab name, topology file, or --all.
		if labName == "" {
			if !inspectAll {
				return errors.New("no lab specified; provide a lab name, a topology file (-t/--topo), or use --all to inspect all deployed labs")
			}
		}

		// Connect to Kubernetes
		client, err := k8s.NewClient(kubeconfig, kubeCtx)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()

		// If --all is specified (and no specific labName):
		if inspectAll && labName == "" {
			labs, err := client.ListManagedLabs(ctx)
			if err != nil {
				return err
			}
			if len(labs) == 0 {
				ui.Info("No deployed labs found.")
				return nil
			}

			var allRows []ui.TableRow
			for _, lab := range labs {
				rows, err := client.GetSummaryRows(ctx, lab, nil)
				if err != nil {
					ui.Warn("Failed to inspect lab %q: %v", lab, err)
					continue
				}
				allRows = append(allRows, rows...)
			}

			if len(allRows) == 0 {
				ui.Info("No deployed labs found.")
				return nil
			}

			for i := range allRows {
				allRows[i].Index = i + 1
			}

			fmt.Println()
			ui.PrintTable(allRows)
			fmt.Println()
			return nil
		}

		// Specific labName
		nsName, err := safeguards.DeriveNamespace(labName)
		if err != nil {
			return err
		}

		nsObj, err := client.KubeClient.CoreV1().Namespaces().Get(ctx, nsName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				ui.Info("lab %q (namespace %q) not found or not deployed", labName, nsName)
				return nil
			}
			return fmt.Errorf("failed to get namespace %q: %w", nsName, err)
		}
		if nsObj == nil || nsObj.Status.Phase == corev1.NamespaceTerminating {
			ui.Info("lab %q (namespace %q) not found or not deployed", labName, nsName)
			return nil
		}

		rows, err := client.GetSummaryRows(ctx, labName, expectedNodes)
		if err != nil {
			return err
		}

		if len(rows) == 0 {
			ui.Info("No nodes found for lab %q", labName)
			return nil
		}

		fmt.Println()
		ui.PrintTable(rows)
		fmt.Println()
		return nil
	},
}
