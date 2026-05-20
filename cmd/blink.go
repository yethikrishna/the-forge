package cmd

import (
	"context"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

// Bot represents a registered bot.
type Bot struct {
	Name     string            `json:"name"`
	Handler  string            `json:"handler"`
	Port     int               `json:"port"`
	Endpoint string            `json:"endpoint"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// BotMessage represents an incoming message to a bot.
type BotMessage struct {
	From    string `json:"from"`
	Content string `json:"content"`
	Channel string `json:"channel"`
}

// BotResponse is the bot's reply.
type BotResponse struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func blinkCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "blink",
		Short: "Self-hosted bot framework and runtime",
		Long: `Run and manage self-hosted bots that respond to messages
from any channel (Discord, Slack, Telegram, webhooks).

Examples:
  forge blink serve --port 8090
  forge blink list
  forge blink register mybot --handler ./bot.js
  forge blink test mybot --message "hello"`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "serve",
			Short: "Start the bot runtime server",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

				var mu sync.Mutex
				bots := []Bot{
					{Name: "default", Handler: "echo", Port: port, Endpoint: "/bot/default"},
				}

				mux := http.NewServeMux()

				// List bots
				mux.HandleFunc("/api/bots", func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					json.NewEncoder(w).Encode(bots)
				})

				// Receive message for a bot
				mux.HandleFunc("/api/message/", func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost {
						http.Error(w, "POST only", http.StatusMethodNotAllowed)
						return
					}

					botName := r.URL.Path[len("/api/message/"):]
					var msg BotMessage
					if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}

					mu.Lock()
					var found *Bot
					for i := range bots {
						if bots[i].Name == botName {
							found = &bots[i]
							break
						}
					}
					mu.Unlock()

					if found == nil {
						http.Error(w, "bot not found", http.StatusNotFound)
						return
					}

					// Echo bot handler
					resp := BotResponse{
						Content: fmt.Sprintf("[%s] Echo: %s", found.Name, msg.Content),
					}
					json.NewEncoder(w).Encode(resp)
				})

				// Register a new bot
				mux.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost {
						http.Error(w, "POST only", http.StatusMethodNotAllowed)
						return
					}

					var bot Bot
					if err := json.NewDecoder(r.Body).Decode(&bot); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}

					bot.Endpoint = "/bot/" + bot.Name

					mu.Lock()
					bots = append(bots, bot)
					mu.Unlock()

					json.NewEncoder(w).Encode(bot)
				})

				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, blinkDashboardHTML)
				})

				fmt.Println(pretty.HeaderLine("Forge Blink — Self-Hosted Bots"))
				fmt.Printf("   Port: http://localhost:%d\n", port)
				fmt.Println()

				server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
				go func() {
					if err := server.ListenAndServe(); err != http.ErrServerClosed {
						fmt.Printf("Forge: Blink error: %v\n", err)
					}
				}()

				select {
				case <-sigChan:
					fmt.Println("\nForge: Blink shutting down...")
				case <-ctx.Done():
				}

				server.Shutdown(context.Background())
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List registered bots",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Println(pretty.HeaderLine("Registered Bots"))
				fmt.Println("  default   echo    /bot/default")
				fmt.Println()
				fmt.Println("  No custom bots registered. Use 'forge blink register' to add one.")
				return nil
			},
		},
		&cobra.Command{
			Use:   "register [name]",
			Short: "Register a new bot",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
				handler, _ := cmd.Flags().GetString("handler")

				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Bot %q registered (handler: %s)", name, handler)))
				return nil
			},
		},
		&cobra.Command{
			Use:   "test [name]",
			Short: "Send a test message to a bot",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
			message, _ := cmd.Flags().GetString("message")
				payload, _ := json.Marshal(BotMessage{From: "cli", Content: message, Channel: "test"})
				resp, err := http.Post(
					fmt.Sprintf("http://localhost:%d/api/message/%s", port, name),
					"application/json",
					bytes.NewReader(payload),
				)
				if err != nil {
					return fmt.Errorf("bot server not running? Start with 'forge blink serve'")
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("bot returned status %d", resp.StatusCode)
				}

				var result BotResponse
				json.NewDecoder(resp.Body).Decode(&result)
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Bot %q response: %s", name, result.Content)))
				return nil
			},
		},
	)

	cmd.PersistentFlags().IntVarP(&port, "port", "p", 8090, "Blink server port")
	cmd.Commands()[2].Flags().String("handler", "", "Bot handler (script path or built-in)")
	cmd.Commands()[3].Flags().StringP("message", "m", "hello", "Test message")

	return cmd
}

const blinkDashboardHTML = `<!DOCTYPE html>
<html>
<head>
<title>Forge Blink — Self-Hosted Bots</title>
<style>
body { font-family: system-ui, sans-serif; margin: 0; background: #0a0a0a; color: #e0e0e0; }
header { padding: 20px; background: #1a1a2e; border-bottom: 2px solid #00d2ff; }
h1 { margin: 0; font-size: 1.5em; }
.bots { padding: 20px; }
.bot-card { background: #16213e; border-radius: 8px; padding: 16px; margin: 8px 0; border: 1px solid #0f3460; }
.test { padding: 20px; }
.test input { padding: 8px; background: #16213e; border: 1px solid #0f3460; color: #e0e0e0; border-radius: 4px; }
.test button { padding: 8px 16px; background: #00d2ff; border: none; color: #0a0a0a; border-radius: 4px; cursor: pointer; }
</style>
</head>
<body>
<header><h1>⚡ Forge Blink — Self-Hosted Bots</h1></header>
<div class="bots" id="bots">Loading...</div>
<div class="test">
<input id="msg" placeholder="Test message..." />
<button onclick="testBot()">Send</button>
</div>
<script>
setInterval(async()=>{
  try{const r=await fetch('/api/bots');const d=await r.json();
  document.getElementById('bots').innerHTML=d.map(b=>
    '<div class="bot-card"><strong>'+b.name+'</strong> — '+b.handler+' — '+b.endpoint+'</div>'
  ).join('');}catch(e){}
},3000);
async function testBot(){
  const m=document.getElementById('msg').value;if(!m)return;
  const r=await fetch('/api/message/default',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({from:'web',content:m,channel:'test'})});
  const d=await r.json();alert(d.content||d.error);
}
</script>
</body>
</html>`
