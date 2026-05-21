// Package archaeologist provides AI-powered git forensics.
// It analyzes git history to understand why code was written, detect dead code,
// identify code at risk, and surface historical patterns.
//
// Every line has a story.
package forensics

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// BlameEntry represents a git blame result for a line.
type BlameEntry struct {
	Commit  string `json:"commit"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// CommitInfo represents a git commit.
type CommitInfo struct {
	Hash    string    `json:"hash"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Message string    `json:"message"`
}

// FileHistory represents the history of a file.
type FileHistory struct {
	Path         string       `json:"path"`
	TotalCommits int          `json:"total_commits"`
	Authors      []string     `json:"authors"`
	Commits      []CommitInfo `json:"commits"`
	ChurnScore   float64      `json:"churn_score"`
	LastChanged  time.Time    `json:"last_changed"`
}

// DeadCodeCandidate represents a potential dead code location.
type DeadCodeCandidate struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
	Reason  string `json:"reason"`
}

// Hotspot represents a code area with high risk.
type Hotspot struct {
	File        string  `json:"file"`
	ChurnScore  float64 `json:"churn_score"`
	NumAuthors  int     `json:"num_authors"`
	NumCommits  int     `json:"num_commits"`
	RiskLevel   string  `json:"risk_level"`
	LastChanged string  `json:"last_changed"`
}

// Archaeologist performs git forensics.
type Archaeologist struct {
	repoPath string
}

// New creates a new git archaeologist.
func New(repoPath string) *Archaeologist {
	return &Archaeologist{repoPath: repoPath}
}

// Blame runs git blame on a file.
func (a *Archaeologist) Blame(file string) ([]BlameEntry, error) {
	cmd := exec.Command("git", "blame", "--porcelain", file)
	cmd.Dir = a.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("archaeologist: blame: %w", err)
	}

	var entries []BlameEntry
	var current *BlameEntry
	lineNum := 0

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "author ") {
			if current == nil {
				current = &BlameEntry{}
			}
			current.Author = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "author-mail ") {
			// skip
		} else if strings.HasPrefix(line, "author-time ") {
			if current != nil {
				ts := strings.TrimPrefix(line, "author-time ")
				if unix, err := strconv.ParseInt(ts, 10, 64); err == nil {
					current.Date = time.Unix(unix, 0).Format("2006-01-02")
				}
			}
		} else if strings.HasPrefix(line, "summary ") {
			// skip
		} else if len(line) >= 40 && !strings.Contains(line, " ") && current != nil {
			// Commit hash line
			if current.Commit == "" {
				current.Commit = line[:40]
			}
		} else if strings.HasPrefix(line, "\t") {
			lineNum++
			if current != nil {
				current.Line = lineNum
				current.Content = strings.TrimPrefix(line, "\t")
				entries = append(entries, *current)
				current = &BlameEntry{Commit: current.Commit, Author: current.Author, Date: current.Date}
			}
		}
	}

	return entries, nil
}

// FileLog returns the commit history for a file.
func (a *Archaeologist) FileLog(file string, limit int) (*FileHistory, error) {
	args := []string{"log", "--format=%H|%an|%aI|%s", "--no-merges"}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}
	args = append(args, "--", file)

	cmd := exec.Command("git", args...)
	cmd.Dir = a.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("archaeologist: log: %w", err)
	}

	history := &FileHistory{Path: file}
	authorSet := make(map[string]bool)

	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		date, _ := time.Parse(time.RFC3339, parts[2])
		history.Commits = append(history.Commits, CommitInfo{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    date,
			Message: parts[3],
		})
		authorSet[parts[1]] = true
		history.LastChanged = date
	}

	history.TotalCommits = len(history.Commits)
	for author := range authorSet {
		history.Authors = append(history.Authors, author)
	}

	// Calculate churn score (commits * authors / file age in weeks)
	if history.TotalCommits > 0 {
		ageWeeks := time.Since(history.LastChanged).Hours() / 168
		if ageWeeks < 1 {
			ageWeeks = 1
		}
		history.ChurnScore = float64(history.TotalCommits*len(authorSet)) / ageWeeks
	}

	return history, nil
}

// Hotspots identifies files with high change frequency (high churn = high risk).
func (a *Archaeologist) Hotspots(limit int) ([]Hotspot, error) {
	// Get files sorted by commit count
	cmd := exec.Command("git", "log", "--format=format:", "--name-only", "--no-merges", "-n500")
	cmd.Dir = a.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("archaeologist: hotspots: %w", err)
	}

	fileCommits := make(map[string]int)
	fileAuthors := make(map[string]map[string]bool)

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fileCommits[line]++
		if fileAuthors[line] == nil {
			fileAuthors[line] = make(map[string]bool)
		}
	}

	// Get author info per file
	for file := range fileCommits {
		cmd := exec.Command("git", "log", "--format=%an", "--no-merges", "-n100", "--", file)
		cmd.Dir = a.repoPath
		authors, err := cmd.Output()
		if err != nil {
			continue
		}
		for _, author := range strings.Split(string(authors), "\n") {
			author = strings.TrimSpace(author)
			if author != "" {
				fileAuthors[file][author] = true
			}
		}
	}

	// Build hotspots
	var hotspots []Hotspot
	for file, commits := range fileCommits {
		numAuthors := len(fileAuthors[file])
		churn := float64(commits * numAuthors)

		riskLevel := "low"
		if churn > 50 {
			riskLevel = "high"
		} else if churn > 15 {
			riskLevel = "medium"
		}

		// Get last changed date
		lastChanged := "unknown"
		cmd := exec.Command("git", "log", "-1", "--format=%aI", "--", file)
		cmd.Dir = a.repoPath
		dateOutput, err := cmd.Output()
		if err == nil {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(dateOutput))); err == nil {
				lastChanged = t.Format("2006-01-02")
			}
		}

		hotspots = append(hotspots, Hotspot{
			File:        file,
			ChurnScore:  churn,
			NumAuthors:  numAuthors,
			NumCommits:  commits,
			RiskLevel:   riskLevel,
			LastChanged: lastChanged,
		})
	}

	// Sort by churn score (simple selection)
	for i := 0; i < len(hotspots); i++ {
		for j := i + 1; j < len(hotspots); j++ {
			if hotspots[j].ChurnScore > hotspots[i].ChurnScore {
				hotspots[i], hotspots[j] = hotspots[j], hotspots[i]
			}
		}
	}

	if limit > 0 && len(hotspots) > limit {
		hotspots = hotspots[:limit]
	}

	return hotspots, nil
}

// DeadCode finds potentially dead code (functions/methods not referenced elsewhere).
func (a *Archaeologist) DeadCode(patterns []string) ([]DeadCodeCandidate, error) {
	// Get list of Go files
	cmd := exec.Command("git", "ls-files", "*.go")
	cmd.Dir = a.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("archaeologist: dead code: %w", err)
	}

	var candidates []DeadCodeCandidate
	files := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, file := range files {
		if file == "" {
			continue
		}
		// Look for exported functions
		cmd := exec.Command("grep", "-n", "^func ", file)
		cmd.Dir = a.repoPath
		grepOutput, err := cmd.Output()
		if err != nil {
			continue // No matches
		}

		for _, line := range strings.Split(string(grepOutput), "\n") {
			if line == "" {
				continue
			}
			// Parse "123:func Foo()"
			parts := strings.SplitN(line, ":", 3)
			if len(parts) < 3 {
				continue
			}
			lineNum, _ := strconv.Atoi(parts[0])
			content := parts[2]

			// Extract function name
			funcName := extractFuncName(content)
			if funcName == "" || funcName == "main" || funcName == "init" {
				continue
			}

			// Check if referenced elsewhere (grep for the name)
			refCmd := exec.Command("git", "grep", "-c", funcName, "--", "*.go")
			refCmd.Dir = a.repoPath
			refOutput, err := refCmd.Output()
			if err != nil {
				continue
			}

			// Count references
			refCount := 0
			for _, refLine := range strings.Split(strings.TrimSpace(string(refOutput)), "\n") {
				if strings.Contains(refLine, ":") {
					refCount++
				}
			}

			// If only referenced in 1 file (its own definition), it's potentially dead
			if refCount <= 1 {
				candidates = append(candidates, DeadCodeCandidate{
					File:    file,
					Line:    lineNum,
					Content: strings.TrimSpace(content),
					Reason:  fmt.Sprintf("function %s not referenced outside its file", funcName),
				})
			}
		}
	}

	return candidates, nil
}

// WhyWasThisWritten explains the git history for a specific line.
func (a *Archaeologist) WhyWasThisWritten(file string, line int) (*CommitInfo, error) {
	cmd := exec.Command("git", "blame", "-L", fmt.Sprintf("%d,%d", line, line), "--porcelain", file)
	cmd.Dir = a.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("archaeologist: why: %w", err)
	}

	var commitHash, author, message string
	var date time.Time

	for _, line := range strings.Split(string(output), "\n") {
		if len(line) >= 40 && !strings.Contains(line, " ") && commitHash == "" {
			commitHash = line[:40]
		} else if strings.HasPrefix(line, "author ") {
			author = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "summary ") {
			message = strings.TrimPrefix(line, "summary ")
		} else if strings.HasPrefix(line, "author-time ") {
			ts := strings.TrimPrefix(line, "author-time ")
			if unix, err := strconv.ParseInt(ts, 10, 64); err == nil {
				date = time.Unix(unix, 0)
			}
		}
	}

	return &CommitInfo{
		Hash:    commitHash,
		Author:  author,
		Date:    date,
		Message: message,
	}, nil
}

// extractFuncName extracts a Go function name from a line like "func Foo(...)".
func extractFuncName(line string) string {
	line = strings.TrimPrefix(line, "func ")
	// Receiver method: (r *Type) Method
	if strings.HasPrefix(line, "(") {
		if idx := strings.Index(line, ") "); idx >= 0 {
			line = line[idx+2:]
		}
	}
	// Extract name before (
	if idx := strings.Index(line, "("); idx > 0 {
		return line[:idx]
	}
	if idx := strings.Index(line, " "); idx > 0 {
		return line[:idx]
	}
	return line
}

// FormatBlame renders blame entries for display.
func FormatBlame(entries []BlameEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("%s  %-15s  %s  %4d  %s\n",
			e.Commit[:8], e.Author, e.Date, e.Line, e.Content))
	}
	return sb.String()
}

// FormatHotspot renders a hotspot for display.
func FormatHotspot(h Hotspot) string {
	return fmt.Sprintf("%-50s  churn:%.0f  authors:%d  commits:%d  risk:%s  last:%s",
		h.File, h.ChurnScore, h.NumAuthors, h.NumCommits, h.RiskLevel, h.LastChanged)
}

// FormatDeadCode renders a dead code candidate for display.
func FormatDeadCode(d DeadCodeCandidate) string {
	return fmt.Sprintf("%s:%d  %s  (%s)", d.File, d.Line, d.Content, d.Reason)
}

// FormatHistory renders file history for display.
func FormatHistory(h *FileHistory) string {
	return fmt.Sprintf("%s  commits:%d  authors:%d  churn:%.1f  last:%s",
		h.Path, h.TotalCommits, len(h.Authors), h.ChurnScore, h.LastChanged.Format("2006-01-02"))
}

// MarshalJSON is a helper to JSON-marshal any value.
func MarshalJSON(v interface{}) string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}
