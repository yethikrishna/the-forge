// Package pair provides interactive human-agent pair programming.
// Two hands on the sword strike truer than one.
package pair

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Role represents who is acting.
type Role string

const (
	RoleHuman Role = "human"
	RoleAgent Role = "agent"
)

// Turn represents a single turn in the pair session.
type Turn struct {
	Role      Role              `json:"role"`
	Content   string            `json:"content"`
	Timestamp time.Time         `json:"timestamp"`
	Files     map[string]string `json:"files,omitempty"` // file snapshots
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Session is a pair programming session.
type Session struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Turns       []Turn    `json:"turns"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Status      string    `json:"status"` // active, paused, completed
	Files       map[string]string `json:"files,omitempty"`
	Mode        string    `json:"mode"` // drive, navigate, observe
}

// Mode determines who leads.
const (
	ModeDrive    = "drive"    // Agent drives, human reviews
	ModeNavigate = "navigate" // Human drives, agent assists
	ModeObserve  = "observe"  // Agent observes and suggests
)

// Pair manages a pair programming session.
type Pair struct {
	session   *Session
	output    io.Writer
	input     io.Reader
	onAgent   func(ctx context.Context, turns []Turn) (string, error)
	mu        sync.Mutex
	storePath string
}

// NewPair creates a new pair programming session.
func NewPair(name, mode string, onAgent func(ctx context.Context, turns []Turn) (string, error)) *Pair {
	return &Pair{
		session: &Session{
			ID:        fmt.Sprintf("pair-%d", time.Now().UnixNano()),
			Name:      name,
			Turns:     make([]Turn, 0),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			Status:    "active",
			Files:     make(map[string]string),
			Mode:      mode,
		},
		output:  os.Stdout,
		input:   os.Stdin,
		onAgent: onAgent,
	}
}

// WithIO sets custom input/output.
func (p *Pair) WithIO(in io.Reader, out io.Writer) *Pair {
	p.input = in
	p.output = out
	return p
}

// WithStore enables persistence.
func (p *Pair) WithStore(path string) *Pair {
	p.storePath = path
	return p
}

// Start begins the pair programming session.
func (p *Pair) Start(ctx context.Context) error {
	fmt.Fprintf(p.output, "\n=== Pair Programming Session: %s ===\n", p.session.Name)
	fmt.Fprintf(p.output, "Mode: %s\n", p.session.Mode)
	fmt.Fprintf(p.output, "Type your input. Use /help for commands, /quit to exit.\n\n")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintf(p.output, "\nSession interrupted. Saving...\n")
		p.session.Status = "paused"
		p.save()
		cancel()
	}()

	scanner := bufio.NewScanner(p.input)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Prompt based on mode
		prompt := "you"
		if p.session.Mode == ModeDrive {
			prompt = "review"
		}
		fmt.Fprintf(p.output, "[%s] > ", prompt)

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if handled := p.handleCommand(ctx, input); !handled {
				break // /quit
			}
			continue
		}

		// Record human turn
		p.addTurn(Turn{
			Role:      RoleHuman,
			Content:   input,
			Timestamp: time.Now().UTC(),
		})

		// Get agent response
		if p.onAgent != nil {
			resp, err := p.onAgent(ctx, p.session.Turns)
			if err != nil {
				fmt.Fprintf(p.output, "[agent error] %v\n", err)
				continue
			}

			p.addTurn(Turn{
				Role:      RoleAgent,
				Content:   resp,
				Timestamp: time.Now().UTC(),
			})

			fmt.Fprintf(p.output, "[agent] %s\n\n", resp)
		}
	}

	p.session.Status = "completed"
	p.save()
	return nil
}

// AddHumanTurn adds a human turn programmatically.
func (p *Pair) AddHumanTurn(content string) {
	p.addTurn(Turn{
		Role:      RoleHuman,
		Content:   content,
		Timestamp: time.Now().UTC(),
	})
}

// AddAgentTurn adds an agent turn programmatically.
func (p *Pair) AddAgentTurn(content string) {
	p.addTurn(Turn{
		Role:      RoleAgent,
		Content:   content,
		Timestamp: time.Now().UTC(),
	})
}

// Session returns the current session.
func (p *Pair) Session() *Session {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.session
}

// History returns all turns.
func (p *Pair) History() []Turn {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.session.Turns
}

// LastTurn returns the last turn.
func (p *Pair) LastTurn() *Turn {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.session.Turns) == 0 {
		return nil
	}
	return &p.session.Turns[len(p.session.Turns)-1]
}

// SetMode changes the pair mode.
func (p *Pair) SetMode(mode string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.session.Mode = mode
	fmt.Fprintf(p.output, "Mode changed to: %s\n", mode)
}

// Context returns the session context as a formatted string.
func (p *Pair) Context() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	var b strings.Builder
	for _, turn := range p.session.Turns {
		b.WriteString(fmt.Sprintf("[%s] %s\n", turn.Role, turn.Content))
	}
	return b.String()
}

// Export exports the session as JSON.
func (p *Pair) Export() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return json.MarshalIndent(p.session, "", "  ")
}

func (p *Pair) addTurn(turn Turn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.session.Turns = append(p.session.Turns, turn)
	p.session.UpdatedAt = time.Now().UTC()
	p.save()
}

func (p *Pair) handleCommand(ctx context.Context, cmd string) bool {
	parts := strings.Fields(cmd)
	command := parts[0]

	switch command {
	case "/quit", "/exit", "/q":
		fmt.Fprintf(p.output, "Ending pair session.\n")
		p.session.Status = "completed"
		p.save()
		return false
	case "/help", "/h", "/?":
		fmt.Fprintf(p.output, "Commands:\n")
		fmt.Fprintf(p.output, "  /help     — Show this help\n")
		fmt.Fprintf(p.output, "  /quit     — End session\n")
		fmt.Fprintf(p.output, "  /mode     — Change mode (drive, navigate, observe)\n")
		fmt.Fprintf(p.output, "  /history  — Show conversation history\n")
		fmt.Fprintf(p.output, "  /context  — Show full context\n")
		fmt.Fprintf(p.output, "  /export   — Export session as JSON\n")
		fmt.Fprintf(p.output, "  /files    — List tracked files\n")
		fmt.Fprintf(p.output, "  /stats    — Show session statistics\n")
		fmt.Fprintf(p.output, "  /undo     — Remove last turn\n")
		return true
	case "/mode":
		if len(parts) > 1 {
			p.SetMode(parts[1])
		} else {
			fmt.Fprintf(p.output, "Current mode: %s\n", p.session.Mode)
		}
		return true
	case "/history":
		for _, turn := range p.session.Turns {
			fmt.Fprintf(p.output, "[%s] %s\n", turn.Role, turn.Content)
		}
		return true
	case "/context":
		fmt.Fprintf(p.output, "%s\n", p.Context())
		return true
	case "/export":
		data, err := p.Export()
		if err != nil {
			fmt.Fprintf(p.output, "Export error: %v\n", err)
		} else {
			fmt.Fprintf(p.output, "%s\n", string(data))
		}
		return true
	case "/files":
		for path := range p.session.Files {
			fmt.Fprintf(p.output, "  %s\n", path)
		}
		return true
	case "/stats":
		fmt.Fprintf(p.output, "Session: %s\n", p.session.ID)
		fmt.Fprintf(p.output, "Turns:   %d\n", len(p.session.Turns))
		fmt.Fprintf(p.output, "Mode:    %s\n", p.session.Mode)
		fmt.Fprintf(p.output, "Files:   %d\n", len(p.session.Files))
		fmt.Fprintf(p.output, "Status:  %s\n", p.session.Status)
		return true
	case "/undo":
		p.mu.Lock()
		if len(p.session.Turns) > 0 {
			p.session.Turns = p.session.Turns[:len(p.session.Turns)-1]
			fmt.Fprintf(p.output, "Removed last turn.\n")
		}
		p.mu.Unlock()
		return true
	default:
		fmt.Fprintf(p.output, "Unknown command: %s\n", command)
		return true
	}
}

func (p *Pair) save() {
	if p.storePath == "" {
		return
	}
	data, err := json.MarshalIndent(p.session, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(p.storePath, data, 0o644)
}
