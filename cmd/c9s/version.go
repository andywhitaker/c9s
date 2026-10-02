package main

import (
	"fmt"

	"c9s/pkg/ui"
	"c9s/pkg/version"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show c9s version and exit",
	Run: func(cmd *cobra.Command, args []string) {
		ui.PrintBanner()
		fmt.Print(version.Info())
	},
}
