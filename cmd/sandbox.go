package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/forge/sword/internal/sandbox"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func sandboxCmd() *cobra.Command {
	var cpuShares int64
	var memoryMB int64
	var networkOff bool
	var readonlyFS bool
	var timeout int

	cmd := &cobra.Command{
		Use:   "sandbox run [command]",
		Short: "Run commands in isolated sandboxes",
		Long: `Execute commands in isolated environments with resource limits.
Supports Docker and MicroVM backends.

Examples:
  forge sandbox run -- echo hello
  forge sandbox run --cpu 1024 --memory 1024 -- go test ./...
  forge sandbox run --network-off -- curl https://example.com
  forge sandbox run --backend microvm -- uname -a`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, _ := cmd.Flags().GetString("backend")
			image, _ := cmd.Flags().GetString("image")
			name, _ := cmd.Flags().GetString("name")

			if name == "" {
				name = "sandbox-run"
			}

			fmt.Println(pretty.HeaderLine("Forge Sandbox"))

			switch backend {
			case "docker", "":
				return runDockerSandbox(name, image, args, cpuShares, memoryMB, networkOff, readonlyFS, timeout)
			case "microvm":
				return runMicroVMSandbox(name, image, args, timeout)
			default:
				return fmt.Errorf("unsupported backend: %s", backend)
			}
		},
	}

	cmd.Flags().String("backend", "docker", "Sandbox backend (docker|microvm)")
	cmd.Flags().StringP("image", "i", "alpine:3.20", "Container image")
	cmd.Flags().String("name", "", "Sandbox name")
	cmd.Flags().Int64Var(&cpuShares, "cpu", 512, "CPU shares")
	cmd.Flags().Int64Var(&memoryMB, "memory", 512, "Memory limit (MB)")
	cmd.Flags().BoolVar(&networkOff, "network-off", false, "Disable network")
	cmd.Flags().BoolVar(&readonlyFS, "readonly", false, "Read-only filesystem")
	cmd.Flags().IntVar(&timeout, "timeout", 300, "Execution timeout (seconds)")

	return cmd
}

func runDockerSandbox(name, image string, command []string, cpu, mem int64, netOff, readonly bool, timeoutSec int) error {
	home, _ := os.UserHomeDir()
	dir := home + "/.forge/sandbox"
	mgr := sandbox.NewDockerSandboxManager(dir)

	sb, err := mgr.Create(sandbox.DockerSandboxConfig{
		Name:       name,
		Image:      image,
		Command:    command,
		CPUShares:  cpu,
		MemoryMB:   mem,
		NetworkOff: netOff,
		ReadonlyFS: readonly,
	})
	if err != nil {
		return err
	}

	fmt.Printf("  Sandbox: %s (docker)\n", sb.ID)
	fmt.Printf("  Image:   %s\n", image)
	fmt.Printf("  Command: %v\n", command)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	if err := mgr.Start(ctx, sb.ID); err != nil {
		return fmt.Errorf("start sandbox: %w", err)
	}

	fmt.Println(pretty.InfoLine("Running..."))

	if err := mgr.Wait(ctx, sb.ID, 0); err != nil {
		fmt.Printf("  Wait error: %v\n", err)
	}

	// Show logs
	logs, err := mgr.Logs(ctx, sb.ID)
	if err == nil && logs != "" {
		fmt.Println(logs)
	}

	// Cleanup
	mgr.Remove(ctx, sb.ID)

	updated, _ := mgr.Get(sb.ID)
	if updated != nil && updated.ExitCode != 0 {
		os.Exit(updated.ExitCode)
	}

	return nil
}

func runMicroVMSandbox(name, image string, command []string, timeoutSec int) error {
	fmt.Println(pretty.InfoLine("MicroVM sandbox backend (experimental)"))
	fmt.Printf("  Name: %s, Command: %v\n", name, command)
	return fmt.Errorf("MicroVM backend requires firecracker setup — use Docker backend for now")
}
