// Package diff provides agent output visualization and comparison.
// See every mark the sword leaves on the metal.
package diff

import (
	"fmt"
	"strings"
)

// ChangeType represents the type of change.
type ChangeType int

const (
	ChangeNone   ChangeType = iota // Unchanged
	ChangeAdd                      // Addition
	ChangeDelete                   // Deletion
	ChangeModify                   // Modification
)

// Line represents a single line in a diff.
type Line struct {
	Type    ChangeType
	Content string
	OldLine int
	NewLine int
}

// Diff represents a set of changes.
type Diff struct {
	File     string
	Lines    []Line
	Added    int
	Deleted  int
	Modified int
}

// Summary returns a one-line summary.
func (d *Diff) Summary() string {
	return fmt.Sprintf("%s: +%d -%d ~%d", d.File, d.Added, d.Deleted, d.Modified)
}

// Compare compares two strings and returns a diff.
func Compare(old, new_, file string) *Diff {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new_, "\n")

	d := &Diff{File: file}

	// Simple LCS-based diff
	lcs := longestCommonSubsequence(oldLines, newLines)

	oi, ni := 0, 0
	for _, match := range lcs {
		// Add deletions before this match
		for oi < match.OldIdx {
			d.Lines = append(d.Lines, Line{Type: ChangeDelete, Content: oldLines[oi], OldLine: oi + 1})
			d.Deleted++
			oi++
		}
		// Add insertions before this match
		for ni < match.NewIdx {
			d.Lines = append(d.Lines, Line{Type: ChangeAdd, Content: newLines[ni], NewLine: ni + 1})
			d.Added++
			ni++
		}
		// Add unchanged
		d.Lines = append(d.Lines, Line{Type: ChangeNone, Content: oldLines[oi], OldLine: oi + 1, NewLine: ni + 1})
		oi++
		ni++
	}

	// Remaining deletions
	for oi < len(oldLines) {
		d.Lines = append(d.Lines, Line{Type: ChangeDelete, Content: oldLines[oi], OldLine: oi + 1})
		d.Deleted++
		oi++
	}

	// Remaining insertions
	for ni < len(newLines) {
		d.Lines = append(d.Lines, Line{Type: ChangeAdd, Content: newLines[ni], NewLine: ni + 1})
		d.Added++
		ni++
	}

	return d
}

type match struct {
	OldIdx int
	NewIdx int
}

func longestCommonSubsequence(old, new_ []string) []match {
	m, n := len(old), len(new_)

	// Build DP table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if old[i-1] == new_[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack
	var result []match
	i, j := m, n
	for i > 0 && j > 0 {
		if old[i-1] == new_[j-1] {
			result = append([]match{{OldIdx: i - 1, NewIdx: j - 1}}, result...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return result
}

// FormatColor formats a diff with ANSI color codes.
func FormatColor(d *Diff) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\033[1m--- %s\033[0m\n", d.File))
	b.WriteString(fmt.Sprintf("\033[1m+++ %s\033[0m\n", d.File))

	for _, line := range d.Lines {
		switch line.Type {
		case ChangeAdd:
			b.WriteString(fmt.Sprintf("\033[32m+%s\033[0m\n", line.Content))
		case ChangeDelete:
			b.WriteString(fmt.Sprintf("\033[31m-%s\033[0m\n", line.Content))
		case ChangeModify:
			b.WriteString(fmt.Sprintf("\033[33m~%s\033[0m\n", line.Content))
		default:
			b.WriteString(fmt.Sprintf(" %s\n", line.Content))
		}
	}

	return b.String()
}

// FormatPlain formats a diff without colors.
func FormatPlain(d *Diff) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("--- %s\n", d.File))
	b.WriteString(fmt.Sprintf("+++ %s\n", d.File))

	for _, line := range d.Lines {
		switch line.Type {
		case ChangeAdd:
			b.WriteString(fmt.Sprintf("+%s\n", line.Content))
		case ChangeDelete:
			b.WriteString(fmt.Sprintf("-%s\n", line.Content))
		case ChangeModify:
			b.WriteString(fmt.Sprintf("~%s\n", line.Content))
		default:
			b.WriteString(fmt.Sprintf(" %s\n", line.Content))
		}
	}

	return b.String()
}

// Stats returns diff statistics.
func Stats(d *Diff) string {
	total := d.Added + d.Deleted + d.Modified
	return fmt.Sprintf("%d changes: +%d added, -%d deleted, ~%d modified", total, d.Added, d.Deleted, d.Modified)
}

// Patch generates a unified diff patch.
func Patch(d *Diff) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- a/%s\n", d.File))
	b.WriteString(fmt.Sprintf("+++ b/%s\n", d.File))
	b.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", countOldLines(d), countNewLines(d)))

	for _, line := range d.Lines {
		switch line.Type {
		case ChangeAdd:
			b.WriteString(fmt.Sprintf("+%s\n", line.Content))
		case ChangeDelete:
			b.WriteString(fmt.Sprintf("-%s\n", line.Content))
		default:
			b.WriteString(fmt.Sprintf(" %s\n", line.Content))
		}
	}

	return b.String()
}

func countOldLines(d *Diff) int {
	count := 0
	for _, l := range d.Lines {
		if l.Type != ChangeAdd {
			count++
		}
	}
	return count
}

func countNewLines(d *Diff) int {
	count := 0
	for _, l := range d.Lines {
		if l.Type != ChangeDelete {
			count++
		}
	}
	return count
}

// Apply applies a diff to original content, returning the new content.
func Apply(original string, d *Diff) string {
	var lines []string
	for _, line := range d.Lines {
		if line.Type == ChangeDelete {
			continue
		}
		lines = append(lines, line.Content)
	}
	return strings.Join(lines, "\n")
}

// Reverse returns the inverse diff (undoes the changes).
func Reverse(d *Diff) *Diff {
	rd := &Diff{File: d.File}
	for _, line := range d.Lines {
		switch line.Type {
		case ChangeAdd:
			rd.Lines = append(rd.Lines, Line{Type: ChangeDelete, Content: line.Content, OldLine: line.NewLine})
			rd.Deleted++
		case ChangeDelete:
			rd.Lines = append(rd.Lines, Line{Type: ChangeAdd, Content: line.Content, NewLine: line.OldLine})
			rd.Added++
		default:
			rd.Lines = append(rd.Lines, line)
		}
	}
	return rd
}
