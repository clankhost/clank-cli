package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/sse"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <service-id>",
	Short: "Stream logs for a service",
	Long: `Stream runtime logs from the active container. Use --build to stream
build logs from a specific deployment instead.`,
	Args: cobra.ExactArgs(1),
	RunE: runLogs,
}

// runtimeLogData is the JSON payload for runtime_log SSE events.
type runtimeLogData struct {
	Line string `json:"line"`
}

func runLogs(cmd *cobra.Command, args []string) error {
	serviceID := args[0]
	buildDeploymentID, _ := cmd.Flags().GetString("build")
	tail, _ := cmd.Flags().GetInt("tail")

	client := newClient()

	// Set up signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	var path string
	var eventType string

	if buildDeploymentID != "" {
		// Build logs for a specific deployment.
		path = fmt.Sprintf("/api/deployments/%s/logs/stream", buildDeploymentID)
		eventType = "build_log"
	} else {
		// Runtime logs for the active container.
		path = fmt.Sprintf("/api/services/%s/logs/stream?tail=%d", serviceID, tail)
		eventType = "runtime_log"
	}

	body, err := client.SSEStream(path)
	if err != nil {
		if api.IsNotFound(err) {
			return fmt.Errorf("no active deployment found for this service")
		}
		return err
	}

	reader := sse.NewReader(body)
	defer reader.Close()

	// Stream in a goroutine so we can listen for signals.
	errCh := make(chan error, 1)
	go func() {
		for {
			evt, err := reader.Next()
			if err == io.EOF {
				errCh <- nil
				return
			}
			if err != nil {
				errCh <- err
				return
			}

			if evt.Type == eventType {
				switch eventType {
				case "runtime_log":
					var data runtimeLogData
					if json.Unmarshal([]byte(evt.Data), &data) == nil {
						fmt.Println(data.Line)
					}
				case "build_log":
					var data buildLogData
					if json.Unmarshal([]byte(evt.Data), &data) == nil {
						fmt.Println(data.Line)
					}
				}
			}
		}
	}()

	select {
	case <-sigCh:
		fmt.Fprintln(os.Stderr, "\nDisconnected.")
		return nil
	case err := <-errCh:
		return err
	}
}

func init() {
	logsCmd.Flags().Int("tail", 200, "number of lines to tail (runtime logs only)")
	logsCmd.Flags().String("build", "", "stream build logs for a specific deployment ID")
	rootCmd.AddCommand(logsCmd)
}
