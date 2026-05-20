// Package sandbox provides secure code execution environments.
// Every experiment needs a safe chamber.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Language represents a supported programming language.
type Language string

const (
	Go         Language = "go"
	Python     Language = "python"
	JavaScript Language = "javascript"
	TypeScript Language = "typescript"
	Rust       Language = "rust"
	Bash       Language = "bash"
	Ruby       Language = "ruby"
	Java       Language = "java"
)

// Config holds sandbox configuration.
type Config struct {
	Language    Language
	Timeout     time.Duration
	MemoryLimit int64  // bytes
	WorkDir     string // working directory
	Network     bool   // allow network access
	Env         map[string]string
}

// Result holds the execution result.
type Result struct {
	ExitCode   int
	Stdout     string
	Stderr     string
	Duration   time.Duration
	OOMKilled  bool
	TimedOut   bool
}

// Sandbox executes code in an isolated environment.
type Sandbox struct {
	config Config
}

// New creates a new sandbox with the given configuration.
func New(config Config) *Sandbox {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	return &Sandbox{config: config}
}

// Execute runs code in the sandbox.
func (s *Sandbox) Execute(ctx context.Context, code string) (*Result, error) {
	// Create temp directory for execution
	dir, err := os.MkdirTemp(s.config.WorkDir, "forge-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("sandbox: temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	// Write code to file
	filename, err := s.writeFile(dir, code)
	if err != nil {
		return nil, err
	}

	// Build command
	cmd, cleanup, err := s.buildCommand(ctx, dir, filename)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set environment
	if s.config.Env != nil {
		cmd.Env = os.Environ()
		for k, v := range s.config.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Execute with timeout
	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.ExitCode = -1
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}

	return result, nil
}

// ExecuteFile runs a file in the sandbox.
func (s *Sandbox) ExecuteFile(ctx context.Context, path string) (*Result, error) {
	code, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("sandbox: read file: %w", err)
	}
	return s.Execute(ctx, string(code))
}

// SupportedLanguages returns a list of supported languages.
func SupportedLanguages() []Language {
	return []Language{Go, Python, JavaScript, TypeScript, Rust, Bash, Ruby, Java}
}

// DetectLanguage detects the language from a file extension.
func DetectLanguage(ext string) (Language, bool) {
	switch strings.ToLower(ext) {
	case ".go":
		return Go, true
	case ".py":
		return Python, true
	case ".js", ".mjs":
		return JavaScript, true
	case ".ts":
		return TypeScript, true
	case ".rs":
		return Rust, true
	case ".sh", ".bash":
		return Bash, true
	case ".rb":
		return Ruby, true
	case ".java":
		return Java, true
	default:
		return "", false
	}
}

// writeFile writes code to a temporary file with the correct extension.
func (s *Sandbox) writeFile(dir, code string) (string, error) {
	ext := s.languageExt()
	filename := "code" + ext
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		return "", fmt.Errorf("sandbox: write file: %w", err)
	}
	return filename, nil
}

// buildCommand creates the exec.Cmd for the given language.
func (s *Sandbox) buildCommand(ctx context.Context, dir, filename string) (*exec.Cmd, func(), error) {
	var cmd *exec.Cmd
	var cleanup func()

	timeoutCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	_ = cancel

	switch s.config.Language {
	case Go:
		cmd = exec.CommandContext(timeoutCtx, "go", "run", filename)
		cmd.Dir = dir

	case Python:
		cmd = exec.CommandContext(timeoutCtx, "python3", filename)
		cmd.Dir = dir

	case JavaScript:
		cmd = exec.CommandContext(timeoutCtx, "node", filename)
		cmd.Dir = dir

	case TypeScript:
		cmd = exec.CommandContext(timeoutCtx, "npx", "ts-node", filename)
		cmd.Dir = dir

	case Rust:
		// Compile first, then run
		binPath := filepath.Join(dir, "code_bin")
		compileCmd := exec.CommandContext(timeoutCtx, "rustc", "-o", binPath, filename)
		compileCmd.Dir = dir
		if out, err := compileCmd.CombinedOutput(); err != nil {
			return nil, nil, fmt.Errorf("sandbox: rust compile: %s: %w", string(out), err)
		}
		cmd = exec.CommandContext(timeoutCtx, binPath)
		cmd.Dir = dir

	case Bash:
		cmd = exec.CommandContext(timeoutCtx, "bash", filename)
		cmd.Dir = dir

	case Ruby:
		cmd = exec.CommandContext(timeoutCtx, "ruby", filename)
		cmd.Dir = dir

	case Java:
		// Compile first
		compileCmd := exec.CommandContext(timeoutCtx, "javac", filename)
		compileCmd.Dir = dir
		if out, err := compileCmd.CombinedOutput(); err != nil {
			return nil, nil, fmt.Errorf("sandbox: java compile: %s: %w", string(out), err)
		}
		cmd = exec.CommandContext(timeoutCtx, "java", "Code")
		cmd.Dir = dir

	default:
		return nil, nil, fmt.Errorf("sandbox: unsupported language: %s", s.config.Language)
	}

	return cmd, cleanup, nil
}

func (s *Sandbox) languageExt() string {
	switch s.config.Language {
	case Go:
		return ".go"
	case Python:
		return ".py"
	case JavaScript:
		return ".js"
	case TypeScript:
		return ".ts"
	case Rust:
		return ".rs"
	case Bash:
		return ".sh"
	case Ruby:
		return ".rb"
	case Java:
		return ".java"
	default:
		return ".txt"
	}
}

// Eval is a convenience function for quick code evaluation.
func Eval(lang Language, code string) (*Result, error) {
	s := New(Config{Language: lang, Timeout: 30 * time.Second})
	return s.Execute(context.Background(), code)
}

// IsAvailable checks if a language runtime is available on the system.
func IsAvailable(lang Language) bool {
	var cmd *exec.Cmd
	switch lang {
	case Go:
		cmd = exec.Command("go", "version")
	case Python:
		cmd = exec.Command("python3", "--version")
	case JavaScript:
		cmd = exec.Command("node", "--version")
	case Rust:
		cmd = exec.Command("rustc", "--version")
	case Bash:
		cmd = exec.Command("bash", "--version")
	case Ruby:
		cmd = exec.Command("ruby", "--version")
	case Java:
		cmd = exec.Command("java", "-version")
	default:
		return false
	}
	return cmd.Run() == nil
}

// RuntimeInfo returns information about available runtimes.
func RuntimeInfo() map[Language]string {
	info := make(map[Language]string)
	for _, lang := range SupportedLanguages() {
		if IsAvailable(lang) {
			info[lang] = "available"
		} else {
			info[lang] = "not installed"
		}
	}
	// Add OS/arch info
	info["_os"] = runtime.GOOS
	info["_arch"] = runtime.GOARCH
	return info
}

// io.Writer type alias for compatibility
type Writer = io.Writer
