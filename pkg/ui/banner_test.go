package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestBannerDimensions(t *testing.T) {
	if len(bannerCLAB) != 4 {
		t.Fatalf("expected 4 rows for bannerCLAB, got %d", len(bannerCLAB))
	}
	if len(bannerErnetes) != 4 {
		t.Fatalf("expected 4 rows for bannerErnetes, got %d", len(bannerErnetes))
	}

	for i := 0; i < 4; i++ {
		clabRunes := len([]rune(bannerCLAB[i]))
		if clabRunes != 26 {
			t.Errorf("bannerCLAB row %d expected 26 columns, got %d", i, clabRunes)
		}

		ernetesRunes := len([]rune(bannerErnetes[i]))
		if ernetesRunes != 48 {
			t.Errorf("bannerErnetes row %d expected 48 columns, got %d", i, ernetesRunes)
		}

		combinedRunes := len([]rune(bannerCLAB[i] + bannerErnetes[i]))
		if combinedRunes != 74 {
			t.Errorf("combined banner row %d expected 74 columns, got %d", i, combinedRunes)
		}
	}
}

func TestBrailleBanner(t *testing.T) {
	rows := strings.Split(BrailleBanner, "\n")
	if len(rows) != 4 {
		t.Fatalf("expected 4 lines in BrailleBanner, got %d", len(rows))
	}

	for i := 0; i < 4; i++ {
		expectedRow := bannerCLAB[i] + bannerErnetes[i]
		if rows[i] != expectedRow {
			t.Errorf("row %d mismatch:\ngot:  %q\nwant: %q", i, rows[i], expectedRow)
		}
		if len([]rune(rows[i])) != 74 {
			t.Errorf("BrailleBanner row %d expected 74 columns, got %d", i, len([]rune(rows[i])))
		}
	}
}

func TestColorClabBlue(t *testing.T) {
	expected := "\033[38;2;0;201;255m"
	if ColorClabBlue != expected {
		t.Errorf("expected ColorClabBlue to be %q, got %q", expected, ColorClabBlue)
	}

	DisableColors()
	if ColorClabBlue != "" {
		t.Errorf("expected ColorClabBlue to be empty after DisableColors, got %q", ColorClabBlue)
	}

	// Restore ColorClabBlue and other colors for subsequent tests
	ColorClabBlue = expected
	ColorWhite = "\033[37m"
	ColorReset = "\033[0m"
}

func TestPrintBanner(t *testing.T) {
	// Test standard banner output
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	PrintBanner()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 printed lines from PrintBanner, got %d: %q", len(lines), output)
	}

	// Test C9S_NO_BANNER suppression
	t.Setenv("C9S_NO_BANNER", "1")

	r2, w2, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w2

	PrintBanner()

	w2.Close()
	os.Stdout = oldStdout

	var buf2 bytes.Buffer
	_, _ = io.Copy(&buf2, r2)
	if buf2.Len() != 0 {
		t.Errorf("expected no output when C9S_NO_BANNER is set, got: %q", buf2.String())
	}
}
