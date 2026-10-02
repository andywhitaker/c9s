package ui

import (
	"os"
)

var (
	ColorReset   = "\033[0m"
	ColorBold    = "\033[1m"
	ColorDim     = "\033[2m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"

	// Bold colors
	ColorBoldRed    = "\033[1;31m"
	ColorBoldGreen  = "\033[1;32m"
	ColorBoldYellow = "\033[1;33m"
	ColorBoldCyan   = "\033[1;36m"
	ColorBoldWhite  = "\033[1;37m"

	ColorClabBlue = "\033[38;2;0;201;255m"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		DisableColors()
	}
}

// DisableColors turns off all ANSI color escapes.
func DisableColors() {
	ColorReset = ""
	ColorBold = ""
	ColorDim = ""
	ColorRed = ""
	ColorGreen = ""
	ColorYellow = ""
	ColorBlue = ""
	ColorMagenta = ""
	ColorCyan = ""
	ColorWhite = ""
	ColorBoldRed = ""
	ColorBoldGreen = ""
	ColorBoldYellow = ""
	ColorBoldCyan = ""
	ColorBoldWhite = ""
	ColorClabBlue = ""
}

// ResetColors restores default ANSI colors.
func ResetColors() {
	ColorReset = "\033[0m"
	ColorBold = "\033[1m"
	ColorDim = "\033[2m"
	ColorRed = "\033[31m"
	ColorGreen = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan = "\033[36m"
	ColorWhite = "\033[37m"

	ColorBoldRed = "\033[1;31m"
	ColorBoldGreen = "\033[1;32m"
	ColorBoldYellow = "\033[1;33m"
	ColorBoldCyan = "\033[1;36m"
	ColorBoldWhite = "\033[1;37m"

	ColorClabBlue = "\033[38;2;0;201;255m"
}
