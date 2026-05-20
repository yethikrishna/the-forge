package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func doctorCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose Forge environment and configuration",
		Long: `Run a comprehensive health check on your Forge setup.
Checks Go version, API keys, network connectivity, configuration,
and suggests fixes for common issues.

Examples:
  forge doctor
  forge doctor --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(pretty.HeaderLine("Forge Environment Diagnostic"))
			fmt.Println()

			passed := 0
			warned := 0
			failed := 0

			// Run all checks
			results := []checkResult{}

			results = append(results, checkGoVersion()...)
			results = append(results, checkOSArch()...)
			results = append(results, checkForgeBinary()...)
			results = append(results, checkAPIKeys()...)
			results = append(results, checkNetworkConnectivity()...)
			results = append(results, checkForgefile()...)
			results = append(results, checkDiskSpace()...)
			results = append(results, checkGit()...)

			// Print results
			for _, r := range results {
				switch r.status {
				case statusPass:
					passed++
					fmt.Printf("  %s %s\n", pretty.Sprint(pretty.Success, pretty.Checkmark), r.message)
				case statusWarn:
					warned++
					fmt.Printf("  %s %s\n", pretty.Sprint(pretty.Warning, "!"), r.message)
				case statusFail:
					failed++
					fmt.Printf("  %s %s\n", pretty.Sprint(pretty.Error, pretty.Cross), r.message)
				}
				if r.hint != "" {
					fmt.Printf("    %s\n", pretty.Sprint(pretty.DimF, r.hint))
				}
				if verbose && r.detail != "" {
					for _, line := range strings.Split(r.detail, "\n") {
						fmt.Printf("    %s\n", pretty.Sprint(pretty.DimF, line))
					}
				}
			}

			// Summary
			fmt.Println()
			total := passed + warned + failed
			fmt.Printf("  %s %d checks: %d passed, %d warnings, %d failed\n",
				pretty.Sprint(pretty.BoldF, "Summary:"),
				total, passed, warned, failed)

			if failed > 0 {
				fmt.Println()
				fmt.Printf("  %s Run with --verbose for more details.\n",
					pretty.Sprint(pretty.Info, pretty.Arrow))
				return fmt.Errorf("environment has %d issue(s) that need attention", failed)
			}

			fmt.Println()
			fmt.Println("  The forge burns bright. Ready to wield.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed diagnostic output")

	return cmd
}

type checkStatus int

const (
	statusPass checkStatus = iota
	statusWarn
	statusFail
)

type checkResult struct {
	status checkStatus
	message string
	hint    string
	detail  string
}

func pass(msg string) checkResult {
	return checkResult{status: statusPass, message: msg}
}

func passDetail(msg, detail string) checkResult {
	return checkResult{status: statusPass, message: msg, detail: detail}
}

func warn(msg, hint string) checkResult {
	return checkResult{status: statusWarn, message: msg, hint: hint}
}

func fail(msg, hint string) checkResult {
	return checkResult{status: statusFail, message: msg, hint: hint}
}

func checkGoVersion() []checkResult {
	var results []checkResult

	goPath, err := exec.LookPath("go")
	if err != nil {
		results = append(results, fail("Go toolchain not found in PATH", "Install Go 1.21+ from https://go.dev/dl/"))
		return results
	}

	out, err := exec.Command("go", "version").Output()
	if err != nil {
		results = append(results, fail("Cannot determine Go version", "Ensure 'go version' runs successfully"))
		return results
	}

	versionStr := strings.TrimSpace(string(out))
	results = append(results, passDetail("Go toolchain: "+versionStr, "Path: "+goPath))

	return results
}

func checkOSArch() []checkResult {
	return []checkResult{
		passDetail(fmt.Sprintf("Platform: %s/%s", runtime.GOOS, runtime.GOARCH), fmt.Sprintf("Num CPU: %d", runtime.NumCPU())),
	}
}

func checkForgeBinary() []checkResult {
	return []checkResult{
		pass(fmt.Sprintf("Forge version: v%s (built %s)", forgeVersion, buildTime)),
	}
}

func checkAPIKeys() []checkResult {
	var results []checkResult

	keys := []struct {
		name     string
		envVar   string
		required bool
	}{
		{"OpenAI", "OPENAI_API_KEY", false},
		{"Anthropic", "ANTHROPIC_API_KEY", false},
		{"Google AI", "GOOGLE_AI_API_KEY", false},
		{"xAI", "XAI_API_KEY", false},
		{"Groq", "GROQ_API_KEY", false},
	}

	found := 0
	for _, k := range keys {
		val := os.Getenv(k.envVar)
		if val != "" {
			found++
			results = append(results, passDetail(
				fmt.Sprintf("%s API key: configured", k.name),
				fmt.Sprintf("Key: %s...%s", val[:min(4, len(val))], val[max(0, len(val)-4):]),
			))
		} else {
			results = append(results, warn(
				fmt.Sprintf("%s API key: not set", k.name),
				fmt.Sprintf("Set %s to use %s models", k.envVar, k.name),
			))
		}
	}

	if found == 0 {
		results = append(results, fail(
			"No LLM API keys configured",
			"Set at least one: OPENAI_API_KEY, ANTHROPIC_API_KEY, or GOOGLE_AI_API_KEY",
		))
	}

	return results
}

func checkNetworkConnectivity() []checkResult {
	var results []checkResult

	endpoints := []struct {
		name string
		addr string
	}{
		{"OpenAI API", "api.openai.com:443"},
		{"Anthropic API", "api.anthropic.com:443"},
		{"GitHub", "github.com:443"},
	}

	for _, ep := range endpoints {
		conn, err := net.DialTimeout("tcp", ep.addr, 5*time.Second)
		if err != nil {
			results = append(results, warn(
				fmt.Sprintf("%s: unreachable", ep.name),
				fmt.Sprintf("Could not connect to %s — check firewall or proxy", ep.addr),
			))
			continue
		}
		conn.Close()
		results = append(results, pass(fmt.Sprintf("%s: reachable", ep.name)))
	}

	return results
}

func checkForgefile() []checkResult {
	var results []checkResult

	// Check for Forgefile or forge.yaml in current directory and parent dirs
	forgeNames := []string{"Forgefile", "forge.yaml", "forge.yml"}
	wd, _ := os.Getwd()

	var forgePath string
	for _, name := range forgeNames {
		p := filepath.Join(wd, name)
		if _, err := os.Stat(p); err == nil {
			forgePath = p
			break
		}
	}

	if forgePath != "" {
		results = append(results, pass(fmt.Sprintf("Forgefile: found (%s)", forgePath)))

		// Read and validate basic content
		data, err := os.ReadFile(forgePath)
		if err == nil {
			content := string(data)
			if strings.Contains(content, "[project]") {
				results = append(results, pass("Forgefile: [project] section present"))
			} else {
				results = append(results, warn("Forgefile: missing [project] section", "Add a [project] section with name and version"))
			}
			if strings.Contains(content, "[agent]") {
				results = append(results, pass("Forgefile: [agent] section present"))
			} else {
				results = append(results, warn("Forgefile: missing [agent] section", "Add an [agent] section with type and model"))
			}
		}
	} else {
		results = append(results, warn("No Forgefile found in current directory", "Run 'forge init' to create one"))
	}

	// Check .forge directory
	forgeDir := filepath.Join(wd, ".forge")
	if info, err := os.Stat(forgeDir); err == nil && info.IsDir() {
		results = append(results, pass(".forge directory: exists"))
	} else {
		results = append(results, warn(".forge directory: not found", "Run 'forge init' to create project structure"))
	}

	return results
}

func checkDiskSpace() []checkResult {
	var results []checkResult

	wd, _ := os.Getwd()
	var stat syscallStat
	if err := stat.get(wd); err == nil {
		availMB := stat.available() / (1024 * 1024)
		if availMB < 100 {
			results = append(results, fail(
				fmt.Sprintf("Disk space: %d MB available (critically low)", availMB),
				"Free up disk space — Forge needs room to build and cache",
			))
		} else if availMB < 1024 {
			results = append(results, warn(
				fmt.Sprintf("Disk space: %d MB available (low)", availMB),
				"Consider freeing up disk space for optimal operation",
			))
		} else {
			availGB := availMB / 1024
			results = append(results, pass(fmt.Sprintf("Disk space: %d GB available", availGB)))
		}
	}

	return results
}

func checkGit() []checkResult {
	var results []checkResult

	gitPath, err := exec.LookPath("git")
	if err != nil {
		results = append(results, warn("Git not found in PATH", "Install git for version control features"))
		return results
	}

	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		results = append(results, warn("Git: cannot determine version", ""))
		return results
	}

	results = append(results, passDetail("Git: "+strings.TrimSpace(string(out)), "Path: "+gitPath))

	// Check if in a git repo
	wd, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(wd, ".git")); err == nil {
		results = append(results, pass("Git repository: initialized"))
	} else {
		results = append(results, warn("Not inside a git repository", "Run 'git init' for version control features"))
	}

	return results
}

func init() {
	// doctor is registered via root.go's AddCommand — no auto-reg needed
}
