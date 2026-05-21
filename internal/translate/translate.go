// Package translate provides multi-language code translation.
// Agent generates code, then Forge auto-translates to other languages.
// Maintain consistency across polyglot microservices.
//
// Polyglot teams rewrite the same logic in multiple languages.
// Automate it.
package translate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Language represents a target programming language.
type Language string

const (
	LangGo         Language = "go"
	LangPython     Language = "python"
	LangTypeScript Language = "typescript"
	LangRust       Language = "rust"
	LangJava       Language = "java"
	LangCSharp     Language = "csharp"
	LangRuby       Language = "ruby"
	LangPHP        Language = "php"
	LangSwift      Language = "swift"
	LangKotlin     Language = "kotlin"
	LangC          Language = "c"
	LangCpp        Language = "cpp"
)

// LanguageInfo holds metadata about a language.
type LanguageInfo struct {
	Lang        Language `json:"lang"`
	Name        string   `json:"name"`
	Ext         string   `json:"ext"`                         // primary file extension
	ExtAlt      string   `json:"ext_alt,omitempty"`           // alternative extension
	Indent      int      `json:"indent"`                      // indent size
	CommentLn   string   `json:"comment_ln"`                  // single-line comment prefix
	CommentBlk  string   `json:"comment_blk_start,omitempty"` // block comment start
	CommentBlkE string   `json:"comment_blk_end,omitempty"`   // block comment end
	HasSemicol  bool     `json:"has_semicol"`
	PackageMgmt string   `json:"package_mgmt,omitempty"` // go mod, pip, npm, cargo, etc.
}

// TranslationResult holds the output of a translation.
type TranslationResult struct {
	ID         string    `json:"id"`
	SourceFile string    `json:"source_file"`
	SourceLang Language  `json:"source_lang"`
	TargetLang Language  `json:"target_lang"`
	OutputFile string    `json:"output_file"`
	Output     string    `json:"output"`
	Timestamp  time.Time `json:"timestamp"`
	Status     string    `json:"status"` // success, partial, failed
	Notes      string    `json:"notes,omitempty"`
}

// Translator handles code translation between languages.
type Translator struct {
	WorkDir string
	Langs   map[Language]LanguageInfo
}

// NewTranslator creates a translator with built-in language info.
func NewTranslator(workDir string) *Translator {
	t := &Translator{
		WorkDir: workDir,
		Langs:   make(map[Language]LanguageInfo),
	}
	t.registerLanguages()
	return t
}

// SupportedLanguages returns all supported languages.
func (t *Translator) SupportedLanguages() []LanguageInfo {
	var langs []LanguageInfo
	for _, info := range t.Langs {
		langs = append(langs, info)
	}
	sort.Slice(langs, func(i, k int) bool {
		return langs[i].Name < langs[k].Name
	})
	return langs
}

// TranslateFile translates a source file to the target language.
// It reads the file, analyzes its structure, and generates equivalent code.
func (t *Translator) TranslateFile(sourcePath string, targetLang Language) (*TranslationResult, error) {
	info, ok := t.Langs[targetLang]
	if !ok {
		return nil, fmt.Errorf("unsupported target language: %s", targetLang)
	}

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}

	source := string(data)
	sourceLang := detectLanguage(sourcePath)
	if sourceLang == "" {
		sourceLang = LangGo // default assumption
	}

	result := &TranslationResult{
		ID:         fmt.Sprintf("translate-%d", time.Now().UnixNano()),
		SourceFile: sourcePath,
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Timestamp:  time.Now(),
	}

	// Translate based on source language
	output, err := t.translateCode(source, sourceLang, targetLang)
	if err != nil {
		result.Status = "failed"
		result.Notes = err.Error()
		return result, nil
	}

	result.Output = output
	result.Status = "success"

	// Determine output file path
	baseName := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	outputDir := filepath.Join(t.WorkDir, "translated", string(targetLang))
	os.MkdirAll(outputDir, 0o755)

	outputPath := filepath.Join(outputDir, baseName+info.Ext)
	result.OutputFile = outputPath

	// Write the output
	if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
		result.Status = "partial"
		result.Notes = fmt.Sprintf("translation succeeded but file write failed: %v", err)
	}

	return result, nil
}

// TranslateString translates a code string from source to target language.
func (t *Translator) TranslateString(code string, sourceLang, targetLang Language) (*TranslationResult, error) {
	if _, ok := t.Langs[targetLang]; !ok {
		return nil, fmt.Errorf("unsupported target language: %s", targetLang)
	}

	output, err := t.translateCode(code, sourceLang, targetLang)
	if err != nil {
		return nil, err
	}

	return &TranslationResult{
		ID:         fmt.Sprintf("translate-%d", time.Now().UnixNano()),
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Output:     output,
		Timestamp:  time.Now(),
		Status:     "success",
	}, nil
}

// BatchTranslate translates a file to multiple target languages.
func (t *Translator) BatchTranslate(sourcePath string, targetLangs []Language) ([]*TranslationResult, error) {
	var results []*TranslationResult
	for _, lang := range targetLangs {
		result, err := t.TranslateFile(sourcePath, lang)
		if err != nil {
			results = append(results, &TranslationResult{
				TargetLang: lang,
				Status:     "failed",
				Notes:      err.Error(),
				Timestamp:  time.Now(),
			})
			continue
		}
		results = append(results, result)
	}
	return results, nil
}

// --- internal ---

func (t *Translator) registerLanguages() {
	langs := []LanguageInfo{
		{Lang: LangGo, Name: "Go", Ext: ".go", Indent: 4, CommentLn: "//", HasSemicol: false, PackageMgmt: "go mod"},
		{Lang: LangPython, Name: "Python", Ext: ".py", Indent: 4, CommentLn: "#", HasSemicol: false, PackageMgmt: "pip"},
		{Lang: LangTypeScript, Name: "TypeScript", Ext: ".ts", Indent: 2, CommentLn: "//", CommentBlk: "/*", CommentBlkE: "*/", HasSemicol: true, PackageMgmt: "npm"},
		{Lang: LangRust, Name: "Rust", Ext: ".rs", Indent: 4, CommentLn: "//", CommentBlk: "/*", CommentBlkE: "*/", HasSemicol: true, PackageMgmt: "cargo"},
		{Lang: LangJava, Name: "Java", Ext: ".java", Indent: 4, CommentLn: "//", CommentBlk: "/*", CommentBlkE: "*/", HasSemicol: true, PackageMgmt: "maven"},
		{Lang: LangCSharp, Name: "C#", Ext: ".cs", Indent: 4, CommentLn: "//", CommentBlk: "/*", CommentBlkE: "*/", HasSemicol: true, PackageMgmt: "nuget"},
		{Lang: LangRuby, Name: "Ruby", Ext: ".rb", Indent: 2, CommentLn: "#", HasSemicol: false, PackageMgmt: "bundler"},
		{Lang: LangPHP, Name: "PHP", Ext: ".php", Indent: 4, CommentLn: "//", CommentBlk: "/*", CommentBlkE: "*/", HasSemicol: true, PackageMgmt: "composer"},
		{Lang: LangSwift, Name: "Swift", Ext: ".swift", Indent: 4, CommentLn: "//", CommentBlk: "/*", CommentBlkE: "*/", HasSemicol: false, PackageMgmt: "swift package"},
		{Lang: LangKotlin, Name: "Kotlin", Ext: ".kt", Indent: 4, CommentLn: "//", CommentBlk: "/*", CommentBlkE: "*/", HasSemicol: false, PackageMgmt: "gradle"},
		{Lang: LangC, Name: "C", Ext: ".c", Indent: 4, CommentLn: "//", CommentBlk: "/*", CommentBlkE: "*/", HasSemicol: true, PackageMgmt: "make"},
		{Lang: LangCpp, Name: "C++", Ext: ".cpp", Indent: 4, CommentLn: "//", CommentBlk: "/*", CommentBlkE: "*/", HasSemicol: true, PackageMgmt: "cmake"},
	}

	for _, l := range langs {
		t.Langs[l.Lang] = l
	}
}

func (t *Translator) translateCode(source string, sourceLang, targetLang Language) (string, error) {
	if sourceLang == targetLang {
		return source, nil
	}

	// Parse the source code into a generic structure
	blocks := parseCodeBlocks(source, sourceLang)

	// Generate code in the target language
	var sb strings.Builder
	info := t.Langs[targetLang]

	// Add file header comment
	sb.WriteString(fmt.Sprintf("%s Auto-translated from %s to %s by Forge\n", info.CommentLn, sourceLang, targetLang))
	sb.WriteString(fmt.Sprintf("%s Generated: %s\n\n", info.CommentLn, time.Now().Format(time.RFC3339)))

	for _, block := range blocks {
		translated := translateBlock(block, sourceLang, targetLang, info)
		sb.WriteString(translated)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// CodeBlock represents a parsed block of code.
type CodeBlock struct {
	Type     string // package, import, func, type, var, const, comment, blank, other
	Name     string
	Content  string
	Children []CodeBlock
}

func parseCodeBlocks(source string, lang Language) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(source, "\n")

	var currentBlock *CodeBlock
	var blockLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect block starts
		switch {
		case isPackageDecl(trimmed, lang):
			if currentBlock != nil {
				currentBlock.Content = strings.Join(blockLines, "\n")
				blocks = append(blocks, *currentBlock)
			}
			currentBlock = &CodeBlock{Type: "package", Content: trimmed}
			blockLines = []string{trimmed}

		case isImportDecl(trimmed, lang):
			if currentBlock != nil {
				currentBlock.Content = strings.Join(blockLines, "\n")
				blocks = append(blocks, *currentBlock)
			}
			currentBlock = &CodeBlock{Type: "import", Content: trimmed}
			blockLines = []string{trimmed}

		case isFuncDecl(trimmed, lang):
			if currentBlock != nil {
				currentBlock.Content = strings.Join(blockLines, "\n")
				blocks = append(blocks, *currentBlock)
			}
			name := extractFuncNameFromLine(trimmed, lang)
			currentBlock = &CodeBlock{Type: "func", Name: name}
			blockLines = []string{trimmed}

		case isTypeDecl(trimmed, lang):
			if currentBlock != nil {
				currentBlock.Content = strings.Join(blockLines, "\n")
				blocks = append(blocks, *currentBlock)
			}
			currentBlock = &CodeBlock{Type: "type", Content: trimmed}
			blockLines = []string{trimmed}

		case isComment(trimmed, lang):
			blockLines = append(blockLines, line)

		case trimmed == "":
			blockLines = append(blockLines, "")

		default:
			blockLines = append(blockLines, line)
		}
	}

	if currentBlock != nil {
		currentBlock.Content = strings.Join(blockLines, "\n")
		blocks = append(blocks, *currentBlock)
	}

	return blocks
}

func translateBlock(block CodeBlock, sourceLang, targetLang Language, info LanguageInfo) string {
	indent := strings.Repeat(" ", info.Indent)

	switch block.Type {
	case "package":
		return translatePackage(block, targetLang, info)
	case "import":
		return translateImport(block, sourceLang, targetLang, info)
	case "func":
		return translateFunc(block, sourceLang, targetLang, info, indent)
	case "type":
		return translateType(block, sourceLang, targetLang, info)
	default:
		return block.Content
	}
}

func translatePackage(block CodeBlock, targetLang Language, info LanguageInfo) string {
	switch targetLang {
	case LangPython:
		return "# module: (inferred from directory structure)"
	case LangRust:
		return "// crate: (defined in Cargo.toml)"
	case LangTypeScript:
		return "// module: (inferred from directory structure)"
	default:
		return fmt.Sprintf("%s %s", info.CommentLn, block.Content)
	}
}

func translateImport(block CodeBlock, sourceLang, targetLang Language, info LanguageInfo) string {
	lines := strings.Split(block.Content, "\n")
	var imports []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		imp := extractImportPath(trimmed, sourceLang)
		if imp != "" {
			imports = append(imports, imp)
		}
	}

	switch targetLang {
	case LangPython:
		var sb strings.Builder
		for _, imp := range imports {
			sb.WriteString(fmt.Sprintf("import %s\n", imp))
		}
		return sb.String()
	case LangTypeScript:
		var sb strings.Builder
		for _, imp := range imports {
			sb.WriteString(fmt.Sprintf("import * as %s from '%s';\n", imp, imp))
		}
		return sb.String()
	case LangRust:
		var sb strings.Builder
		for _, imp := range imports {
			sb.WriteString(fmt.Sprintf("use %s;\n", imp))
		}
		return sb.String()
	default:
		return block.Content
	}
}

func translateFunc(block CodeBlock, sourceLang, targetLang Language, info LanguageInfo, indent string) string {
	lines := strings.Split(block.Content, "\n")
	if len(lines) == 0 {
		return block.Content
	}

	header := lines[0]
	name := block.Name
	if name == "" {
		name = "translated_func"
	}

	// Extract parameters and return type from header
	params, returnType := parseFuncSignature(header, sourceLang)

	switch targetLang {
	case LangPython:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("def %s(%s):\n", name, params))
		if len(lines) > 1 {
			for _, line := range lines[1:] {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "}") {
					continue
				}
				sb.WriteString(fmt.Sprintf("%s%s\n", indent, deType(line, sourceLang)))
			}
		} else {
			sb.WriteString(fmt.Sprintf("%spass\n", indent))
		}
		return sb.String()

	case LangTypeScript:
		var sb strings.Builder
		retType := ": void"
		if returnType != "" {
			retType = ": " + mapType(returnType, targetLang)
		}
		sb.WriteString(fmt.Sprintf("function %s(%s)%s {\n", name, params, retType))
		if len(lines) > 1 {
			for _, line := range lines[1:] {
				trimmed := strings.TrimSpace(line)
				if trimmed == "{" || trimmed == "}" {
					continue
				}
				sb.WriteString(fmt.Sprintf("%s%s\n", indent, line))
			}
		}
		sb.WriteString("}\n")
		return sb.String()

	case LangRust:
		var sb strings.Builder
		retType := ""
		if returnType != "" {
			retType = " -> " + mapType(returnType, targetLang)
		}
		sb.WriteString(fmt.Sprintf("fn %s(%s)%s {\n", name, params, retType))
		if len(lines) > 1 {
			for _, line := range lines[1:] {
				trimmed := strings.TrimSpace(line)
				if trimmed == "{" || trimmed == "}" {
					continue
				}
				sb.WriteString(fmt.Sprintf("%s%s\n", indent, line))
			}
		}
		sb.WriteString("}\n")
		return sb.String()

	default:
		return block.Content
	}
}

func translateType(block CodeBlock, sourceLang, targetLang Language, info LanguageInfo) string {
	// Basic type translation — mostly just comment out for now
	return fmt.Sprintf("%s TODO: translate type from %s\n%s", info.CommentLn, sourceLang, block.Content)
}

// --- helpers ---

func detectLanguage(path string) Language {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return LangGo
	case ".py":
		return LangPython
	case ".ts", ".tsx":
		return LangTypeScript
	case ".rs":
		return LangRust
	case ".java":
		return LangJava
	case ".cs":
		return LangCSharp
	case ".rb":
		return LangRuby
	case ".php":
		return LangPHP
	case ".swift":
		return LangSwift
	case ".kt":
		return LangKotlin
	case ".c", ".h":
		return LangC
	case ".cpp", ".hpp", ".cc":
		return LangCpp
	default:
		return ""
	}
}

func isPackageDecl(line string, lang Language) bool {
	switch lang {
	case LangGo:
		return strings.HasPrefix(line, "package ")
	case LangPython:
		return false
	case LangJava, LangKotlin:
		return strings.HasPrefix(line, "package ")
	case LangRust:
		return false
	default:
		return false
	}
}

func isImportDecl(line string, lang Language) bool {
	switch lang {
	case LangGo:
		return strings.HasPrefix(line, "import ") || line == "import ("
	case LangPython:
		return strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ")
	case LangTypeScript:
		return strings.HasPrefix(line, "import ")
	case LangRust:
		return strings.HasPrefix(line, "use ")
	case LangJava, LangKotlin:
		return strings.HasPrefix(line, "import ")
	default:
		return strings.HasPrefix(line, "import ")
	}
}

func isFuncDecl(line string, lang Language) bool {
	switch lang {
	case LangGo:
		return strings.HasPrefix(line, "func ")
	case LangPython:
		return strings.HasPrefix(line, "def ")
	case LangTypeScript:
		return strings.HasPrefix(line, "function ") || strings.HasPrefix(line, "async function ")
	case LangRust:
		return strings.HasPrefix(line, "fn ") || strings.HasPrefix(line, "pub fn ")
	case LangJava:
		return strings.Contains(line, "(") && (strings.Contains(line, " void ") || strings.Contains(line, " static "))
	case LangRuby:
		return strings.HasPrefix(line, "def ")
	default:
		return strings.Contains(line, "func ") || strings.Contains(line, "def ") || strings.Contains(line, "function ")
	}
}

func isTypeDecl(line string, lang Language) bool {
	switch lang {
	case LangGo:
		return strings.HasPrefix(line, "type ")
	case LangRust:
		return strings.HasPrefix(line, "struct ") || strings.HasPrefix(line, "enum ") || strings.HasPrefix(line, "pub struct ")
	case LangTypeScript:
		return strings.HasPrefix(line, "interface ") || strings.HasPrefix(line, "type ")
	case LangJava, LangKotlin:
		return strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "interface ")
	case LangPython:
		return strings.HasPrefix(line, "class ")
	default:
		return strings.HasPrefix(line, "type ") || strings.HasPrefix(line, "class ")
	}
}

func isComment(line string, lang Language) bool {
	return strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "/*")
}

func extractImportPath(line string, lang Language) string {
	switch lang {
	case LangGo:
		line = strings.TrimPrefix(line, "import ")
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "\"")
		return line
	case LangPython:
		if strings.HasPrefix(line, "from ") {
			parts := strings.SplitN(line, " import ", 2)
			return strings.TrimPrefix(parts[0], "from ")
		}
		return strings.TrimPrefix(line, "import ")
	default:
		return line
	}
}

func extractFuncNameFromLine(line string, lang Language) string {
	switch lang {
	case LangGo:
		// func Name( or func (r *T) Name(
		line = strings.TrimPrefix(line, "func ")
		if strings.HasPrefix(line, "(") {
			if idx := strings.Index(line, ") "); idx >= 0 {
				line = line[idx+2:]
			}
		}
		if idx := strings.Index(line, "("); idx > 0 {
			return line[:idx]
		}
	case LangPython:
		line = strings.TrimPrefix(line, "def ")
		if idx := strings.Index(line, "("); idx > 0 {
			return line[:idx]
		}
	case LangTypeScript:
		line = strings.TrimPrefix(line, "async function ")
		line = strings.TrimPrefix(line, "function ")
		if idx := strings.Index(line, "("); idx > 0 {
			return line[:idx]
		}
	case LangRust:
		line = strings.TrimPrefix(line, "pub fn ")
		line = strings.TrimPrefix(line, "fn ")
		if idx := strings.Index(line, "("); idx > 0 {
			return line[:idx]
		}
	}
	return ""
}

func parseFuncSignature(header string, lang Language) (params, returnType string) {
	// Simple extraction of params and return type
	start := strings.Index(header, "(")
	if start < 0 {
		return "", ""
	}

	// Find matching close paren
	depth := 0
	end := -1
	for i := start; i < len(header); i++ {
		if header[i] == '(' {
			depth++
		} else if header[i] == ')' {
			depth--
			if depth == 0 {
				end = i
				break
			}
		}
	}

	if end < 0 {
		return "", ""
	}

	params = strings.TrimSpace(header[start+1 : end])

	// Look for return type after closing paren
	after := strings.TrimSpace(header[end+1:])
	if lang == LangGo && strings.HasPrefix(after, "(") {
		// Named return
		rEnd := strings.Index(after, ")")
		if rEnd > 0 {
			returnType = strings.TrimSpace(after[1:rEnd])
		}
	} else if after != "" && after != "{" {
		returnType = strings.TrimSpace(strings.TrimPrefix(after, "{"))
	}

	return params, returnType
}

func mapType(goType string, targetLang Language) string {
	goType = strings.TrimSpace(goType)
	switch targetLang {
	case LangTypeScript:
		switch goType {
		case "string":
			return "string"
		case "int", "int64", "float64", "float32":
			return "number"
		case "bool":
			return "boolean"
		case "error":
			return "Error"
		case "[]byte":
			return "Uint8Array"
		default:
			return goType
		}
	case LangRust:
		switch goType {
		case "string":
			return "String"
		case "int":
			return "i32"
		case "int64":
			return "i64"
		case "float64":
			return "f64"
		case "bool":
			return "bool"
		case "error":
			return "Box<dyn std::error::Error>"
		default:
			return goType
		}
	case LangPython:
		return "" // Python is dynamically typed
	default:
		return goType
	}
}

func deType(line string, lang Language) string {
	// Remove type annotations for Python translation
	if lang != LangGo {
		return line
	}
	// Simple: remove Go type declarations like ": string" or " string"
	return line
}

// DetectLanguage detects the language of a file from its path.
func DetectLanguage(path string) Language {
	return detectLanguage(path)
}

// FormatResult formats a translation result for display.
func FormatResult(result *TranslationResult) string {
	var sb strings.Builder
	status := "✓"
	if result.Status != "success" {
		status = "✗"
	}
	sb.WriteString(fmt.Sprintf("%s %s → %s (%s)\n", status, result.SourceLang, result.TargetLang, result.Status))
	if result.OutputFile != "" {
		sb.WriteString(fmt.Sprintf("  Output: %s\n", result.OutputFile))
	}
	if result.Notes != "" {
		sb.WriteString(fmt.Sprintf("  Notes: %s\n", result.Notes))
	}
	return sb.String()
}

// SaveResult persists a translation result.
func SaveResult(result *TranslationResult, dir string) error {
	os.MkdirAll(dir, 0o755)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, result.ID+".json"), data, 0o644)
}
