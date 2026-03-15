package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clankhost/clank-cli/internal/initdetect"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Prepare a project for deployment on Clank",
	Long: `Detect the project type, generate a Dockerfile if missing, and create
a clank.yaml configuration file. Does NOT deploy — just prepares.`,
	RunE: runInit,
}

// clankYAML is the structure written to clank.yaml.
type clankYAML struct {
	Service serviceConfig        `yaml:"service"`
	Env     map[string]string    `yaml:"env,omitempty"`
}

type serviceConfig struct {
	Name            string `yaml:"name"`
	Port            int    `yaml:"port"`
	HealthCheckPath string `yaml:"health_check_path"`
	Branch          string `yaml:"branch"`
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	nameOverride, _ := cmd.Flags().GetString("name")
	portOverride, _ := cmd.Flags().GetInt("port")

	// Detect project type.
	result := initdetect.Detect(dir)

	if nameOverride != "" {
		result.Name = nameOverride
	}
	if portOverride != 0 {
		result.Port = portOverride
	}

	if result.Type == initdetect.Unknown {
		fmt.Println("Could not detect project type.")
		fmt.Println("Supported: Node.js (SPA/server), Python, static HTML")
		fmt.Println("You can create a Dockerfile manually and re-run 'clank init'.")
	} else {
		fmt.Printf("Detected: %s (%s)\n", result.Type, result.Framework)
	}

	// Generate Dockerfile if missing.
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		if result.Type != initdetect.Unknown {
			content, err := initdetect.GenerateDockerfile(result)
			if err != nil {
				return fmt.Errorf("generating Dockerfile: %w", err)
			}
			if err := os.WriteFile(dockerfilePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("writing Dockerfile: %w", err)
			}
			fmt.Println("Created Dockerfile")
		}
	} else {
		fmt.Println("Dockerfile already exists, skipping.")
	}

	// Generate clank.yaml if missing.
	clankPath := filepath.Join(dir, "clank.yaml")
	if _, err := os.Stat(clankPath); os.IsNotExist(err) {
		config := clankYAML{
			Service: serviceConfig{
				Name:            result.Name,
				Port:            result.Port,
				HealthCheckPath: result.HealthCheckPath,
				Branch:          "main",
			},
			Env: map[string]string{
				"# DATABASE_URL": "",
				"# API_KEY":      "",
			},
		}

		data, err := yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("marshaling clank.yaml: %w", err)
		}

		header := "# Clank deployment configuration\n# Docs: https://github.com/anaremore/clank/blob/main/docs/production-setup.md\n\n"
		if err := os.WriteFile(clankPath, []byte(header+string(data)), 0644); err != nil {
			return fmt.Errorf("writing clank.yaml: %w", err)
		}
		fmt.Println("Created clank.yaml")
	} else {
		fmt.Println("clank.yaml already exists, skipping.")
	}

	// Summary.
	fmt.Println()
	fmt.Printf("  Service: %s\n", result.Name)
	fmt.Printf("  Port:    %d\n", result.Port)
	fmt.Printf("  Health:  %s\n", result.HealthCheckPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review the generated Dockerfile and clank.yaml")
	fmt.Println("  2. Push your code to a Git repository")
	fmt.Println("  3. Run: clank services create --project <id> --name <name> --repo <url>")
	fmt.Println("  4. Run: clank deploy <service-id>")

	return nil
}

func init() {
	initCmd.Flags().String("name", "", "override detected service name")
	initCmd.Flags().Int("port", 0, "override detected port")
	rootCmd.AddCommand(initCmd)
}
