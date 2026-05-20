package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/forge/sword/internal/envbuilder"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func envCmd() *cobra.Command {
	builder := envbuilder.NewBuilder()

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage development environments from Dockerfiles",
		Long: `Create, start, stop, and manage development environments
built from Dockerfiles or pre-built images.

Requires Docker installed and running.

Examples:
  forge env create --image golang:1.23 my-go-env
  forge env create --dockerfile ./Dockerfile my-env
  forge env create --template go my-go-env
  forge env start my-env
  forge env exec my-env -- go test ./...
  forge env stop my-env
  forge env list`,
	}

	var image string
	var dockerfile string
	var template string
	var port int

	cmd.AddCommand(
		&cobra.Command{
			Use:   "create [name]",
			Short: "Create a new development environment",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
				config := envbuilder.BuildConfig{
					Name: name,
					Port: port,
				}

				if image != "" {
					config.Image = image
				} else if dockerfile != "" {
					config.Dockerfile = dockerfile
				} else if template != "" {
					df, err := envbuilder.GenerateDockerfile(template)
					if err != nil {
						return err
					}
					config.Dockerfile = df
				} else {
					return fmt.Errorf("specify --image, --dockerfile, or --template")
				}

				if !builder.DockerAvailable() {
					return fmt.Errorf("Docker not available. Install Docker and try again")
				}

				env, err := builder.Build(context.Background(), config)
				if err != nil {
					return err
				}

				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Environment %q created", name)))
				fmt.Printf("   Image:  %s\n", env.Image)
				fmt.Printf("   Status: %s\n", env.Status)
				return nil
			},
		},
		&cobra.Command{
			Use:   "start [name]",
			Short: "Start a development environment",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				env, err := builder.Start(context.Background(), args[0])
				if err != nil {
					return err
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Environment %q started", args[0])))
				if env.Port > 0 {
					fmt.Printf("   Port: http://localhost:%d\n", env.Port)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "stop [name]",
			Short: "Stop a development environment",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := builder.Stop(context.Background(), args[0]); err != nil {
					return err
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Environment %q stopped", args[0])))
				return nil
			},
		},
		&cobra.Command{
			Use:   "rm [name]",
			Short: "Remove a development environment",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := builder.Remove(context.Background(), args[0]); err != nil {
					return err
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Environment %q removed", args[0])))
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List development environments",
			RunE: func(cmd *cobra.Command, args []string) error {
				envs := builder.List()
				if len(envs) == 0 {
					fmt.Println("Forge: No environments")
					return nil
				}
				for _, env := range envs {
					fmt.Printf("  %-20s %-10s %s\n", env.Name, env.Status, env.Image)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "exec [name] -- [command...]",
			Short: "Execute a command in a running environment",
			Args:  cobra.MinimumNArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				output, err := builder.Exec(context.Background(), args[0], args[1:])
				if err != nil {
					return err
				}
				fmt.Print(output)
				return nil
			},
		},
	)

	cmd.Flags().StringVarP(&image, "image", "i", "", "Pre-built Docker image")
	cmd.Flags().StringVarP(&dockerfile, "dockerfile", "d", "", "Path to Dockerfile")
	cmd.Flags().StringVarP(&template, "template", "t", "", "Language template (go|python|node|rust)")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "Port to expose")

	return cmd
}
