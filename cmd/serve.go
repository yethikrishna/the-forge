package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	agentAPIVersion = "latest"
	agentAPIBin     = "agentapi"
)

func getForgeDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".forge")
	os.MkdirAll(dir, 0o755)
	return dir
}

func getBinDir() string {
	dir := filepath.Join(getForgeDir(), "bin")
	os.MkdirAll(dir, 0o755)
	return dir
}

func findAgentAPI() (string, error) {
	binPath := filepath.Join(getBinDir(), agentAPIBin)
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}
	if path, err := exec.LookPath(agentAPIBin); err == nil {
		return path, nil
	}

	fmt.Println("Forge: Downloading agentapi...")
	osStr := "linux"
	archStr := "arm64"
	if out, err := exec.Command("uname", "-m").Output(); err == nil {
		a := strings.TrimSpace(string(out))
		if a == "x86_64" {
			archStr = "amd64"
		}
	}
	if out, err := exec.Command("uname", "-s").Output(); err == nil {
		s := strings.ToLower(strings.TrimSpace(string(out)))
		if s == "darwin" {
			osStr = "darwin"
		}
	}

	url := fmt.Sprintf("https://github.com/coder/agentapi/releases/%s/download/agentapi-%s-%s", agentAPIVersion, osStr, archStr)
	dlCmd := exec.Command("curl", "-fsSL", url, "-o", binPath)
	if err := dlCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to download agentapi: %w", err)
	}
	os.Chmod(binPath, 0o755)
	fmt.Printf("Forge: Downloaded agentapi to %s\n", binPath)
	return binPath, nil
}

func waitForPort(ctx context.Context, port int, timeout time.Duration) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}

func findFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func serveCmd() *cobra.Command {
	var port int
	var agent string
	var model string
	var jail bool
	var jailRule string
	var jailJS string
	var verbose bool
	var acp bool

	cmd := &cobra.Command{
		Use:   "serve [agent-command...]",
		Short: "Start the Forge orchestration server",
		Long: `Start the unified server that:
  - Launches AI agents via AgentAPI (ACP or PTY transport)
  - Routes requests to any model (OpenAI, Anthropic, Google, xAI, Azure)
  - Sandboxes operations with httpjail (when --jail enabled)
  - Exposes a unified HTTP/WebSocket API

Examples:
  forge serve -- claude
  forge serve --type=codex -- codex
  forge serve -m openai/gpt-5-mini -- claude
  forge serve --acp -- claude
  forge serve --jail --jail-rule=github.com -- claude`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				fmt.Println("\nForge: Cooling down...")
				cancel()
			}()

			transport := "PTY"
			if acp {
				transport = "ACP"
			}

			fmt.Println("Forge: The Forge ignites...")
			fmt.Printf("   Agent:     %s\n", agent)
			fmt.Printf("   Model:     %s\n", model)
			fmt.Printf("   Port:      %d\n", port)
			fmt.Printf("   Transport: %s\n", transport)
			fmt.Printf("   Jailed:    %v\n", jail)
			if jail {
				fmt.Printf("   Jail rule: %s\n", jailRule)
			}

			// Find or download agentapi
			agentAPIPath, err := findAgentAPI()
			if err != nil {
				return fmt.Errorf("agentapi not available: %w", err)
			}

			// Build agentapi command arguments
			agentCmd := args
			if len(agentCmd) == 0 {
				agentCmd = []string{agent}
			}

			apiArgs := []string{"server"}
			if agent != "" && agent != "claude" {
				apiArgs = append(apiArgs, "--type", agent)
			}
			apiArgs = append(apiArgs, "--port", strconv.Itoa(port))
			if acp {
				apiArgs = append(apiArgs, "--experimental-acp")
			}
			apiArgs = append(apiArgs, "--")
			apiArgs = append(apiArgs, agentCmd...)

			// Start model proxy if non-Anthropic model requested
			parts := strings.SplitN(model, "/", 2)
			provider := "anthropic"
			if len(parts) == 2 {
				provider = parts[0]
			}

			env := os.Environ()
			if provider != "anthropic" {
				proxyPort, err := findFreePort()
				if err != nil {
					return fmt.Errorf("failed to find free port for model proxy: %w", err)
				}

				anyclaudePath, err := exec.LookPath("anyclaude")
				if err != nil {
					fmt.Printf("Forge: Model routing to %s requested but anyclaude not found\n", provider)
					fmt.Printf("Forge: Install with: bun install -g anyclaude\n")
					fmt.Printf("Forge: Falling back to native agent model selection\n")
				} else {
					proxyEnv := env
					debugLevel := "0"
					if verbose {
						debugLevel = "1"
					}
					proxyCmd := exec.CommandContext(ctx, anyclaudePath, "--model", model)
					proxyCmd.Env = append(proxyEnv, "PROXY_ONLY=true", "ANYCLAUDE_DEBUG="+debugLevel)
					proxyCmd.Stdout = os.Stderr
					proxyCmd.Stderr = os.Stderr
					if err := proxyCmd.Start(); err != nil {
						fmt.Printf("Forge: Model proxy failed to start: %v\n", err)
					} else {
						defer func() {
							proxyCmd.Process.Signal(syscall.SIGTERM)
							time.Sleep(time.Second)
							proxyCmd.Process.Kill()
						}()
						env = append(env, fmt.Sprintf("ANTHROPIC_BASE_URL=http://127.0.0.1:%d", proxyPort))
						fmt.Printf("Forge: Model proxy started on port %d (routing to %s)\n", proxyPort, model)
					}
				}
			}

			// Build the final command - optionally wrapped with httpjail
			var proc *exec.Cmd
			if jail {
				httpjailPath, err := exec.LookPath("httpjail")
				if err != nil {
					fmt.Println("Forge: httpjail not found, running without sandbox")
					fmt.Println("Forge: Install with: cargo install httpjail")
					proc = exec.CommandContext(ctx, agentAPIPath, apiArgs...)
				} else {
					jailArgs := []string{}
					if jailRule != "" {
						jailArgs = append(jailArgs, "--allow", jailRule)
					}
					if jailJS != "" {
						jailArgs = append(jailArgs, "--js", jailJS)
					}
					if verbose {
						jailArgs = append(jailArgs, "--verbose")
					}
					jailArgs = append(jailArgs, "--")
					jailArgs = append(jailArgs, agentAPIPath)
					jailArgs = append(jailArgs, apiArgs...)
					proc = exec.CommandContext(ctx, httpjailPath, jailArgs...)
					fmt.Printf("Forge: Running in httpjail sandbox (allow: %s)\n", jailRule)
				}
			} else {
				proc = exec.CommandContext(ctx, agentAPIPath, apiArgs...)
			}

			proc.Env = env
			proc.Stdin = os.Stdin
			proc.Stdout = os.Stdout
			proc.Stderr = os.Stderr

			if err := proc.Start(); err != nil {
				return fmt.Errorf("failed to start agentapi: %w", err)
			}

			// Wait for server to be ready
			fmt.Println("Forge: Waiting for agentapi server...")
			if err := waitForPort(ctx, port, 30*time.Second); err != nil {
				return fmt.Errorf("agentapi server did not start: %w", err)
			}

			fmt.Printf("\nForge: Server ready!\n")
			fmt.Printf("   Chat:   http://localhost:%d/chat\n", port)
			fmt.Printf("   API:    http://localhost:%d/api\n", port)
			fmt.Printf("   Docs:   http://localhost:%d/docs\n", port)
			fmt.Printf("   Events: http://localhost:%d/events\n", port)
			if jail {
				fmt.Println("   Network: SANDBOXED (httpjail)")
			}
			fmt.Println("\nForge: The wielder and the sword are one.")

			// Wait for process or context cancellation
			done := make(chan error, 1)
			go func() {
				done <- proc.Wait()
			}()

			select {
			case <-ctx.Done():
				proc.Process.Signal(syscall.SIGTERM)
				time.Sleep(3 * time.Second)
				proc.Process.Kill()
			case err := <-done:
				if err != nil {
					return fmt.Errorf("agentapi exited: %w", err)
				}
			}

			fmt.Println("Forge: The Forge cools.")
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 3284, "Server port")
	cmd.Flags().StringVarP(&agent, "agent", "a", "claude", "Agent type (claude|codex|gemini|aider|goose|amp|cursor|auggie|q|opencode|custom)")
	cmd.Flags().StringVarP(&model, "model", "m", "anthropic/claude-sonnet-4-20250514", "Model (provider/model format)")
	cmd.Flags().BoolVarP(&jail, "jail", "j", false, "Enable httpjail network sandboxing")
	cmd.Flags().StringVar(&jailRule, "jail-rule", "github.com", "httpjail allow rule")
	cmd.Flags().StringVar(&jailJS, "jail-js", "", "httpjail JavaScript rule")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose logging")
	cmd.Flags().BoolVar(&acp, "acp", false, "Use ACP transport instead of PTY")

	return cmd
}
