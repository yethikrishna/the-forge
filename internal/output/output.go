// Package output provides a unified output formatting system for all Forge commands.
// Supports JSON, quiet, verbose, and default (human-readable) output modes.
// Ensures no ANSI escape codes in JSON mode and stable schemas for machine parsing.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Format represents the output format mode.
type Format string

const (
	FormatDefault Format = "default" // Human-readable, colored
	FormatJSON    Format = "json"    // Machine-readable JSON
	FormatQuiet   Format = "quiet"   // Minimal output, errors only
	FormatVerbose Format = "verbose" // Maximum detail
)

// OutputManager manages output formatting for a command session.
type OutputManager struct {
	format Format
	stdout io.Writer
	stderr io.Writer
	mu     sync.Mutex
	start  time.Time
}

// New creates a new OutputManager with the given format.
func New(format Format) *OutputManager {
	return &OutputManager{
		format: format,
		stdout: os.Stdout,
		stderr: os.Stderr,
		start:  time.Now(),
	}
}

// NewWithWriters creates an OutputManager with custom writers (for testing).
func NewWithWriters(format Format, stdout, stderr io.Writer) *OutputManager {
	return &OutputManager{
		format: format,
		stdout: stdout,
		stderr: stderr,
		start:  time.Now(),
	}
}

// Format returns the current output format.
func (o *OutputManager) Format() Format {
	return o.format
}

// IsJSON returns true if output is in JSON mode.
func (o *OutputManager) IsJSON() bool {
	return o.format == FormatJSON
}

// IsQuiet returns true if output is in quiet mode.
func (o *OutputManager) IsQuiet() bool {
	return o.format == FormatQuiet
}

// IsVerbose returns true if output is in verbose mode.
func (o *OutputManager) IsVerbose() bool {
	return o.format == FormatVerbose
}

// Print writes a message to stdout in the current format.
func (o *OutputManager) Print(msg string) {
	if o.format == FormatQuiet {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprint(o.stdout, msg)
}

// Println writes a message with newline to stdout.
func (o *OutputManager) Println(msg string) {
	if o.format == FormatQuiet {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintln(o.stdout, msg)
}

// Printf writes a formatted message to stdout.
func (o *OutputManager) Printf(format string, args ...interface{}) {
	if o.format == FormatQuiet {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.stdout, format, args...)
}

// Error writes an error message to stderr.
func (o *OutputManager) Error(msg string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.format == FormatJSON {
		o.writeJSONError(msg)
	} else {
		fmt.Fprintln(o.stderr, "Error: "+msg)
	}
}

// Errorf writes a formatted error message to stderr.
func (o *OutputManager) Errorf(format string, args ...interface{}) {
	o.Error(fmt.Sprintf(format, args...))
}

// Verbose writes a message only in verbose mode.
func (o *OutputManager) Verbose(msg string) {
	if o.format != FormatVerbose {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintln(o.stdout, "[verbose] "+msg)
}

// Verbosef writes a formatted message only in verbose mode.
func (o *OutputManager) Verbosef(format string, args ...interface{}) {
	o.Verbose(fmt.Sprintf(format, args...))
}

// Result outputs a structured result. In JSON mode, it serializes as JSON.
// In other modes, it uses the provided format function.
func (o *OutputManager) Result(data interface{}, humanFn func() string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.format == FormatJSON {
		o.writeJSON(data)
		return
	}

	if o.format == FormatQuiet {
		return
	}

	if humanFn != nil {
		fmt.Fprint(o.stdout, humanFn())
	}
}

// Table outputs tabular data. In JSON mode, outputs the rows as a JSON array.
// In other modes, formats as aligned columns.
func (o *OutputManager) Table(headers []string, rows [][]string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.format == FormatJSON {
		o.writeJSONTable(headers, rows)
		return
	}

	if o.format == FormatQuiet {
		return
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

	// Print header
	for i, h := range headers {
		fmt.Fprintf(o.stdout, "%-*s  ", widths[i], h)
	}
	fmt.Fprintln(o.stdout)

	// Print separator
	for i := range headers {
		fmt.Fprintf(o.stdout, "%s  ", strings.Repeat("-", widths[i]))
	}
	fmt.Fprintln(o.stdout)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			w := ""
			if i < len(widths) {
				w = fmt.Sprintf("%-*s", widths[i], cell)
			} else {
				w = cell
			}
			fmt.Fprintf(o.stdout, "%s  ", w)
		}
		fmt.Fprintln(o.stdout)
	}
}

// List outputs a list of items.
func (o *OutputManager) List(items []string, prefix string) {
	if o.format == FormatQuiet {
		return
	}
	if o.format == FormatJSON {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.writeJSON(items)
		return
	}
	for _, item := range items {
		fmt.Fprintf(o.stdout, "%s%s\n", prefix, item)
	}
}

// Progress outputs a progress indicator. Only shown in verbose or default mode.
func (o *OutputManager) Progress(current, total int, msg string) {
	if o.format == FormatQuiet || o.format == FormatJSON {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	pct := 0
	if total > 0 {
		pct = current * 100 / total
	}
	fmt.Fprintf(o.stdout, "\r[%d/%d] %d%% %s", current, total, pct, msg)
	if current >= total {
		fmt.Fprintln(o.stdout)
	}
}

// Success outputs a success message.
func (o *OutputManager) Success(msg string) {
	if o.format == FormatQuiet {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.format == FormatJSON {
		o.writeJSON(map[string]interface{}{
			"status":  "success",
			"message": msg,
		})
	} else {
		fmt.Fprintf(o.stdout, "✓ %s\n", msg)
	}
}

// Warning outputs a warning message.
func (o *OutputManager) Warning(msg string) {
	if o.format == FormatQuiet {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.format == FormatJSON {
		o.writeJSON(map[string]interface{}{
			"status":  "warning",
			"message": msg,
		})
	} else {
		fmt.Fprintf(o.stderr, "⚠ %s\n", msg)
	}
}

// Header outputs a section header.
func (o *OutputManager) Header(title string) {
	if o.format == FormatQuiet || o.format == FormatJSON {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.stdout, "\n%s\n%s\n", title, strings.Repeat("=", len(title)))
}

// KeyValue outputs key-value pairs.
func (o *OutputManager) KeyValue(pairs map[string]string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.format == FormatJSON {
		o.writeJSON(pairs)
		return
	}

	if o.format == FormatQuiet {
		return
	}

	maxKey := 0
	for k := range pairs {
		if len(k) > maxKey {
			maxKey = len(k)
		}
	}

	for k, v := range pairs {
		fmt.Fprintf(o.stdout, "  %-*s  %s\n", maxKey, k, v)
	}
}

// Duration outputs elapsed time since the OutputManager was created.
func (o *OutputManager) Duration() time.Duration {
	return time.Since(o.start)
}

// Footer outputs a footer with timing information.
func (o *OutputManager) Footer() {
	if o.format == FormatQuiet || o.format == FormatJSON {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.stdout, "\nCompleted in %s\n", o.Duration().Round(time.Millisecond))
}

// writeJSON serializes data as JSON to stdout.
func (o *OutputManager) writeJSON(data interface{}) {
	enc := json.NewEncoder(o.stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		fmt.Fprintf(o.stderr, "json encode error: %v\n", err)
	}
}

// writeJSONError writes a structured JSON error to stderr.
func (o *OutputManager) writeJSONError(msg string) {
	enc := json.NewEncoder(o.stderr)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]interface{}{
		"error":   msg,
		"success": false,
	})
}

// writeJSONTable writes a table as a JSON array of objects.
func (o *OutputManager) writeJSONTable(headers []string, rows [][]string) {
	result := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]string)
		for i, cell := range row {
			if i < len(headers) {
				m[headers[i]] = cell
			}
		}
		result = append(result, m)
	}
	o.writeJSON(result)
}

// ParseFormat converts a string to a Format.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "json", "j":
		return FormatJSON, nil
	case "quiet", "q", "silent":
		return FormatQuiet, nil
	case "verbose", "v", "debug":
		return FormatVerbose, nil
	case "default", "d", "human", "":
		return FormatDefault, nil
	default:
		return FormatDefault, fmt.Errorf("unknown output format: %q (valid: json, quiet, verbose, default)", s)
	}
}

// StripANSI removes ANSI escape sequences from a string.
func StripANSI(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

// CommandResult is a standard result structure for JSON output.
type CommandResult struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Warnings  []string    `json:"warnings,omitempty"`
	Duration  string      `json:"duration,omitempty"`
	Timestamp string      `json:"timestamp"`
}

// NewCommandResult creates a successful CommandResult.
func NewCommandResult(data interface{}) *CommandResult {
	return &CommandResult{
		Success:   true,
		Data:      data,
		Duration:  time.Since(time.Now()).Round(time.Millisecond).String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewCommandError creates a failed CommandResult.
func NewCommandError(err string) *CommandResult {
	return &CommandResult{
		Success:   false,
		Error:     err,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
