package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/anaremore/clank/apps/cli/internal/sse"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy <service-id>",
	Short: "Trigger a deployment for a service",
	Long:  "Triggers a manual deploy and streams build logs + deployment events in real time.",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) error {
	serviceID := args[0]
	noFollow, _ := cmd.Flags().GetBool("no-follow")

	client := api.New(cfg.BaseURL, cfg.Token)

	deployment, err := api.TriggerDeploy(client, serviceID)
	if err != nil {
		if api.IsConflict(err) {
			return fmt.Errorf("a deployment is already in progress for this service")
		}
		return err
	}

	fmt.Printf("Deployment %s triggered (%s)\n", output.ShortID(deployment.ID), deployment.Status)

	if noFollow {
		fmt.Printf("Deployment ID: %s\n", deployment.ID)
		return nil
	}

	return followDeployment(client, deployment.ID)
}

// followDeployment streams build logs and deployment events until a terminal state.
func followDeployment(client *api.Client, deploymentID string) error {
	// Set up signal handling for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	var wg sync.WaitGroup
	done := make(chan struct{})
	exitCode := 0

	// Stream build logs.
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamBuildLogs(client, deploymentID, done)
	}()

	// Stream deployment events.
	wg.Add(1)
	go func() {
		defer wg.Done()
		code := streamDeployEvents(client, deploymentID, done)
		exitCode = code
	}()

	// Wait for signal or completion.
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-sigCh:
		fmt.Println("\nInterrupted. Deployment continues in background.")
		fmt.Printf("Check status: clank services info %s\n", deploymentID)
		return nil
	case <-done:
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	}
}

// buildLogData is the JSON payload for build_log SSE events.
type buildLogData struct {
	Line      string `json:"line"`
	Timestamp string `json:"timestamp"`
}

func streamBuildLogs(client *api.Client, deploymentID string, done chan struct{}) {
	body, err := client.SSEStream(fmt.Sprintf("/api/deployments/%s/logs/stream", deploymentID))
	if err != nil {
		// Build logs may not be available (e.g., rollback). Not fatal.
		return
	}

	reader := sse.NewReader(body)
	defer reader.Close()

	for {
		select {
		case <-done:
			return
		default:
		}

		evt, err := reader.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			return
		}

		if evt.Type == "build_log" {
			var data buildLogData
			if json.Unmarshal([]byte(evt.Data), &data) == nil {
				fmt.Println(data.Line)
			}
		}
	}
}

// deployEventData is the JSON payload for deployment_event SSE events.
type deployEventData struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func streamDeployEvents(client *api.Client, deploymentID string, done chan struct{}) int {
	body, err := client.SSEStream(fmt.Sprintf("/api/deployments/%s/events/stream", deploymentID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stream events: %v\n", err)
		// Fall back to polling.
		return pollFinalStatus(client, deploymentID)
	}

	reader := sse.NewReader(body)
	defer reader.Close()

	for {
		select {
		case <-done:
			return 0
		default:
		}

		evt, err := reader.Next()
		if err == io.EOF {
			// Stream ended — check final status.
			return pollFinalStatus(client, deploymentID)
		}
		if err != nil {
			return pollFinalStatus(client, deploymentID)
		}

		if evt.Type == "deployment_event" {
			var data deployEventData
			if json.Unmarshal([]byte(evt.Data), &data) == nil {
				fmt.Printf("--> %s: %s\n", output.StatusColor(data.EventType), data.Message)

				// Check for terminal states.
				status := extractStatus(data.EventType)
				if api.IsTerminalStatus(status) {
					if status == "active" {
						fmt.Println("\nDeployment succeeded!")
						return 0
					}
					fmt.Fprintf(os.Stderr, "\nDeployment %s.\n", status)
					return 1
				}
			}
		}
	}
}

// extractStatus converts event types like "deployment.active" to status "active".
func extractStatus(eventType string) string {
	parts := splitLast(eventType, ".")
	if len(parts) == 2 {
		return parts[1]
	}
	return eventType
}

func splitLast(s, sep string) []string {
	idx := len(s) - 1
	for idx >= 0 {
		if string(s[idx]) == sep {
			return []string{s[:idx], s[idx+1:]}
		}
		idx--
	}
	return []string{s}
}

// pollFinalStatus checks the deployment status when SSE stream is unavailable.
func pollFinalStatus(client *api.Client, deploymentID string) int {
	dep, err := api.GetDeployment(client, deploymentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not check deployment status: %v\n", err)
		return 1
	}

	fmt.Printf("Final status: %s\n", output.StatusColor(dep.Status))
	if dep.Status == "active" {
		return 0
	}
	if dep.ErrorMessage != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", *dep.ErrorMessage)
	}
	return 1
}

func init() {
	deployCmd.Flags().Bool("no-follow", false, "don't stream logs; just print deployment ID and exit")
	rootCmd.AddCommand(deployCmd)
}
