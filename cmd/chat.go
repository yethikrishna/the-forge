package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/forge/sword/internal/aisdk"
	forgeslog "github.com/forge/sword/internal/slog"
	"github.com/forge/sword/internal/timer"
	"github.com/spf13/cobra"
)

func chatCmd() *cobra.Command {
	var provider string
	var model string
	var temperature float64
	var maxTokens int
	var stream bool
	var systemPrompt string

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Interactive terminal chat with any LLM model",
		Long: `Start an interactive chat session with any LLM model.
Supports OpenAI, Anthropic, Google, xAI, and custom providers.

Examples:
  forge chat
  forge chat -m claude-sonnet-4-20250514 -p anthropic
  forge chat -m gpt-5-mini -p openai
  forge chat -m gemini-2.5-pro -p google --stream
  forge chat --system "You are a Go expert"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Build model config
			config := aisdk.ModelConfig{
				Provider:    aisdk.Provider(provider),
				Model:       model,
				MaxTokens:   maxTokens,
				Temperature: temperature,
			}

			// Get API key from environment
			switch config.Provider {
			case aisdk.ProviderAnthropic:
				config.APIKey = os.Getenv("ANTHROPIC_API_KEY")
			case aisdk.ProviderOpenAI:
				config.APIKey = os.Getenv("OPENAI_API_KEY")
			case aisdk.ProviderGoogle:
				config.APIKey = os.Getenv("GOOGLE_API_KEY")
			case aisdk.ProviderXAI:
				config.APIKey = os.Getenv("XAI_API_KEY")
			}

			if config.APIKey == "" && config.Provider != aisdk.ProviderCustom {
				fmt.Printf("Forge: No API key found for %s\n", config.Provider)
				fmt.Printf("  Set environment variable (e.g. %s_API_KEY) and try again\n",
					strings.ToUpper(string(config.Provider)))
				return fmt.Errorf("missing API key for %s", config.Provider)
			}

			client := aisdk.NewClient(config)

			// Build message history
			var messages []aisdk.ChatMessage
			if systemPrompt != "" {
				messages = append(messages, aisdk.ChatMessage{
					Role:    "system",
					Content: systemPrompt,
				})
			}

			fmt.Println("Forge: Chat session started")
			fmt.Printf("   Provider: %s\n", provider)
			fmt.Printf("   Model:    %s\n", model)
			fmt.Printf("   Stream:   %v\n", stream)
			fmt.Println("   Type 'exit' or Ctrl+D to quit")
			fmt.Println()

			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Print("You: ")
				if !scanner.Scan() {
					break
				}
				input := strings.TrimSpace(scanner.Text())
				if input == "" {
					continue
				}
				if input == "exit" || input == "quit" {
					fmt.Println("Forge: Chat session ended")
					break
				}

				messages = append(messages, aisdk.ChatMessage{
					Role:    "user",
					Content: input,
				})

				tm := timer.New()

				if stream {
					ch, err := client.ChatStream(ctx, messages)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						continue
					}

					fmt.Print("AI: ")
					var fullContent strings.Builder
					for chunk := range ch {
						if len(chunk.Choices) > 0 {
							delta := chunk.Choices[0].Delta.Content
							fmt.Print(delta)
							fullContent.WriteString(delta)
						}
					}
					fmt.Println()

					messages = append(messages, aisdk.ChatMessage{
						Role:    "assistant",
						Content: fullContent.String(),
					})
				} else {
					resp, err := client.Chat(ctx, messages)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						continue
					}

					if len(resp.Choices) > 0 {
						content := resp.Choices[0].Message.Content
						fmt.Printf("AI: %s\n", content)

						messages = append(messages, aisdk.ChatMessage{
							Role:    "assistant",
							Content: content,
						})

						forgeslog.Debug("chat response",
							"tokens", resp.Usage.TotalTokens,
							"duration", tm.String(),
						)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "anthropic", "LLM provider (anthropic|openai|google|xai|custom)")
	cmd.Flags().StringVarP(&model, "model", "m", "claude-sonnet-4-20250514", "Model name")
	cmd.Flags().Float64Var(&temperature, "temperature", 0.7, "Sampling temperature")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 4096, "Max tokens in response")
	cmd.Flags().BoolVarP(&stream, "stream", "s", true, "Stream responses")
	cmd.Flags().StringVar(&systemPrompt, "system", "", "System prompt")

	return cmd
}
