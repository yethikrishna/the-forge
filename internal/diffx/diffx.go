// Package diffx provides semantic code diffing.
// Unlike standard diff, diffx understands code structure:
//   - Detects moved functions (not just delete+add)
//   - Identifies renamed variables
//   - Recognizes reformatted code vs actual changes
//   - Groups changes by logical unit (function, class, struct)
//   - Produces structured diff output for agent consumption
//
// See the change, not just the characters.
package diffx

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ChangeType classifies a semantic change.
type ChangeType string

const (
	ChangeAdded      ChangeType = "added"
	ChangeRemoved    ChangeType = "removed"
	ChangeModified   ChangeType = "modified"
	ChangeMoved      ChangeType = "moved"
	ChangeRenamed    ChangeType = "renamed"
	ChangeReformatted ChangeType = "reformatted"
	ChangeUnchanged  ChangeType = "unchanged"
)

// Language represents a programming language.
type Language string

const (
	LangGo         Language = "go"
	LangPython     Language = "python"
	LangTypeScript Language = "typescript"
	LangRust       Language = "rust"
	LangJava       Language = "java"
	LangUnknown    Language = "unknown"
)

// CodeBlock represents a logical code unit.
type CodeBlock struct {
	Type     string   `json:"type"` // "function", "method", "struct", "interface", "class", "import", "const", "var"
	Name     string   `json:"name"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Content  string   `json:"content"`
	Language Language `json:"language"`
}

// SemanticChange represents a single semantic change.
type SemanticChange struct {
	Type      ChangeType `json:"type"`
	BlockType string     `json:"block_type"`
	Name      string     `json:"name"`
	OldBlock  *CodeBlock `json:"old_block,omitempty"`
	NewBlock  *CodeBlock `json:"new_block,omitempty"`
	Summary   string     `json:"summary"`
}

// DiffResult holds the complete semantic diff.
type DiffResult struct {
	Language Language          `json:"language"`
	OldFile  string            `json:"old_file"`
	NewFile  string            `json:"new_file"`
	Changes  []SemanticChange  `json:"changes"`
	Stats    DiffStats         `json:"stats"`
}

// DiffStats holds diff statistics.
type DiffStats struct {
	Added       int `json:"added"`
	Removed     int `json:"removed"`
	Modified    int `json:"modified"`
	Moved       int `json:"moved"`
	Renamed     int `json:"renamed"`
	Reformatted int `json:"reformatted"`
	Unchanged   int `json:"unchanged"`
}

// Differ performs semantic code diffing.
type Differ struct {
	lang Language
}

// NewDiffer creates a new semantic differ.
func NewDiffer(lang Language) *Differ {
	if lang == "" {
		lang = LangUnknown
	}
	return &Differ{lang: lang}
}

// DetectLanguage detects the programming language from file extension.
func DetectLanguage(filename string) Language {
	ext := ""
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		ext = filename[idx:]
	}

	switch ext {
	case ".go":
		return LangGo
	case ".py":
		return LangPython
	case ".ts", ".tsx", ".js", ".jsx":
		return LangTypeScript
	case ".rs":
		return LangRust
	case ".java":
		return LangJava
	default:
		return LangUnknown
	}
}

// Diff computes the semantic diff between two file contents.
func (d *Differ) Diff(oldContent, newContent string) *DiffResult {
	if d.lang == LangUnknown {
		d.lang = LangGo // default
	}

	result := &DiffResult{
		Language: d.lang,
	}

	oldBlocks := d.parse(oldContent)
	newBlocks := d.parse(newContent)

	// Build maps by name
	oldMap := make(map[string]*CodeBlock)
	for i := range oldBlocks {
		oldMap[oldBlocks[i].Name] = &oldBlocks[i]
	}
	newMap := make(map[string]*CodeBlock)
	for i := range newBlocks {
		newMap[newBlocks[i].Name] = &newBlocks[i]
	}

	// Find changes
	for name, newBlock := range newMap {
		if oldBlock, ok := oldMap[name]; ok {
			if oldBlock.Content == newBlock.Content {
				if oldBlock.StartLine != newBlock.StartLine {
					result.Changes = append(result.Changes, SemanticChange{
						Type:      ChangeMoved,
						BlockType: newBlock.Type,
						Name:      name,
						OldBlock:  oldBlock,
						NewBlock:  newBlock,
						Summary:   fmt.Sprintf("%s %s moved from line %d to %d", newBlock.Type, name, oldBlock.StartLine, newBlock.StartLine),
					})
				} else {
					result.Stats.Unchanged++
				}
			} else {
				changeType := ChangeModified
				summary := fmt.Sprintf("%s %s modified", newBlock.Type, name)

				// Check if it's just reformatting
				if isReformat(oldBlock.Content, newBlock.Content) {
					changeType = ChangeReformatted
					summary = fmt.Sprintf("%s %s reformatted", newBlock.Type, name)
				}

				result.Changes = append(result.Changes, SemanticChange{
					Type:      changeType,
					BlockType: newBlock.Type,
					Name:      name,
					OldBlock:  oldBlock,
					NewBlock:  newBlock,
					Summary:   summary,
				})
			}
		} else {
			// Check for renames (same type, similar content)
			renamed := false
			for oldName, oldBlock := range oldMap {
				if _, exists := newMap[oldName]; exists {
					continue
				}
				if oldBlock.Type == newBlock.Type && similarity(oldBlock.Content, newBlock.Content) > 0.6 {
					result.Changes = append(result.Changes, SemanticChange{
						Type:      ChangeRenamed,
						BlockType: newBlock.Type,
						Name:      fmt.Sprintf("%s → %s", oldName, name),
						OldBlock:  oldBlock,
						NewBlock:  newBlock,
						Summary:   fmt.Sprintf("%s %s renamed to %s", newBlock.Type, oldName, name),
					})
					renamed = true
					break
				}
			}
			if !renamed {
				result.Changes = append(result.Changes, SemanticChange{
					Type:      ChangeAdded,
					BlockType: newBlock.Type,
					Name:      name,
					NewBlock:  newBlock,
					Summary:   fmt.Sprintf("%s %s added at line %d", newBlock.Type, name, newBlock.StartLine),
				})
			}
		}
	}

	// Find removed
	for name, oldBlock := range oldMap {
		if _, ok := newMap[name]; !ok {
			// Check if it was renamed (already handled above)
			wasRenamed := false
			for _, change := range result.Changes {
				if change.Type == ChangeRenamed && strings.HasPrefix(change.Name, name+" →") {
					wasRenamed = true
					break
				}
			}
			if !wasRenamed {
				result.Changes = append(result.Changes, SemanticChange{
					Type:      ChangeRemoved,
					BlockType: oldBlock.Type,
					Name:      name,
					OldBlock:  oldBlock,
					Summary:   fmt.Sprintf("%s %s removed", oldBlock.Type, name),
				})
			}
		}
	}

	// Sort changes by line number
	sort.Slice(result.Changes, func(i, j int) bool {
		getLine := func(c SemanticChange) int {
			if c.NewBlock != nil {
				return c.NewBlock.StartLine
			}
			if c.OldBlock != nil {
				return c.OldBlock.StartLine
			}
			return 0
		}
		return getLine(result.Changes[i]) < getLine(result.Changes[j])
	})

	// Update stats
	for _, c := range result.Changes {
		switch c.Type {
		case ChangeAdded:
			result.Stats.Added++
		case ChangeRemoved:
			result.Stats.Removed++
		case ChangeModified:
			result.Stats.Modified++
		case ChangeMoved:
			result.Stats.Moved++
		case ChangeRenamed:
			result.Stats.Renamed++
		case ChangeReformatted:
			result.Stats.Reformatted++
		}
	}

	return result
}

// parse extracts code blocks from source code.
func (d *Differ) parse(content string) []CodeBlock {
	switch d.lang {
	case LangGo:
		return parseGo(content)
	default:
		return parseGeneric(content)
	}
}

// parseGo parses Go source code into blocks.
func parseGo(content string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(content, "\n")

	// Regex patterns for Go
	funcRe := regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?(\w+)`)
	structRe := regexp.MustCompile(`^type\s+(\w+)\s+struct`)
	interfaceRe := regexp.MustCompile(`^type\s+(\w+)\s+interface`)
	importRe := regexp.MustCompile(`^import\s`)
	constRe := regexp.MustCompile(`^const\s`)
	varRe := regexp.MustCompile(`^var\s`)

	inBlock := false
	blockType := ""
	blockName := ""
	blockStart := 0
	braceDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inBlock {
			if matches := funcRe.FindStringSubmatch(trimmed); matches != nil {
				blockType = "function"
				blockName = matches[1]
				blockStart = i + 1
				inBlock = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
				if braceDepth <= 0 {
					// Single-line function
					blocks = append(blocks, CodeBlock{
						Type:      blockType,
						Name:      blockName,
						StartLine: blockStart,
						EndLine:   i + 1,
						Content:   line,
						Language:  LangGo,
					})
					inBlock = false
				}
				continue
			}
			if matches := structRe.FindStringSubmatch(trimmed); matches != nil {
				blockType = "struct"
				blockName = matches[1]
				blockStart = i + 1
				inBlock = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
				continue
			}
			if matches := interfaceRe.FindStringSubmatch(trimmed); matches != nil {
				blockType = "interface"
				blockName = matches[1]
				blockStart = i + 1
				inBlock = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
				continue
			}
			if importRe.MatchString(trimmed) {
				blockType = "import"
				blockName = "imports"
				blockStart = i + 1
				if strings.Contains(line, "(") {
					inBlock = true
					braceDepth = 1
				} else {
					blocks = append(blocks, CodeBlock{
						Type:      blockType, Name: blockName,
						StartLine: blockStart, EndLine: i + 1,
						Content: line, Language: LangGo,
					})
				}
				continue
			}
			if constRe.MatchString(trimmed) {
				blocks = append(blocks, CodeBlock{
					Type: "const", Name: "constants",
					StartLine: i + 1, EndLine: i + 1,
					Content: line, Language: LangGo,
				})
				continue
			}
			if varRe.MatchString(trimmed) {
				blocks = append(blocks, CodeBlock{
					Type: "var", Name: "variables",
					StartLine: i + 1, EndLine: i + 1,
					Content: line, Language: LangGo,
				})
				continue
			}
		} else {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 {
				blocks = append(blocks, CodeBlock{
					Type:      blockType,
					Name:      blockName,
					StartLine: blockStart,
					EndLine:   i + 1,
					Content:   strings.Join(lines[blockStart-1:i+1], "\n"),
					Language:  LangGo,
				})
				inBlock = false
			}
		}
	}

	return blocks
}

// parseGeneric does simple function-level parsing for unknown languages.
func parseGeneric(content string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(content, "\n")

	funcRe := regexp.MustCompile(`^func\s+(\w+)|^def\s+(\w+)|^fn\s+(\w+)|^function\s+(\w+)`)

	inBlock := false
	blockName := ""
	blockStart := 0
	braceDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inBlock {
			if matches := funcRe.FindStringSubmatch(trimmed); matches != nil {
				for _, m := range matches[1:] {
					if m != "" {
						blockName = m
						break
					}
				}
				blockStart = i + 1
				inBlock = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
				continue
			}
		} else {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 {
				blocks = append(blocks, CodeBlock{
					Type:      "function",
					Name:      blockName,
					StartLine: blockStart,
					EndLine:   i + 1,
					Content:   strings.Join(lines[blockStart-1:i+1], "\n"),
				})
				inBlock = false
			}
		}
	}

	return blocks
}

// isReformat checks if the change is just whitespace/formatting.
func isReformat(old, new string) bool {
	normOld := normalizeWhitespace(old)
	normNew := normalizeWhitespace(new)
	return normOld == normNew
}

func normalizeWhitespace(s string) string {
	// Collapse multiple spaces/tabs into single space, trim lines
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			// Collapse internal whitespace
			re := regexp.MustCompile(`\s+`)
			trimmed = re.ReplaceAllString(trimmed, " ")
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}

// similarity computes a simple similarity score between two strings.
func similarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	// Simple Jaccard similarity on words
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)

	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[w] = true
	}
	setB := make(map[string]bool)
	for _, w := range wordsB {
		setB[w] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

// RenderDiff renders a semantic diff result for display.
func RenderDiff(result *DiffResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Semantic Diff: %s (%s)\n", result.Language, result.OldFile)
	fmt.Fprintf(&b, "Changes: %d added, %d removed, %d modified, %d moved, %d renamed, %d reformatted\n",
		result.Stats.Added, result.Stats.Removed, result.Stats.Modified,
		result.Stats.Moved, result.Stats.Renamed, result.Stats.Reformatted)

	for _, change := range result.Changes {
		icon := "~"
		switch change.Type {
		case ChangeAdded:
			icon = "+"
		case ChangeRemoved:
			icon = "-"
		case ChangeModified:
			icon = "~"
		case ChangeMoved:
			icon = "→"
		case ChangeRenamed:
			icon = "✎"
		case ChangeReformatted:
			icon = "±"
		}
		fmt.Fprintf(&b, "  %s %s\n", icon, change.Summary)
	}

	return b.String()
}
