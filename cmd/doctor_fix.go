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

	// Fix: Forge Go SDK not found → guide installation.
	if strings.Contains(msg, "forge go sdk not found") {
		return fixInstallGoSDK(r)
	}

	// Fix: Forge Go SDK not in PATH → add export to shell profile.
	if strings.Contains(msg, "forge go sdk") && strings.Contains(msg, "not in path") {
		return fixGoSDKPath(r)
	}

	// Fix: Stale WAL files → trigger replay by opening persistence.Store.
	if strings.Contains(msg, "stale wal files found") {
		return fixReplayWAL(r)
	}

	// Fix: .forge not writable → chmod.
	if strings.Contains(msg, ".forge directory is not writable") {
		return fixForgePermissions(r)
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

// fixInstallGoSDK guides the user to install the Forge-managed Go SDK.
func fixInstallGoSDK(r checkResult) *fixResult {
	home, _ := os.UserHomeDir()
	sdkDir := filepath.Join(home, "go-sdk")

	// Check if directory already partially exists.
	if _, err := os.Stat(sdkDir); err == nil {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Forge Go SDK directory exists but Go binary missing",
			applied:   false,
			manualMsg: fmt.Sprintf("Re-download Go and extract to %s: https://go.dev/dl/", sdkDir),
		}
	}

	// Create the sdk directory so the user knows where to install.
	if err := os.MkdirAll(sdkDir, 0o755); err != nil {
		return &fixResult{
			checkMsg: r.message,
			fixDesc:  "Create ~/go-sdk directory",
			applied:  false,
			err:      err,
		}
	}

	return &fixResult{
		checkMsg:  r.message,
		fixDesc:   "Created ~/go-sdk directory",
		applied:   true,
		manualMsg: "Download Go from https://go.dev/dl/ and extract to ~/go-sdk/go",
	}
}

// fixGoSDKPath appends the SDK bin export to the user's shell profile.
func fixGoSDKPath(r checkResult) *fixResult {
	home, _ := os.UserHomeDir()
	sdkBinDir := filepath.Join(home, "go-sdk", "go", "bin")
	exportLine := fmt.Sprintf("\nexport PATH=$PATH:%s\n", sdkBinDir)

	// Try common shell profiles in order of preference.
	profiles := []string{
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
	}

	for _, profile := range profiles {
		data, err := os.ReadFile(profile)
		if err != nil {
			continue
		}
		// Already present?
		if strings.Contains(string(data), sdkBinDir) {
			return &fixResult{
				checkMsg:  r.message,
				fixDesc:   fmt.Sprintf("Go SDK PATH already in %s", profile),
				applied:   true,
				manualMsg: "Restart your shell or run: source " + profile,
			}
		}
		// Append.
		f, err := os.OpenFile(profile, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			continue
		}
		_, werr := f.WriteString(exportLine)
		f.Close()
		if werr == nil {
			return &fixResult{
				checkMsg:  r.message,
				fixDesc:   fmt.Sprintf("Added Go SDK to PATH in %s", profile),
				applied:   true,
				manualMsg: "Restart your shell or run: source " + profile,
			}
		}
	}

	return &fixResult{
		checkMsg:  r.message,
		fixDesc:   "Add Go SDK to PATH",
		applied:   false,
		manualMsg: fmt.Sprintf("Manually add to your shell profile: export PATH=$PATH:%s", sdkBinDir),
	}
}

// fixReplayWAL replays stale WAL files by reading and promoting them.
// The persistence.Store already handles this on Open(), so we just open and
// immediately close a store for each WAL-containing directory.
func fixReplayWAL(r checkResult) *fixResult {
	walDirs := []string{
		filepath.Join(".forge", "catalog"),
		filepath.Join(".forge", "govern"),
		filepath.Join(".forge", "costlive"),
		filepath.Join(".forge", "mcpgateway"),
	}

	replayed := 0
	for _, d := range walDirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			continue
		}
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".wal") {
				continue
			}
			walPath := filepath.Join(d, e.Name())
			data, rerr := os.ReadFile(walPath)
			if rerr != nil || len(data) == 0 {
				os.Remove(walPath)
				continue
			}
			// Promote WAL → JSON.
			stem := strings.TrimSuffix(e.Name(), ".wal")
			target := filepath.Join(d, stem+".json")
			if werr := os.WriteFile(target, data, 0o644); werr == nil {
				os.Remove(walPath)
				replayed++
			}
		}
	}

	if replayed > 0 {
		return &fixResult{
			checkMsg: r.message,
			fixDesc:  fmt.Sprintf("Replayed %d WAL file(s) to restore persistence state", replayed),
			applied:  true,
		}
	}
	return &fixResult{
		checkMsg: r.message,
		fixDesc:  "WAL replay: no replayable files found",
		applied:  true, // Not an error — files may already be gone.
	}
}

// fixForgePermissions attempts to chmod .forge to be writable by the current user.
func fixForgePermissions(r checkResult) *fixResult {
	if err := os.Chmod(".forge", 0o755); err != nil {
		return &fixResult{
			checkMsg:  r.message,
			fixDesc:   "Fix .forge directory permissions",
			applied:   false,
			err:       err,
			manualMsg: "Run: chmod 755 .forge",
		}
	}
	return &fixResult{
		checkMsg: r.message,
		fixDesc:  "Fixed .forge directory permissions (chmod 755)",
		applied:  true,
	}
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
