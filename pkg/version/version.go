package version

import (
	"fmt"
	"strings"
)

var (
	// Version is the current version of the c9s CLI tool.
	Version = "0.0.1"
	// Commit holds the git commit hash set at build time.
	Commit = "unknown"
	// Date holds the build timestamp set at build time.
	Date = "unknown"
	// Source is the upstream repository.
	Source = "https://github.com/andywhitaker/c9s"
	// TargetGroup is the Kubernetes CRD group version.
	TargetGroup = "c9s.run/v1alpha1"
)

// Info returns formatted version details similar to containerlab.
func Info() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("    version: %s\n", Version))
	if Commit != "" && Commit != "unknown" {
		sb.WriteString(fmt.Sprintf("     commit: %s\n", Commit))
	}
	if Date != "" && Date != "unknown" {
		sb.WriteString(fmt.Sprintf("       date: %s\n", Date))
	}
	sb.WriteString(fmt.Sprintf("     source: %s\n", Source))
	sb.WriteString(fmt.Sprintf(" target API: %s\n", TargetGroup))
	return sb.String()
}
