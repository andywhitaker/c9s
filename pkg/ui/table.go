package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// TableRow represents a row in the summary table.
type TableRow struct {
	Index     int
	LabName   string
	Namespace string
	NodeName  string
	Kind      string
	Image     string
	State     string
	IPv4         string
	IPv4Internal string
	IPv6         string
	IPv6Internal string
}

// PrintTable formats and displays the summary table matching containerlab style.
func PrintTable(rows []TableRow) {
	if len(rows) == 0 {
		return
	}

	type labGroup struct {
		labName   string
		namespace string
		rows      []TableRow
	}

	var groups []labGroup
	groupMap := make(map[string]int)

	for _, r := range rows {
		lab := r.LabName
		ns := r.Namespace
		if ns == "" && lab != "" {
			ns = fmt.Sprintf("lab-%s", lab)
		}
		key := lab + "\x00" + ns
		if idx, ok := groupMap[key]; ok {
			groups[idx].rows = append(groups[idx].rows, r)
		} else {
			groupMap[key] = len(groups)
			groups = append(groups, labGroup{
				labName:   lab,
				namespace: ns,
				rows:      []TableRow{r},
			})
		}
	}

	for gIdx, group := range groups {
		if gIdx > 0 {
			fmt.Println()
		}

		headers := []string{"#", "Name", "Kind", "Image", "State", "IP Address (Ext)", "IP Address (Int)"}
		colWidths := make([]int, len(headers))
		for i, h := range headers {
			colWidths[i] = len(h)
		}

		var dataRows [][]string
		for _, r := range group.rows {
			ipv4Ext := r.IPv4
			if ipv4Ext == "" {
				ipv4Ext = "N/A"
			}
			ipv4Int := r.IPv4Internal
			if ipv4Int == "" {
				ipv4Int = "N/A"
			}

			rowStr := []string{
				fmt.Sprintf("%d", r.Index),
				r.NodeName,
				r.Kind,
				r.Image,
				r.State,
				ipv4Ext,
				ipv4Int,
			}
			for cIdx, val := range rowStr {
				if len(val) > colWidths[cIdx] {
					colWidths[cIdx] = len(val)
				}
			}
			dataRows = append(dataRows, rowStr)

			var ipv6Ext, ipv6Int string
			if r.IPv6Internal != "" && r.IPv6Internal != "N/A" {
				ipv6Int = r.IPv6Internal
			}
			if r.IPv6 != "" && r.IPv6 != "N/A" {
				if ipv6Int == "" && (strings.Contains(r.IPv6, "/") || (r.IPv4Internal != "" && r.IPv4 == "")) {
					ipv6Int = r.IPv6
				} else {
					ipv6Ext = r.IPv6
				}
			}

			if ipv6Ext != "" || ipv6Int != "" {
				row6Str := []string{
					"",
					"",
					"",
					"",
					"",
					ipv6Ext,
					ipv6Int,
				}
				for cIdx, val := range row6Str {
					if len(val) > colWidths[cIdx] {
						colWidths[cIdx] = len(val)
					}
				}
				dataRows = append(dataRows, row6Str)
			}
		}

		// Padding
		for i := range colWidths {
			colWidths[i] += 2 // 1 space on each side
		}

		totalWidth := 0
		for _, w := range colWidths {
			totalWidth += w
		}
		totalWidth += len(colWidths) - 1

		labName := group.labName
		namespace := group.namespace
		if namespace == "" && labName != "" {
			namespace = fmt.Sprintf("lab-%s", labName)
		}

		hasTitle := labName != "" || namespace != ""
		if hasTitle {
			plainTitle := fmt.Sprintf(" Lab: %s  •  Namespace: %s", labName, namespace)
			minWidth := utf8.RuneCountInString(plainTitle) + 1
			if totalWidth < minWidth {
				colWidths[len(colWidths)-1] += minWidth - totalWidth
				totalWidth = minWidth
			}
		}

		// Build separator line
		var sepParts []string
		for _, w := range colWidths {
			sepParts = append(sepParts, strings.Repeat("-", w))
		}
		separator := fmt.Sprintf("+%s+", strings.Join(sepParts, "+"))

		if hasTitle {
			// Render an integrated top title box spanning the full table width
			topBorder := fmt.Sprintf("+%s+", strings.Repeat("-", totalWidth))
			fmt.Printf("%s%s%s\n", ColorDim, topBorder, ColorReset)

			plainTitle := fmt.Sprintf(" Lab: %s  •  Namespace: %s", labName, namespace)
			pad := totalWidth - utf8.RuneCountInString(plainTitle)
			if pad < 0 {
				pad = 0
			}

			titleContent := fmt.Sprintf(" %sLab:%s %s%s%s  %s•%s  %sNamespace:%s %s%s%s%s",
				ColorBold, ColorReset,
				ColorClabBlue, labName, ColorReset,
				ColorDim, ColorReset,
				ColorBold, ColorReset,
				ColorBoldGreen, namespace, ColorReset,
				strings.Repeat(" ", pad),
			)
			fmt.Printf("|%s|\n", titleContent)
			fmt.Printf("%s%s%s\n", ColorDim, separator, ColorReset)
		} else {
			// Print top border
			fmt.Printf("%s%s%s\n", ColorDim, separator, ColorReset)
		}

		// Print header
		var headerParts []string
		for i, h := range headers {
			pad := colWidths[i] - len(h) - 1
			headerParts = append(headerParts, fmt.Sprintf(" %s%s%s%s", ColorBold, h, ColorReset, strings.Repeat(" ", pad)))
		}
		fmt.Printf("|%s|\n", strings.Join(headerParts, "|"))
		fmt.Printf("%s%s%s\n", ColorDim, separator, ColorReset)

		// Print rows
		for _, r := range dataRows {
			var rowParts []string
			for i, cell := range r {
				pad := colWidths[i] - len(cell) - 1
				coloredCell := cell
				if headers[i] == "State" {
					lower := strings.ToLower(cell)
					if lower == "running" || lower == "ready" {
						coloredCell = fmt.Sprintf("%s%s%s", ColorBoldGreen, cell, ColorReset)
					} else if lower == "pending" || lower == "deploying" {
						coloredCell = fmt.Sprintf("%s%s%s", ColorBoldYellow, cell, ColorReset)
					} else if lower == "failed" || lower == "error" {
						coloredCell = fmt.Sprintf("%s%s%s", ColorBoldRed, cell, ColorReset)
					}
				}
				rowParts = append(rowParts, fmt.Sprintf(" %s%s", coloredCell, strings.Repeat(" ", pad)))
			}
			fmt.Printf("|%s|\n", strings.Join(rowParts, "|"))
		}

		// Print bottom border
		fmt.Printf("%s%s%s\n", ColorDim, separator, ColorReset)
	}
}

