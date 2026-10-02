package ui

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func captureStdout(f func()) string {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintTableWithTitleBox(t *testing.T) {
	ResetColors()

	rows := []TableRow{
		{
			Index:        1,
			LabName:      "mytest",
			Namespace:    "lab-mytest",
			NodeName:     "srl1",
			Kind:         "nokia_srlinux",
			Image:        "ghcr.io/nokia/srlinux:latest",
			State:        "Running",
			IPv4:         "172.18.255.1",
			IPv4Internal: "172.20.20.2",
			IPv6:         "N/A",
		},
	}

	out := captureStdout(func() {
		PrintTable(rows)
	})

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) < 6 {
		t.Fatalf("expected at least 6 lines of table output, got %d:\n%s", len(lines), out)
	}

	// Verify top border
	if !strings.HasPrefix(lines[0], ColorDim+"+") || !strings.HasSuffix(lines[0], "+"+ColorReset) {
		t.Errorf("expected top border with ColorDim, got %q", lines[0])
	}

	// Verify title line contains Lab name and Namespace
	if !strings.Contains(lines[1], "Lab:") || !strings.Contains(lines[1], "mytest") {
		t.Errorf("expected Lab name in title line, got %q", lines[1])
	}
	if !strings.Contains(lines[1], "•") {
		t.Errorf("expected bullet separator in title line, got %q", lines[1])
	}
	if !strings.Contains(lines[1], "Namespace:") || !strings.Contains(lines[1], "lab-mytest") {
		t.Errorf("expected Namespace in title line, got %q", lines[1])
	}

	// Verify mid-border
	if !strings.Contains(lines[2], "+-") {
		t.Errorf("expected mid-border on line 2, got %q", lines[2])
	}

	// Verify column headers
	if !strings.Contains(lines[3], "#") || !strings.Contains(lines[3], "Name") || !strings.Contains(lines[3], "Kind") || !strings.Contains(lines[3], "IP Address (Ext)") || !strings.Contains(lines[3], "IP Address (Int)") {
		t.Errorf("expected header row on line 3, got %q", lines[3])
	}
	if strings.Contains(lines[3], "IPv6 Address") {
		t.Errorf("expected header row to NOT contain 'IPv6 Address', got %q", lines[3])
	}

	// Verify row values contain both external and internal IPs
	if !strings.Contains(lines[5], "172.18.255.1") || !strings.Contains(lines[5], "172.20.20.2") {
		t.Errorf("expected row to contain external and internal IPs, got %q", lines[5])
	}
}

func TestPrintTableDefaultNamespace(t *testing.T) {
	ResetColors()

	rows := []TableRow{
		{
			Index:        1,
			LabName:      "autons",
			Namespace:    "",
			NodeName:     "node1",
			Kind:         "linux",
			Image:        "alpine:latest",
			State:        "Ready",
			IPv4:         "172.18.255.10",
			IPv4Internal: "10.0.0.1",
			IPv6:         "N/A",
		},
	}

	out := captureStdout(func() {
		PrintTable(rows)
	})

	if !strings.Contains(out, "Namespace:") || !strings.Contains(out, "lab-autons") {
		t.Errorf("expected defaulted namespace 'lab-autons' in output, got:\n%s", out)
	}
}

func TestPrintTableNoColorAlignment(t *testing.T) {
	DisableColors()
	defer ResetColors()

	rows := []TableRow{
		{
			Index:        1,
			LabName:      "alignment-lab",
			Namespace:    "lab-alignment-lab",
			NodeName:     "router1",
			Kind:         "nokia_srlinux",
			Image:        "ghcr.io/nokia/srlinux:24.3.1",
			State:        "Running",
			IPv4:         "172.18.255.10",
			IPv4Internal: "172.20.20.100/24",
			IPv6:         "2001:db8::1/64",
		},
		{
			Index:        2,
			LabName:      "alignment-lab",
			Namespace:    "lab-alignment-lab",
			NodeName:     "client",
			Kind:         "linux",
			Image:        "ghcr.io/srl-labs/network-multitool",
			State:        "Pending",
			IPv4:         "172.18.255.11",
			IPv4Internal: "172.20.20.101/24",
			IPv6:         "N/A",
		},
	}

	out := captureStdout(func() {
		PrintTable(rows)
	})

	// Verify no ANSI escape codes exist
	if strings.Contains(out, "\033[") {
		t.Errorf("expected no ANSI color escapes when DisableColors is active, got:\n%s", out)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("expected non-empty output")
	}

	expectedWidth := utf8.RuneCountInString(lines[0])
	for i, line := range lines {
		runeCount := utf8.RuneCountInString(line)
		if runeCount != expectedWidth {
			t.Errorf("line %d width mismatch: got %d runes, want %d runes\nLine: %q", i, runeCount, expectedWidth, line)
		}
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "|") {
			t.Errorf("line %d does not start with + or |: %q", i, line)
		}
		if !strings.HasSuffix(line, "+") && !strings.HasSuffix(line, "|") {
			t.Errorf("line %d does not end with + or |: %q", i, line)
		}
	}
}

func TestPrintTableMultipleLabsGrouping(t *testing.T) {
	ResetColors()

	rows := []TableRow{
		{
			Index:        1,
			LabName:      "alpha",
			Namespace:    "lab-alpha",
			NodeName:     "a-node1",
			Kind:         "linux",
			Image:        "alpine:latest",
			State:        "Running",
			IPv4:         "172.18.255.1",
			IPv4Internal: "10.1.1.1",
			IPv6:         "N/A",
		},
		{
			Index:        2,
			LabName:      "alpha",
			Namespace:    "lab-alpha",
			NodeName:     "a-node2",
			Kind:         "linux",
			Image:        "alpine:latest",
			State:        "Running",
			IPv4:         "172.18.255.2",
			IPv4Internal: "10.1.1.2",
			IPv6:         "N/A",
		},
		{
			Index:        3,
			LabName:      "beta",
			Namespace:    "lab-beta",
			NodeName:     "b-node1",
			Kind:         "nokia_srlinux",
			Image:        "srlinux:latest",
			State:        "Running",
			IPv4:         "172.18.255.3",
			IPv4Internal: "10.2.1.1",
			IPv6:         "N/A",
		},
	}

	out := captureStdout(func() {
		PrintTable(rows)
	})

	// Must contain both labs
	if !strings.Contains(out, "Lab:") || !strings.Contains(out, "alpha") {
		t.Errorf("expected Lab: alpha in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Lab:") || !strings.Contains(out, "beta") {
		t.Errorf("expected Lab: beta in output, got:\n%s", out)
	}

	// Tables should be separated by an empty line "\n\n"
	if !strings.Contains(out, "\n\n") {
		t.Errorf("expected empty line separating tables of multiple labs, got:\n%s", out)
	}
}

func TestPrintTableNoLabNameOrNamespace(t *testing.T) {
	ResetColors()

	rows := []TableRow{
		{
			Index:        1,
			LabName:      "",
			Namespace:    "",
			NodeName:     "node1",
			Kind:         "linux",
			Image:        "alpine:latest",
			State:        "Running",
			IPv4:         "172.18.255.1",
			IPv4Internal: "10.0.0.1",
			IPv6:         "N/A",
		},
	}

	out := captureStdout(func() {
		PrintTable(rows)
	})

	if strings.Contains(out, "Lab:") || strings.Contains(out, "Namespace:") {
		t.Errorf("expected no title box when LabName and Namespace are empty, got:\n%s", out)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	// First line should be column separator, not a full title border
	if !strings.Contains(lines[0], "+-") {
		t.Errorf("expected first line to be table column separator, got %q", lines[0])
	}
}

func TestPrintTableEmptyIPDefaults(t *testing.T) {
	ResetColors()

	rows := []TableRow{
		{
			Index:        1,
			LabName:      "emptyips",
			Namespace:    "lab-emptyips",
			NodeName:     "node1",
			Kind:         "linux",
			Image:        "alpine:latest",
			State:        "Running",
			IPv4:         "",
			IPv4Internal: "",
			IPv6:         "",
		},
	}

	out := captureStdout(func() {
		PrintTable(rows)
	})

	if !strings.Contains(out, "IP Address (Ext)") || !strings.Contains(out, "IP Address (Int)") {
		t.Errorf("expected IP Address (Ext) and IP Address (Int) headers, got:\n%s", out)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	rowLine := lines[5]
	count := strings.Count(rowLine, "N/A")
	if count != 2 {
		t.Errorf("expected exactly 2 'N/A' values in row for empty IPs (Ext and Int), got %d in: %s", count, rowLine)
	}
	// Verify device with no IPv6 takes only 1 data row
	// Lines: 0=top, 1=title, 2=mid, 3=header, 4=mid, 5=row1, 6=bottom
	if len(lines) != 7 {
		t.Errorf("expected 7 lines for single device without IPv6, got %d:\n%s", len(lines), out)
	}
}

func TestPrintTableEmptyRows(t *testing.T) {
	out := captureStdout(func() {
		PrintTable([]TableRow{})
	})
	if out != "" {
		t.Errorf("expected no output for empty rows, got %q", out)
	}
}

func TestPrintTableIPv6LayoutAndHeaders(t *testing.T) {
	DisableColors()
	defer ResetColors()

	rows := []TableRow{
		{
			Index:        1,
			LabName:      "ipv6-lab",
			Namespace:    "lab-ipv6-lab",
			NodeName:     "router-dual",
			Kind:         "nokia_srlinux",
			Image:        "ghcr.io/nokia/srlinux:24.3.1",
			State:        "Running",
			IPv4:         "172.18.255.1",
			IPv4Internal: "172.20.20.2/24",
			IPv6:         "2001:db8::1",
			IPv6Internal: "2001:db8::2/64",
		},
		{
			Index:        2,
			LabName:      "ipv6-lab",
			Namespace:    "lab-ipv6-lab",
			NodeName:     "client-ipv4-only",
			Kind:         "linux",
			Image:        "alpine:latest",
			State:        "Ready",
			IPv4:         "172.18.255.3",
			IPv4Internal: "172.20.20.3/24",
			IPv6:         "",
			IPv6Internal: "",
		},
		{
			Index:        3,
			LabName:      "ipv6-lab",
			Namespace:    "lab-ipv6-lab",
			NodeName:     "router-int-ipv6",
			Kind:         "nokia_srlinux",
			Image:        "ghcr.io/nokia/srlinux:24.3.1",
			State:        "Pending",
			IPv4:         "172.18.255.4",
			IPv4Internal: "172.20.20.4/24",
			IPv6:         "",
			IPv6Internal: "2001:db8:4::1/64",
		},
		{
			Index:        4,
			LabName:      "ipv6-lab",
			Namespace:    "lab-ipv6-lab",
			NodeName:     "client-na-ipv6",
			Kind:         "linux",
			Image:        "alpine:latest",
			State:        "Failed",
			IPv4:         "172.18.255.5",
			IPv4Internal: "172.20.20.5/24",
			IPv6:         "N/A",
			IPv6Internal: "N/A",
		},
		{
			Index:        5,
			LabName:      "ipv6-lab",
			Namespace:    "lab-ipv6-lab",
			NodeName:     "router-cidr-fallback",
			Kind:         "nokia_srlinux",
			Image:        "ghcr.io/nokia/srlinux:24.3.1",
			State:        "Running",
			IPv4:         "172.18.255.6",
			IPv4Internal: "172.20.20.6/24",
			IPv6:         "2001:db8:6::1/64",
			IPv6Internal: "",
		},
	}

	out := captureStdout(func() {
		PrintTable(rows)
	})

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")

	// a) Verify column headers contain NO "IPv6 Address" column
	headerLine := lines[3]
	if strings.Contains(headerLine, "IPv6 Address") {
		t.Fatalf("expected NO 'IPv6 Address' column in headers, but found it in: %q", headerLine)
	}
	expectedHeaders := []string{"#", "Name", "Kind", "Image", "State", "IP Address (Ext)", "IP Address (Int)"}
	for _, h := range expectedHeaders {
		if !strings.Contains(headerLine, h) {
			t.Errorf("expected header %q in header line: %q", h, headerLine)
		}
	}

	// Total lines expected:
	// 0: top border
	// 1: title box
	// 2: mid-border
	// 3: headers
	// 4: mid-border
	// 5: router-dual row 1
	// 6: router-dual row 2 (IPv6)
	// 7: client-ipv4-only row 1 (1 row)
	// 8: router-int-ipv6 row 1
	// 9: router-int-ipv6 row 2 (IPv6)
	// 10: client-na-ipv6 row 1 (1 row)
	// 11: router-cidr-fallback row 1
	// 12: router-cidr-fallback row 2 (IPv6)
	// 13: bottom border
	if len(lines) != 14 {
		t.Fatalf("expected exactly 14 lines, got %d:\n%s", len(lines), out)
	}

	// c) router-dual: row 1 has IPv4, row 2 has IPv6 directly under IPv4
	r1Line1 := lines[5]
	r1Line2 := lines[6]
	if !strings.Contains(r1Line1, "router-dual") || !strings.Contains(r1Line1, "172.18.255.1") || !strings.Contains(r1Line1, "172.20.20.2/24") {
		t.Errorf("unexpected router-dual row 1: %q", r1Line1)
	}
	if !strings.Contains(r1Line2, "2001:db8::1") || !strings.Contains(r1Line2, "2001:db8::2/64") {
		t.Errorf("unexpected router-dual row 2: %q", r1Line2)
	}
	// Row 2 should NOT repeat node name, kind, image, or index
	if strings.Contains(r1Line2, "router-dual") || strings.Contains(r1Line2, "nokia_srlinux") {
		t.Errorf("router-dual row 2 should not repeat node metadata: %q", r1Line2)
	}

	// b) client-ipv4-only: takes only 1 row
	r2Line := lines[7]
	if !strings.Contains(r2Line, "client-ipv4-only") || !strings.Contains(r2Line, "172.18.255.3") {
		t.Errorf("unexpected client-ipv4-only row: %q", r2Line)
	}

	// c) router-int-ipv6: row 2 has internal IPv6 directly under IP Address (Int)
	r3Line1 := lines[8]
	r3Line2 := lines[9]
	if !strings.Contains(r3Line1, "router-int-ipv6") || !strings.Contains(r3Line1, "172.18.255.4") {
		t.Errorf("unexpected router-int-ipv6 row 1: %q", r3Line1)
	}
	if !strings.Contains(r3Line2, "2001:db8:4::1/64") {
		t.Errorf("unexpected router-int-ipv6 row 2: %q", r3Line2)
	}

	// b) client-na-ipv6: takes only 1 row
	r4Line := lines[10]
	if !strings.Contains(r4Line, "client-na-ipv6") || !strings.Contains(r4Line, "172.18.255.5") {
		t.Errorf("unexpected client-na-ipv6 row: %q", r4Line)
	}

	// c) router-cidr-fallback: IPv6 CIDR classified as internal
	r5Line1 := lines[11]
	r5Line2 := lines[12]
	if !strings.Contains(r5Line1, "router-cidr-fallback") || !strings.Contains(r5Line1, "172.18.255.6") {
		t.Errorf("unexpected router-cidr-fallback row 1: %q", r5Line1)
	}
	if !strings.Contains(r5Line2, "2001:db8:6::1/64") {
		t.Errorf("unexpected router-cidr-fallback row 2: %q", r5Line2)
	}
}

func TestPrintTableAlignmentWithAndWithoutColor(t *testing.T) {
	rows := []TableRow{
		{
			Index:        1,
			LabName:      "alignment-lab",
			Namespace:    "lab-alignment-lab",
			NodeName:     "long-ip-router",
			Kind:         "nokia_srlinux",
			Image:        "ghcr.io/nokia/srlinux:24.3.1",
			State:        "Running",
			IPv4:         "172.18.255.100",
			IPv4Internal: "172.20.20.100/24",
			IPv6:         "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			IPv6Internal: "2001:0db8:85a3:0000:0000:8a2e:0370:7334/64",
		},
		{
			Index:        2,
			LabName:      "alignment-lab",
			Namespace:    "lab-alignment-lab",
			NodeName:     "normal-client",
			Kind:         "linux",
			Image:        "alpine:latest",
			State:        "Deploying",
			IPv4:         "172.18.255.101",
			IPv4Internal: "172.20.20.101/24",
			IPv6:         "",
			IPv6Internal: "",
		},
	}

	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	// Subtest 1: With Colors
	t.Run("with_color", func(t *testing.T) {
		ResetColors()

		out := captureStdout(func() {
			PrintTable(rows)
		})

		// Verify color escapes are present
		if !strings.Contains(out, "\033[") {
			t.Errorf("expected ANSI escape codes with colors enabled")
		}

		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(lines) == 0 {
			t.Fatal("empty output")
		}

		// Strip ANSI and verify all lines have identical character widths
		expectedWidth := utf8.RuneCountInString(ansiRegex.ReplaceAllString(lines[0], ""))
		for i, line := range lines {
			stripped := ansiRegex.ReplaceAllString(line, "")
			width := utf8.RuneCountInString(stripped)
			if width != expectedWidth {
				t.Errorf("line %d width mismatch with color: got %d, want %d\nLine: %q\nStripped: %q", i, width, expectedWidth, line, stripped)
			}
			if !strings.HasPrefix(stripped, "+") && !strings.HasPrefix(stripped, "|") {
				t.Errorf("line %d does not start with + or |: %q", i, stripped)
			}
			if !strings.HasSuffix(stripped, "+") && !strings.HasSuffix(stripped, "|") {
				t.Errorf("line %d does not end with + or |: %q", i, stripped)
			}
		}
	})

	// Subtest 2: Without Colors
	t.Run("without_color", func(t *testing.T) {
		DisableColors()
		defer ResetColors()

		out := captureStdout(func() {
			PrintTable(rows)
		})

		// Verify NO color escapes are present
		if strings.Contains(out, "\033[") {
			t.Errorf("expected NO ANSI escape codes with colors disabled")
		}

		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(lines) == 0 {
			t.Fatal("empty output")
		}

		expectedWidth := utf8.RuneCountInString(lines[0])
		for i, line := range lines {
			width := utf8.RuneCountInString(line)
			if width != expectedWidth {
				t.Errorf("line %d width mismatch without color: got %d, want %d\nLine: %q", i, width, expectedWidth, line)
			}
			if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "|") {
				t.Errorf("line %d does not start with + or |: %q", i, line)
			}
			if !strings.HasSuffix(line, "+") && !strings.HasSuffix(line, "|") {
				t.Errorf("line %d does not end with + or |: %q", i, line)
			}
		}
	})
}
