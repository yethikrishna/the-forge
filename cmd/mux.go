package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"github.com/forge/sword/internal/acp"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

// MuxAgent represents an agent in the mux layout.
type MuxAgent struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
	LastMsg  string `json:"last_msg,omitempty"`
}

// MuxLayout defines how agents are arranged.
type MuxLayout struct {
	Agents []MuxAgent `json:"agents"`
	Layout string     `json:"layout"` // "grid", "tabs", "split"
}

func muxCmd() *cobra.Command {
	var basePort int
	var layout string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "mux",
		Short: "Run multiple agents in parallel with a unified interface",
		Long: `Launch multiple AI agents in parallel, each in its own
workspace, with a unified web dashboard to monitor and interact
with all of them.

Examples:
  forge mux --agents claude,codex,gemini
  forge mux --agents claude:3284,codex:3285,gemini:3286 --layout grid
  forge mux --agents claude,codex --base-port 4000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			agentSpecs, _ := cmd.Flags().GetStringSlice("agents")

			muxLayout := MuxLayout{
				Layout: layout,
			}

			var mu sync.Mutex

			fmt.Println(pretty.HeaderLine("Forge Mux — Parallel Agent Desktop"))
			fmt.Printf("   Layout: %s\n", layout)
			fmt.Printf("   Agents: %v\n", agentSpecs)
			fmt.Println()

			// Set up agents
			for i, spec := range agentSpecs {
				agentType := spec
				port := basePort + i

				// Parse spec: "type" or "type:port"
				if idx := stringsIndexByte(spec, ':'); idx >= 0 {
					agentType = spec[:idx]
					if p, err := strconv.Atoi(spec[idx+1:]); err == nil && p > 0 {
						port = p
					}
				}

				agent := MuxAgent{
					Name:   agentType,
					Type:   agentType,
					Port:   port,
					URL:    fmt.Sprintf("http://localhost:%d", port),
					Status: "configured",
				}

				muxLayout.Agents = append(muxLayout.Agents, agent)

				fmt.Printf("  %-15s port %d  %s\n", agentType, port, agent.URL)
			}

			// Start orchestration server
			orchPort := basePort - 1
			if orchPort < 1024 {
				orchPort = 8080
			}

			mux := http.NewServeMux()

			mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				json.NewEncoder(w).Encode(muxLayout)
			})

			mux.HandleFunc("/api/agent/", func(w http.ResponseWriter, r *http.Request) {
				name := r.URL.Path[len("/api/agent/"):]
				mu.Lock()
				defer mu.Unlock()

				for _, a := range muxLayout.Agents {
					if a.Name == name {
						json.NewEncoder(w).Encode(a)
						return
					}
				}
				http.Error(w, "agent not found", http.StatusNotFound)
			})

			mux.HandleFunc("/api/message/", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "POST only", http.StatusMethodNotAllowed)
					return
				}

				name := r.URL.Path[len("/api/message/"):]
				var body struct{ Content string `json:"content"` }
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				// Find agent and send message
				mu.Lock()
				var agentURL string
				for _, a := range muxLayout.Agents {
					if a.Name == name {
						agentURL = a.URL
						break
					}
				}
				mu.Unlock()

				if agentURL == "" {
					http.Error(w, "agent not found", http.StatusNotFound)
					return
				}

				client := acp.NewClient(agentURL)
				msg, err := client.SendMessage(r.Context(), body.Content)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}

				json.NewEncoder(w).Encode(msg)
			})

			mux.HandleFunc("/api/broadcast", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "POST only", http.StatusMethodNotAllowed)
					return
				}

				var body struct{ Content string `json:"content"` }
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				// Send to all agents
				mu.Lock()
				agents := make([]MuxAgent, len(muxLayout.Agents))
				copy(agents, muxLayout.Agents)
				mu.Unlock()

				results := make(map[string]string)
				for _, a := range agents {
					client := acp.NewClient(a.URL)
					msg, err := client.SendMessage(r.Context(), body.Content)
					if err != nil {
						results[a.Name] = fmt.Sprintf("error: %v", err)
					} else {
						results[a.Name] = msg.Content
					}
				}

				json.NewEncoder(w).Encode(results)
			})

			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, muxDashboardHTML)
			})

			server := &http.Server{
				Addr:    fmt.Sprintf(":%d", orchPort),
				Handler: mux,
			}

			go func() {
				fmt.Printf("\nForge: Mux dashboard on http://localhost:%d\n", orchPort)
				fmt.Println("  The wielder and the sword are one.")
				if err := server.ListenAndServe(); err != http.ErrServerClosed {
					fmt.Printf("Forge: Server error: %v\n", err)
				}
			}()

			select {
			case <-sigChan:
				fmt.Println("\nForge: Cooling down...")
			case <-ctx.Done():
			}

			server.Shutdown(context.Background())
			return nil
		},
	}

	cmd.Flags().StringSlice("agents", []string{"claude"}, "Agent specs (type or type:port)")
	cmd.Flags().IntVar(&basePort, "base-port", 3284, "Base port for agents")
	cmd.Flags().StringVarP(&layout, "layout", "l", "grid", "Dashboard layout (grid|tabs|split)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose logging")

	return cmd
}

func stringsIndexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

const muxDashboardHTML = `<!DOCTYPE html>
<html>
<head>
<title>Forge Mux — Parallel Agent Desktop</title>
<style>
body { font-family: system-ui, -apple-system, sans-serif; margin: 0; background: #0a0a0a; color: #e0e0e0; }
header { padding: 20px; background: #1a1a2e; border-bottom: 2px solid #e94560; }
h1 { margin: 0; font-size: 1.5em; }
.sword { color: #e94560; }
#agents { display: grid; grid-template-columns: repeat(auto-fit, minmax(400px, 1fr)); gap: 16px; padding: 20px; }
.agent-card { background: #16213e; border-radius: 8px; padding: 16px; border: 1px solid #0f3460; }
.agent-card h2 { margin: 0 0 8px 0; font-size: 1.1em; color: #e94560; }
.agent-card .status { font-size: 0.85em; color: #888; }
.agent-card .url { font-size: 0.85em; color: #0f3460; word-break: break-all; }
.broadcast { padding: 20px; }
.broadcast input { width: 60%%; padding: 8px; background: #16213e; border: 1px solid #0f3460; color: #e0e0e0; border-radius: 4px; }
.broadcast button { padding: 8px 16px; background: #e94560; border: none; color: white; border-radius: 4px; cursor: pointer; }
.broadcast button:hover { background: #c73650; }
</style>
</head>
<body>
<header>
<h1><span class="sword">⚔️</span> Forge Mux — Parallel Agent Desktop</h1>
</header>
<div class="broadcast">
<input id="broadcast-msg" placeholder="Send message to all agents..." />
<button onclick="broadcast()">Broadcast</button>
</div>
<div id="agents">Loading agents...</div>
<script>
setInterval(async () => {
  try {
    const resp = await fetch('/api/agents');
    const data = await resp.json();
    const container = document.getElementById('agents');
    container.innerHTML = data.agents.map(a => 
      '<div class="agent-card"><h2>' + a.name + '</h2>' +
      '<div class="status">Status: ' + a.status + '</div>' +
      '<div class="url">' + a.url + '</div></div>'
    ).join('');
  } catch(e) {}
}, 3000);

async function broadcast() {
  const msg = document.getElementById('broadcast-msg').value;
  if (!msg) return;
  await fetch('/api/broadcast', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({content: msg})
  });
  document.getElementById('broadcast-msg').value = '';
}
</script>
</body>
</html>`
