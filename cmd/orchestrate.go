package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// AgentInstance represents a running agent process
type AgentInstance struct {
	Name    string
	Type    string
	Port    int
	Model   string
	Cmd     *exec.Cmd
	Running bool
	Jailed  bool
	mu      sync.Mutex
}

// AgentOrchestrator manages multiple agent instances
type AgentOrchestrator struct {
	agents  map[string]*AgentInstance
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	verbose bool
}

func NewAgentOrchestrator(ctx context.Context, verbose bool) *AgentOrchestrator {
	ctx, cancel := context.WithCancel(ctx)
	return &AgentOrchestrator{
		agents:  make(map[string]*AgentInstance),
		ctx:     ctx,
		cancel:  cancel,
		verbose: verbose,
	}
}

// StartAgent launches a new agent instance via agentapi
func (o *AgentOrchestrator) StartAgent(name, agentType, model string, port int, jail bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.agents[name]; exists {
		return fmt.Errorf("agent %q already running", name)
	}

	// Find agentapi binary
	agentAPIPath, err := findAgentAPI()
	if err != nil {
		return fmt.Errorf("agentapi not available: %w", err)
	}

	if port == 0 {
		port, err = findFreePort()
		if err != nil {
			return fmt.Errorf("no free port: %w", err)
		}
	}

	// Build agentapi arguments
	apiArgs := []string{"server"}
	if agentType != "" && agentType != "claude" {
		apiArgs = append(apiArgs, "--type", agentType)
	}
	apiArgs = append(apiArgs, "--port", strconv.Itoa(port))
	apiArgs = append(apiArgs, "--")

	// Determine the agent binary
	agentBin := agentType
	if agentType == "" || agentType == "claude" {
		agentBin = "claude"
	}
	apiArgs = append(apiArgs, agentBin)

	// Create the process
	var cmd *exec.Cmd
	env := os.Environ()

	if jail {
		httpjailPath, err := exec.LookPath("httpjail")
		if err == nil {
			jailArgs := []string{"--allow", "github.com", "--", agentAPIPath}
			jailArgs = append(jailArgs, apiArgs...)
			cmd = exec.CommandContext(o.ctx, httpjailPath, jailArgs...)
		} else {
			cmd = exec.CommandContext(o.ctx, agentAPIPath, apiArgs...)
		}
	} else {
		cmd = exec.CommandContext(o.ctx, agentAPIPath, apiArgs...)
	}

	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start agent %q: %w", name, err)
	}

	agent := &AgentInstance{
		Name:    name,
		Type:    agentType,
		Port:    port,
		Model:   model,
		Cmd:     cmd,
		Jailed:  jail,
		Running: true,
	}

	o.agents[name] = agent

	// Wait for the agent to be ready
	go func() {
		if err := waitForPort(o.ctx, port, 30*time.Second); err != nil {
			agent.mu.Lock()
			agent.Running = false
			agent.mu.Unlock()
			fmt.Printf("Forge: Agent %q failed to start: %v\n", name, err)
			return
		}
		fmt.Printf("Forge: Agent %q ready on port %d\n", name, port)
	}()

	// Monitor process exit
	go func() {
		err := cmd.Wait()
		agent.mu.Lock()
		agent.Running = false
		agent.mu.Unlock()
		if err != nil {
			fmt.Printf("Forge: Agent %q exited: %v\n", name, err)
		} else {
			fmt.Printf("Forge: Agent %q stopped\n", name)
		}
	}()

	return nil
}

// StopAgent stops a running agent
func (o *AgentOrchestrator) StopAgent(name string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	agent, exists := o.agents[name]
	if !exists {
		return fmt.Errorf("agent %q not found", name)
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	if !agent.Running {
		return fmt.Errorf("agent %q not running", name)
	}

	agent.Cmd.Process.Signal(syscall.SIGTERM)
	time.Sleep(3 * time.Second)
	agent.Cmd.Process.Kill()
	agent.Running = false
	delete(o.agents, name)
	return nil
}

// ListAgents returns all agent instances
func (o *AgentOrchestrator) ListAgents() []AgentInstance {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]AgentInstance, 0, len(o.agents))
	for _, a := range o.agents {
		a.mu.Lock()
		result = append(result, AgentInstance{
			Name:    a.Name,
			Type:    a.Type,
			Port:    a.Port,
			Model:   a.Model,
			Running: a.Running,
			Jailed:  a.Jailed,
		})
		a.mu.Unlock()
	}
	return result
}

// SendMessage sends a message to a specific agent
func (o *AgentOrchestrator) SendMessage(agentName, content string) (string, error) {
	o.mu.RLock()
	agent, exists := o.agents[agentName]
	o.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("agent %q not found", agentName)
	}

	agent.mu.Lock()
	if !agent.Running {
		agent.mu.Unlock()
		return "", fmt.Errorf("agent %q not running", agentName)
	}
	port := agent.Port
	agent.mu.Unlock()

	// Send message via AgentAPI HTTP endpoint
	payload, _ := json.Marshal(map[string]string{
		"content": content,
		"type":    "user",
	})

	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/message", port), "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to send message to %q: %w", agentName, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

// GetMessages retrieves conversation history from an agent
func (o *AgentOrchestrator) GetMessages(agentName string) (string, error) {
	o.mu.RLock()
	agent, exists := o.agents[agentName]
	o.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("agent %q not found", agentName)
	}

	agent.mu.Lock()
	port := agent.Port
	running := agent.Running
	agent.mu.Unlock()

	if !running {
		return "", fmt.Errorf("agent %q not running", agentName)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/messages", port))
	if err != nil {
		return "", fmt.Errorf("failed to get messages from %q: %w", agentName, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

// Shutdown stops all agents
func (o *AgentOrchestrator) Shutdown() {
	o.cancel()
	o.mu.Lock()
	defer o.mu.Unlock()

	for name, agent := range o.agents {
		agent.mu.Lock()
		if agent.Running {
			agent.Cmd.Process.Signal(syscall.SIGTERM)
			time.Sleep(500 * time.Millisecond)
			agent.Cmd.Process.Kill()
			agent.Running = false
		}
		agent.mu.Unlock()
		delete(o.agents, name)
		fmt.Printf("Forge: Stopped agent %q\n", name)
	}
}

func orchestratorCmd() *cobra.Command {
	var verbose bool
	var basePort int

	cmd := &cobra.Command{
		Use:   "orchestrate",
		Short: "Run multiple AI agents concurrently with orchestration",
		Long: `Start multiple AI agents and manage them through a unified API.

Each agent runs in its own process with its own port.
Send messages to any agent, get conversation history,
and route between agents.

Examples:
  forge orchestrate --agents claude:3284,codex:3285
  forge orchestrate --agents claude:0,gemini:0 --jail
  forge orchestrate --agents claude,codex,gemini`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			orch := NewAgentOrchestrator(ctx, verbose)

			// Parse agent specs from --agents flag
			agentSpecs, _ := cmd.Flags().GetStringSlice("agents")
			jailFlag, _ := cmd.Flags().GetBool("jail")
			modelFlag, _ := cmd.Flags().GetString("model")

			fmt.Println("Forge: Orchestrator starting...")
			fmt.Printf("   Agents: %v\n", agentSpecs)
			fmt.Printf("   Base port: %d\n", basePort)
			fmt.Printf("   Jailed: %v\n", jailFlag)

			for i, spec := range agentSpecs {
				// Parse spec: "type" or "type:port"
				agentType := spec
				port := 0
				if idx := indexByte(spec, ':'); idx >= 0 {
					agentType = spec[:idx]
					if p, err := strconv.Atoi(spec[idx+1:]); err == nil && p > 0 {
						port = p
					}
				}

				if port == 0 {
					port = basePort + i
				}

				name := agentType
				if err := orch.StartAgent(name, agentType, modelFlag, port, jailFlag); err != nil {
					fmt.Printf("Forge: Failed to start %q: %v\n", name, err)
					continue
				}
				fmt.Printf("Forge: Started agent %q on port %d\n", name, port)
			}

			// Start a simple HTTP router on the orchestrate port
			orchPort := basePort - 1
			if orchPort < 1024 {
				orchPort = 8080
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/agents", func(w http.ResponseWriter, r *http.Request) {
				agents := orch.ListAgents()
				json.NewEncoder(w).Encode(agents)
			})
			mux.HandleFunc("/message/", func(w http.ResponseWriter, r *http.Request) {
				// URL: /message/{agentName}
				agentName := r.URL.Path[len("/message/"):]
				if r.Method != "POST" {
					http.Error(w, "POST only", http.StatusMethodNotAllowed)
					return
				}
				var body struct {
					Content string `json:"content"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				resp, err := orch.SendMessage(agentName, body.Content)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Write([]byte(resp))
			})
			mux.HandleFunc("/messages/", func(w http.ResponseWriter, r *http.Request) {
				agentName := r.URL.Path[len("/messages/"):]
				resp, err := orch.GetMessages(agentName)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Write([]byte(resp))
			})
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `Forge Orchestrator

Endpoints:
  GET  /agents           - List running agents
  POST /message/{name}   - Send message to agent {"content": "..."}
  GET  /messages/{name}  - Get conversation history from agent
`)
			})

			server := &http.Server{Addr: fmt.Sprintf(":%d", orchPort), Handler: mux}

			go func() {
				fmt.Printf("\nForge: Orchestrator API on http://localhost:%d\n", orchPort)
				fmt.Println("Forge: The wielder and the sword are one.")
				if err := server.ListenAndServe(); err != http.ErrServerClosed {
					fmt.Printf("Forge: Server error: %v\n", err)
				}
			}()

			// Wait for signal
			select {
			case <-sigChan:
				fmt.Println("\nForge: Cooling down...")
			case <-ctx.Done():
			}

			orch.Shutdown()
			server.Shutdown(context.Background())
			fmt.Println("Forge: The Forge cools.")
			return nil
		},
	}

	cmd.Flags().StringSlice("agents", []string{"claude"}, "Agent specs (type or type:port, comma-separated)")
	cmd.Flags().IntVar(&basePort, "base-port", 3284, "Base port for agent instances")
	var jail bool
	var model string
	cmd.Flags().BoolVarP(&jail, "jail", "j", false, "Enable httpjail for all agents")
	cmd.Flags().StringVarP(&model, "model", "m", "anthropic/claude-sonnet-4-20250514", "Model for all agents")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose logging")

	return cmd
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
