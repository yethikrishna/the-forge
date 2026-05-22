package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func browseCmd() *cobra.Command {
	var (
		browser     string
		screenshot  bool
		snapshot    bool
		interact    bool
		timeout     int
		outputDir   string
		profile     string
		headless    bool
	)

	cmd := &cobra.Command{
		Use:   "browse [url]",
		Short: "Open a URL in the browser for manual or automated intervention",
		Long: `Open a URL in the system browser or Forge's managed browser.

Used when agents need manual intervention for tasks like CAPTCHAs,
complex logins, or visual verification. Also supports headless mode
for automated browsing with screenshots and DOM snapshots.

The browser control server runs on the Forge gateway and can be
accessed by agents for situational browser use when APIs don't exist.

Examples:
  forge browse https://example.com
  forge browse --browser firefox https://example.com
  forge browse --screenshot --headless https://example.com
  forge browse --snapshot --output ./data https://example.com
  forge browse --interact --timeout 300 https://login.example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]

			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				url = "https://" + url
			}

			// Headless mode: use Forge browser control API
			if headless {
				return runHeadlessBrowse(url, screenshot, snapshot, outputDir, profile, timeout)
			}

			// Interactive mode: open in local browser
			if interact {
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Opening %s in interactive browser mode (timeout: %ds)...", url, timeout)))
			} else {
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Opening %s in browser...", url)))
			}

			var cmdExec *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				if browser != "" {
					cmdExec = exec.Command("open", "-a", browser, url)
				} else {
					cmdExec = exec.Command("open", url)
				}
			case "linux":
				if browser != "" {
					cmdExec = exec.Command(browser, url)
				} else {
					cmdExec = exec.Command("xdg-open", url)
				}
			case "windows":
				cmdExec = exec.Command("cmd", "/c", "start", url)
			default:
				return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
			}

			if err := cmdExec.Start(); err != nil {
				return fmt.Errorf("failed to open browser: %w", err)
			}

			fmt.Println(pretty.SuccessLine("Browser opened"))

			// In screenshot/snapshot mode, wait and capture
			if screenshot || snapshot {
				waitSec := 5
				if timeout > 0 && timeout < 30 {
					waitSec = timeout
				}
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Waiting %d seconds for page to load...", waitSec)))
				time.Sleep(time.Duration(waitSec) * time.Second)

				if screenshot {
					if err := captureLocalScreenshot(url, outputDir); err != nil {
						fmt.Println(pretty.ErrorLine(fmt.Sprintf("Screenshot failed: %v", err)))
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&browser, "browser", "b", "", "Browser executable (default: system default)")
	cmd.Flags().BoolVar(&screenshot, "screenshot", false, "Capture screenshot after loading")
	cmd.Flags().BoolVar(&snapshot, "snapshot", false, "Capture DOM snapshot after loading")
	cmd.Flags().BoolVar(&interact, "interact", false, "Interactive mode — keep browser open for manual intervention")
	cmd.Flags().IntVar(&timeout, "timeout", 60, "Timeout in seconds for browser operations")
	cmd.Flags().StringVar(&outputDir, "output", ".", "Output directory for screenshots/snapshots")
	cmd.Flags().StringVar(&profile, "profile", "", "Browser profile (default: forge managed browser)")
	cmd.Flags().BoolVar(&headless, "headless", false, "Run in headless mode via Forge browser control API")

	return cmd
}

// runHeadlessBrowse uses the Forge browser control API for headless browsing.
func runHeadlessBrowse(url string, doScreenshot, doSnapshot bool, outputDir, profile string, timeout int) error {
	baseURL := os.Getenv("FORGE_BROWSER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	// Navigate to URL
	fmt.Println(pretty.InfoLine(fmt.Sprintf("Opening %s in headless browser...", url)))

	navigateBody := fmt.Sprintf(`{"url": "%s"}`, url)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/browser/navigate", strings.NewReader(navigateBody))
	if err != nil {
		return fmt.Errorf("navigate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("navigate failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("navigate returned status %d", resp.StatusCode)
	}

	// Wait for page load
	time.Sleep(2 * time.Second)

	// Capture screenshot
	if doScreenshot {
		if err := captureAPIScreenshot(client, baseURL, url, outputDir); err != nil {
			fmt.Println(pretty.ErrorLine(fmt.Sprintf("Screenshot capture failed: %v", err)))
		}
	}

	// Capture snapshot
	if doSnapshot {
		if err := captureAPISnapshot(client, baseURL, url, outputDir); err != nil {
			fmt.Println(pretty.ErrorLine(fmt.Sprintf("Snapshot capture failed: %v", err)))
		}
	}

	fmt.Println(pretty.SuccessLine("Headless browse completed"))
	return nil
}

// captureAPIScreenshot captures a screenshot via the browser control API.
func captureAPIScreenshot(client *http.Client, baseURL, pageURL, outputDir string) error {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/browser/screenshot", strings.NewReader(`{"format":"png"}`))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("screenshot request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("screenshot returned status %d", resp.StatusCode)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Generate filename from URL
	filename := sanitizeFilename(pageURL) + ".png"
	path := filepath.Join(outputDir, filename)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Println(pretty.SuccessLine(fmt.Sprintf("Screenshot saved to %s", path)))
	return nil
}

// captureAPISnapshot captures a DOM snapshot via the browser control API.
func captureAPISnapshot(client *http.Client, baseURL, pageURL, outputDir string) error {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/browser/snapshot", strings.NewReader(`{"format":"markdown"}`))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("snapshot request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("snapshot returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	// Parse the response for the snapshot content
	var result struct {
		Snapshot string `json:"snapshot"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// If it's plain text, use it directly
		result.Snapshot = string(body)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	filename := sanitizeFilename(pageURL) + ".md"
	path := filepath.Join(outputDir, filename)

	if err := os.WriteFile(path, []byte(result.Snapshot), 0644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	fmt.Println(pretty.SuccessLine(fmt.Sprintf("Snapshot saved to %s", path)))
	return nil
}

// captureLocalScreenshot attempts a local screenshot using system tools.
func captureLocalScreenshot(url, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	filename := sanitizeFilename(url) + ".png"
	path := filepath.Join(outputDir, filename)

	// Try using the forge screenshot tool if available
	if _, err := exec.LookPath("forge"); err == nil {
		cmd := exec.Command("forge", "screenshot", "--output", path, url)
		if err := cmd.Run(); err == nil {
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Screenshot saved to %s", path)))
			return nil
		}
	}

	fmt.Println(pretty.InfoLine("Local screenshot capture not available — use --headless for API-based screenshots"))
	return nil
}

// sanitizeFilename converts a URL to a safe filename.
func sanitizeFilename(url string) string {
	// Remove protocol
	s := strings.TrimPrefix(url, "https://")
	s = strings.TrimPrefix(s, "http://")
	// Replace unsafe characters
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
	// Truncate if too long
	if len(s) > 80 {
		s = s[:80]
	}
	// Add timestamp for uniqueness
	return fmt.Sprintf("%s_%d", s, time.Now().Unix())
}
