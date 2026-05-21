package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/pretty"
)

// fixResult tracks an auto-fix attempt.
type fixResult struct {
	checkMsg  string
	fixDesc   string
	applied   bool
	err       error
	manualMsg string // What to do if auto-fix failed
}

// attemptFixes tries to auto-repair issues detected by doctor checks.
func attemptFixes(results []checkResult) []fixResult {
	var fixes []fixResult

	for _, r := range results {
		if r.status == statusPass {
			continue
		}

		// Try to match known fixable patterns.
		fix := tryFix(r)
		if fix != nil {
			fixes = append(fixes, *fix)
		}
	}

	return fixes
}

func tryFix(r checkResult) *fixResult {
	msg := strings.ToLower(r.message)

	// Fix: No Forgefile → create minimal one.
	if strings.Contains(msg, "no forgefile found") {
		return fixCreateForgefile(r)
	}

	// Fix: .forge directory not found → create it.
	if strings.Contains(msg, ".forge directory") && strings.Contains(msg, "not found") {
		return fixCreateForgeDir(r)
	}

	// Fix: Missing [project] section in Forgefile.
	if strings.Contains(msg, "missing [project] section") {
		return fixAddForgefileSection(r, "project")
	}

	// Fix: Missing [agent] section in Forgefile.
	if strings.Contains(msg, "missing [agent] section") {
		return fixAddForgefileSection(r, "agent")
	}

	// Fix: Not in a git repo → init one.
	if strings.Contains(msg, "not inside a git repository") {
		return fixGitInit(r)
	}

	// Fix: No .forge/genealogy, .forge/consent, etc. dirs → create them.
	if strings.Contains(msg, "directory") && strings.Contains(msg, "not found") {
		// Generic directory creation — only if it looks like a .forge subdirectory.
		if strings.Contains(r.hint, "forge") {
			return fixCreateDir(r, ".forge")
		}
	}

	return nil
}

func fixCreateForgefile(r checkResult) *fixResult {
	wd, _ := os.Getwd()
	path := filepath.Join(wd, "Forgefile")

	// Don't overwrite.
	if _, err := os.Stat(path); err == nil {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Create Forgefile",
			applied:   false,
			manualMsg: "Forgefile already exists but may be malformed. Edit manually.",
		}
	}

	content := `[project]
name = "my-project"
version = "0.1.0"

[agent]
type = "chat"
model = "gpt-4.1-mini"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Create Forgefile",
			applied:   false,
			err:       err,
			manualMsg: "Create a Forgefile manually with [project] and [agent] sections.",
		}
	}

	return &fixResult{
		checkMsg: r.message,
		fixDesc:  "Created Forgefile with default [project] and [agent] sections",
		applied:  true,
	}
}

func fixCreateForgeDir(r checkResult) *fixResult {
	wd, _ := os.Getwd()
	forgeDir := filepath.Join(wd, ".forge")

	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Create .forge directory",
			applied:   false,
			err:       err,
			manualMsg: "Run: mkdir -p .forge",
		}
	}

	// Also create common subdirectories.
	for _, sub := range []string{"genealogy", "consent", "governance", "catalog", "learn"} {
		os.MkdirAll(filepath.Join(forgeDir, sub), 0o755)
	}

	return &fixResult{
		checkMsg: r.message,
		fixDesc:  "Created .forge directory with standard subdirectories",
		applied:  true,
	}
}

func fixAddForgefileSection(r checkResult, section string) *fixResult {
	wd, _ := os.Getwd()

	// Find the Forgefile.
	forgeNames := []string{"Forgefile", "forge.yaml", "forge.yml"}
	var forgePath string
	for _, name := range forgeNames {
		p := filepath.Join(wd, name)
		if _, err := os.Stat(p); err == nil {
			forgePath = p
			break
		}
	}

	if forgePath == "" {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Add [" + section + "] section",
			applied:   false,
			manualMsg: "No Forgefile found. Run forge init first.",
		}
	}

	data, err := os.ReadFile(forgePath)
	if err != nil {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Add [" + section + "] section",
			applied:   false,
			err:       err,
			manualMsg: "Read the Forgefile and add the section manually.",
		}
	}

	content := string(data)

	// Don't add if already present.
	if strings.Contains(content, "["+section+"]") {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Add [" + section + "] section",
			applied:   false,
			manualMsg: "Section already exists — check for typos.",
		}
	}

	var addition string
	switch section {
	case "project":
		addition = "\n[project]\nname = \"my-project\"\nversion = \"0.1.0\"\n"
	case "agent":
		addition = "\n[agent]\ntype = \"chat\"\nmodel = \"gpt-4.1-mini\"\n"
	default:
		addition = fmt.Sprintf("\n[%s]\n", section)
	}

	if err := os.WriteFile(forgePath, []byte(content+addition), 0o644); err != nil {
		return &fixResult{
			checkMsg: r.message,
			fixDesc:  "Add [" + section + "] section",
			applied:  false,
			err:      err,
		}
	}

	return &fixResult{
		checkMsg: r.message,
		fixDesc:  "Added [" + section + "] section to Forgefile",
		applied:  true,
	}
}

func fixGitInit(r checkResult) *fixResult {
	wd, _ := os.Getwd()

	// Check if we can run git init.
	gitPath, err := lookupPath("git")
	if err != nil {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Initialize git repository",
			applied:   false,
			manualMsg: "Install git first, then run: git init",
		}
	}

	// Run git init.
	cmd := execCommand(gitPath, "init", wd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Initialize git repository",
			applied:   false,
			err:       err,
			manualMsg: "Run manually: git init",
		}
	} else {
		_ = output
	}

	return &fixResult{
		checkMsg: r.message,
		fixDesc:  "Initialized git repository",
		applied:  true,
	}
}

func fixCreateDir(r checkResult, baseDir string) *fixResult {
	wd, _ := os.Getwd()
	dir := filepath.Join(wd, baseDir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Create " + baseDir,
			applied:   false,
			err:       err,
			manualMsg: "Run: mkdir -p " + baseDir,
		}
	}

	return &fixResult{
		checkMsg: r.message,
		fixDesc:  "Created " + baseDir,
		applied:  true,
	}
}

// lookupPath finds an executable in PATH.
func lookupPath(name string) (string, error) {
	return exec.LookPath(name)
}

// execCommand creates an exec.Cmd.
func execCommand(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

// printFixResults displays the auto-fix results.
func printFixResults(fixes []fixResult) {
	if len(fixes) == 0 {
		fmt.Println(pretty.Sprint(pretty.DimF, "  No auto-fixable issues found."))
		return
	}

	applied := 0
	manual := 0

	fmt.Println()
	fmt.Println(pretty.Sprint(pretty.BoldF, "  Auto-Fix Results:"))
	fmt.Println()

	for _, f := range fixes {
		if f.applied {
			applied++
			fmt.Printf("  %s %s\n", pretty.Sprint(pretty.Success, pretty.Checkmark), f.fixDesc)
		} else {
			manual++
			fmt.Printf("  %s %s\n", pretty.Sprint(pretty.Warning, "!"), f.fixDesc)
			if f.err != nil {
				fmt.Printf("    Error: %s\n", f.err)
			}
			if f.manualMsg != "" {
				fmt.Printf("    Manual: %s\n", f.manualMsg)
			}
		}
	}

	fmt.Println()
	fmt.Printf("  %d fixed, %d need manual action\n", applied, manual)
}
