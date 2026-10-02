package ui

import (
	"fmt"
	"time"
)

var startTime = time.Now()

func elapsed() string {
	secs := int(time.Since(startTime).Seconds())
	return fmt.Sprintf("%04d", secs)
}

// Info logs an informational message matching containerlab's log format.
func Info(format string, args ...any) {
	prefix := fmt.Sprintf("%sINFO%s[%s]", ColorBoldCyan, ColorReset, elapsed())
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", prefix, msg)
}

// Warn logs a warning message.
func Warn(format string, args ...any) {
	prefix := fmt.Sprintf("%sWARN%s[%s]", ColorBoldYellow, ColorReset, elapsed())
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", prefix, msg)
}

// Error logs an error message.
func Error(format string, args ...any) {
	prefix := fmt.Sprintf("%sERRO%s[%s]", ColorBoldRed, ColorReset, elapsed())
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", prefix, msg)
}

// Success logs a success message with green highlight.
func Success(format string, args ...any) {
	prefix := fmt.Sprintf("%sINFO%s[%s]", ColorBoldGreen, ColorReset, elapsed())
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", prefix, msg)
}
