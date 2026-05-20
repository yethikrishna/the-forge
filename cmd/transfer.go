package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/wush"
	"github.com/spf13/cobra"
)

func transferCmd() *cobra.Command {
	var secret string
	var listenAddr string
	var outputDir string

	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "P2P encrypted file transfer",
		Long: `Transfer files peer-to-peer with encryption.

One side receives, the other side sends.

Examples:
  # Receiver (first)
  forge transfer receive --listen 0.0.0.0:9000 --output ./received

  # Sender (second)
  forge transfer send file.tar.gz --peer 1.2.3.4:9000`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "send [file]",
			Short: "Send a file to a peer",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				mgr := wush.NewManager()
				filePath := args[0]

				peerAddr, _ := cmd.Flags().GetString("peer")

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

				go func() {
					<-sigChan
					cancel()
				}()

				fmt.Println(pretty.InfoLine(fmt.Sprintf("Sending %s to %s", filePath, peerAddr)))

				transfer, err := mgr.Send(ctx, filePath, wush.TransferConfig{
					PeerAddr: peerAddr,
					Secret:   secret,
					OnProgress: func(sent, total int64) {
						pct := float64(sent) / float64(total) * 100
						fmt.Printf("\r  %s %.1f%% (%d/%d bytes)",
							pretty.ProgressBar(int(sent), int(total), 30), pct, sent, total)
					},
				})

				if err != nil {
					return err
				}

				fmt.Println()
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Transfer complete: %s (%d bytes, %v)",
					transfer.FileName, transfer.BytesTransferred, transfer.Duration())))
				return nil
			},
		},
		&cobra.Command{
			Use:   "receive",
			Short: "Receive a file from a peer",
			RunE: func(cmd *cobra.Command, args []string) error {
				mgr := wush.NewManager()

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

				go func() {
					<-sigChan
					cancel()
				}()

				fmt.Println(pretty.InfoLine(fmt.Sprintf("Listening on %s", listenAddr)))
				fmt.Println("  Waiting for incoming transfer...")

				transfer, err := mgr.Receive(ctx, listenAddr, outputDir)
				if err != nil {
					return err
				}

				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Received: %s (%d bytes, %v)",
					transfer.FileName, transfer.BytesTransferred, transfer.Duration())))
				return nil
			},
		},
	)

	cmd.PersistentFlags().StringVarP(&secret, "secret", "s", "", "Shared secret for encryption")
	cmd.PersistentFlags().StringVar(&listenAddr, "listen", "0.0.0.0:9000", "Listen address for receiving")
	cmd.PersistentFlags().StringVarP(&outputDir, "output", "o", ".", "Output directory for received files")

	// Send subcommand flags
	sendCmd := cmd.Commands()[0]
	sendCmd.Flags().String("peer", "", "Peer address (host:port)")

	return cmd
}
