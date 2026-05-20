// Package pretty provides terminal styling and color utilities.
// Even a sword deserves to look sharp.
package pretty

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ANSI color codes.
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"
	Blink     = "\033[5m"

	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
)

// ColorFunc applies styling to a string.
type ColorFunc func(string) string

// Color returns a function that wraps text in the given ANSI codes.
func Color(codes ...string) ColorFunc {
	return func(s string) string {
		if !isTerminal() {
			return s
		}
		return strings.Join(codes, "") + s + Reset
	}
}

// Pre-defined color functions.
var (
	RedF     = Color(Red)
	GreenF   = Color(Green)
	YellowF  = Color(Yellow)
	BlueF    = Color(Blue)
	MagentaF = Color(Magenta)
	CyanF    = Color(Cyan)
	BoldF    = Color(Bold)
	DimF     = Color(Dim)
)

// Status-style formatters.
var (
	Success = Color(Green, Bold)   // Green bold for success
	Warning = Color(Yellow, Bold)  // Yellow bold for warnings
	Error   = Color(Red, Bold)     // Red bold for errors
	Info    = Color(Cyan)          // Cyan for info
	Header  = Color(Bold, Cyan)    // Bold cyan for headers
	Muted   = Color(Dim)           // Dim for muted text
)

// Sprint returns a styled string.
func Sprint(style ColorFunc, a ...any) string {
	return style(fmt.Sprint(a...))
}

// Sprintf returns a styled formatted string.
func Sprintf(style ColorFunc, format string, a ...any) string {
	return style(fmt.Sprintf(format, a...))
}

// Fprint writes a styled string to the writer.
func Fprint(w io.Writer, style ColorFunc, a ...any) (int, error) {
	return fmt.Fprint(w, style(fmt.Sprint(a...)))
}

// Fprintf writes a styled formatted string to the writer.
func Fprintf(w io.Writer, style ColorFunc, format string, a ...any) (int, error) {
	return fmt.Fprint(w, style(fmt.Sprintf(format, a...)))
}

// Print writes a styled string to stdout.
func Print(style ColorFunc, a ...any) {
	Fprint(os.Stdout, style, a...)
}

// Printf writes a styled formatted string to stdout.
func Printf(style ColorFunc, format string, a ...any) {
	Fprintf(os.Stdout, style, format, a...)
}

// Status symbols.
const (
	Checkmark = "✓"
	Cross     = "✗"
	Arrow     = "→"
	Bullet    = "•"
	Star      = "★"
	Fire      = "🔥"
	Sword     = "⚔️"
)

// SuccessLine formats a success message.
func SuccessLine(msg string) string {
	return Sprint(Success, Checkmark+" "+msg)
}

// ErrorLine formats an error message.
func ErrorLine(msg string) string {
	return Sprint(Error, Cross+" "+msg)
}

// WarningLine formats a warning message.
func WarningLine(msg string) string {
	return Sprint(Warning, "! "+msg)
}

// InfoLine formats an info message.
func InfoLine(msg string) string {
	return Sprint(Info, Arrow+" "+msg)
}

// HeaderLine formats a section header.
func HeaderLine(msg string) string {
	return Sprint(Header, Sword+" "+msg)
}

// Table helpers.

// Table renders a simple text table.
func Table(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var b strings.Builder

	// Header row
	for i, h := range headers {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(Sprintf(BoldF, "%-*s", widths[i], h))
	}
	b.WriteString("\n")

	// Separator
	for i := range headers {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(strings.Repeat("─", widths[i]))
	}
	b.WriteString("\n")

	// Data rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				b.WriteString("  ")
			}
			if i < len(widths) {
				fmt.Fprintf(&b, "%-*s", widths[i], cell)
			} else {
				b.WriteString(cell)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// isTerminal checks if stdout is a terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Box draws a simple box around text.
func Box(title, content string) string {
	lines := strings.Split(content, "\n")
	maxWidth := len(title)
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	var b strings.Builder
	border := strings.Repeat("─", maxWidth+2)
	fmt.Fprintf(&b, "┌%s┐\n", border)
	fmt.Fprintf(&b, "│ %-*s │\n", maxWidth, Sprint(BoldF, title))
	fmt.Fprintf(&b, "├%s┤\n", border)
	for _, line := range lines {
		fmt.Fprintf(&b, "│ %-*s │\n", maxWidth, line)
	}
	fmt.Fprintf(&b, "└%s┘\n", border)
	return b.String()
}

// ProgressBar renders a simple progress bar.
func ProgressBar(current, total int, width int) string {
	if total == 0 {
		return "[" + strings.Repeat(" ", width) + "]"
	}
	pct := float64(current) / float64(total)
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}

	bar := Sprint(GreenF, strings.Repeat("█", filled)) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %3.0f%%", bar, pct*100)
}
