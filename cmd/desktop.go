package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func desktopCmd() *cobra.Command {
	var display int
	var resolution string
	var vnc bool
	var noVNC bool

	cmd := &cobra.Command{
		Use:   "desktop",
		Short: "Launch a Linux desktop environment for AI agents",
		Long: `Provision a portable Linux desktop that AI agents
can interact with visually. Supports VNC and noVNC access.

Examples:
  forge desktop start
  forge desktop start --resolution 1920x1080 --vnc
  forge desktop start --novnc --display :2
  forge desktop status
  forge desktop stop`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Start a desktop environment",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

				fmt.Println(pretty.HeaderLine("Forge Desktop — Agent Workspace"))
				fmt.Printf("   Display:    :%d\n", display)
				fmt.Printf("   Resolution: %s\n", resolution)
				if vnc {
					fmt.Printf("   VNC:        port 59%02d\n", display)
				}
				if noVNC {
					fmt.Printf("   noVNC:      http://localhost:6080\n")
				}
				fmt.Println()

				// Check for Xvfb, x11vnc, novnc
				deps := []string{"Xvfb", "x11vnc", "fluxbox"}
				missing := checkDeps(deps)
				if len(missing) > 0 {
					fmt.Println(pretty.WarningLine(fmt.Sprintf("Missing dependencies: %v", missing)))
					fmt.Println("  Install with: apt-get install xvfb x11vnc fluxbox novnc")
				}

				fmt.Println(pretty.InfoLine("Desktop environment starting..."))
				fmt.Println("  The wielder and the sword are one.")

				select {
				case <-sigChan:
					fmt.Println("\nForge: Desktop shutting down...")
				case <-ctx.Done():
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the desktop environment",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Println(pretty.InfoLine("Stopping desktop environment..."))
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Check desktop status",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Println(pretty.HeaderLine("Desktop Status"))
				fmt.Printf("  Display:  :%d\n", display)
				fmt.Println("  Running:  unknown (agent desktop is a framework)")
				return nil
			},
		},
		&cobra.Command{
			Use:   "screenshot",
			Short: "Capture a screenshot of the desktop",
			RunE: func(cmd *cobra.Command, args []string) error {
				outPath := "screenshot.png"
				if len(args) > 0 {
					outPath = args[0]
				}
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Capturing screenshot to %s", outPath)))
				fmt.Println("  Requires: xdpyinfo, import (ImageMagick)")
				return nil
			},
		},
	)

	cmd.PersistentFlags().IntVar(&display, "display", 1, "X display number")
	cmd.PersistentFlags().StringVarP(&resolution, "resolution", "r", "1280x720", "Desktop resolution (WxH)")
	cmd.PersistentFlags().BoolVar(&vnc, "vnc", false, "Enable VNC server")
	cmd.PersistentFlags().BoolVar(&noVNC, "novnc", false, "Enable noVNC web client")

	return cmd
}

func checkDeps(deps []string) []string {
	var missing []string
	for _, dep := range deps {
		if _, err := os.Stat("/usr/bin/" + dep); err != nil {
			missing = append(missing, dep)
		}
	}
	return missing
}
