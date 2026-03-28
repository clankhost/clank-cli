package cmd

import (
	"fmt"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

var resourcesCmd = &cobra.Command{
	Use:   "resources <service-id>",
	Short: "View resource limits for a service",
	Long:  "View the CPU and memory limits configured for a service, along with server capacity and allocation info.",
	Args:  cobra.ExactArgs(1),
	Example: `  clank resources ac84e200-6f84-4110-99ed-4188c5f97130
  clank resources ac84e200-6f84-4110-99ed-4188c5f97130 --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		svc, err := api.GetService(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			data := map[string]any{
				"cpu_limit":       svc.CPULimit,
				"memory_limit_mb": svc.MemoryLimitMB,
			}
			if svc.ServerID != nil {
				if cap, alloc, err := serverCapacity(client, *svc.ServerID); err == nil {
					data["server"] = cap
					data["allocation"] = alloc
				}
			}
			return output.JSON(data)
		}

		fmt.Printf("CPU:     %.2g vCPU\n", svc.CPULimit)
		fmt.Printf("Memory:  %d MB\n", svc.MemoryLimitMB)

		if svc.ServerID != nil {
			if cap, alloc, err := serverCapacity(client, *svc.ServerID); err == nil {
				fmt.Printf("Server:  %s (%d cores, %d MB)\n", cap.Name, cap.CPUCores, cap.MemoryMB)
				oversubscribed := alloc.TotalCPU > float64(cap.CPUCores) || alloc.TotalMemoryMB > cap.MemoryMB
				suffix := ""
				if oversubscribed {
					suffix = " (oversubscribed!)"
				}
				fmt.Printf("Usage:   CPU %.2g/%d cores · Memory %d/%d MB%s\n",
					alloc.TotalCPU, cap.CPUCores, alloc.TotalMemoryMB, cap.MemoryMB, suffix)
			}
		}

		return nil
	},
}

var resourcesSetCmd = &cobra.Command{
	Use:   "set <service-id>",
	Short: "Set CPU and/or memory limits for a service",
	Long:  "Update the CPU and/or memory limits for a service. Changes take effect on next deploy.",
	Args:  cobra.ExactArgs(1),
	Example: `  clank resources set <service-id> --cpu 1
  clank resources set <service-id> --memory 2048
  clank resources set <service-id> --cpu 0.5 --memory 1024`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]
		cpuFlag := cmd.Flags().Lookup("cpu")
		memFlag := cmd.Flags().Lookup("memory")
		cpuChanged := cpuFlag.Changed
		memChanged := memFlag.Changed

		if !cpuChanged && !memChanged {
			return fmt.Errorf("at least one of --cpu or --memory is required")
		}

		var req api.UpdateResourcesRequest

		if cpuChanged {
			cpu, _ := cmd.Flags().GetFloat64("cpu")
			if cpu <= 0 {
				return fmt.Errorf("--cpu must be greater than 0")
			}
			req.CPULimit = &cpu
		}

		if memChanged {
			mem, _ := cmd.Flags().GetInt("memory")
			if mem < 128 {
				return fmt.Errorf("--memory must be at least 128 MB")
			}
			req.MemoryLimitMB = &mem
		}

		client := newClient()

		// Fetch service to get server_id for capacity checks.
		svc, err := api.GetService(client, serviceID)
		if err != nil {
			return err
		}

		if svc.ServerID != nil {
			if err := checkCapacity(client, *svc.ServerID, serviceID, req); err != nil {
				return err
			}
		}

		updated, err := api.UpdateServiceResources(client, serviceID, req)
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(updated)
		}

		fmt.Printf("CPU:     %.2g vCPU\n", updated.CPULimit)
		fmt.Printf("Memory:  %d MB\n", updated.MemoryLimitMB)
		fmt.Println("\nNote: Redeploy to apply new resource limits.")
		return nil
	},
}

// capacityInfo holds server capacity details.
type capacityInfo struct {
	Name     string `json:"name"`
	CPUCores int    `json:"cpu_cores"`
	MemoryMB int    `json:"memory_mb"`
}

// allocationInfo holds the total resource allocation across all services on a server.
type allocationInfo struct {
	TotalCPU      float64 `json:"total_cpu"`
	TotalMemoryMB int     `json:"total_memory_mb"`
	ServiceCount  int     `json:"service_count"`
}

// serverCapacity fetches server info and computes total allocation.
func serverCapacity(client *api.Client, serverID string) (*capacityInfo, *allocationInfo, error) {
	server, err := api.GetServer(client, serverID)
	if err != nil {
		return nil, nil, err
	}

	if server.CPUCores == nil || server.MemoryBytes == nil {
		return nil, nil, fmt.Errorf("server capacity unknown")
	}

	cap := &capacityInfo{
		Name:     server.Name,
		CPUCores: *server.CPUCores,
		MemoryMB: int(*server.MemoryBytes / (1024 * 1024)),
	}

	services, err := api.ListServerServices(client, serverID)
	if err != nil {
		return cap, nil, err
	}

	alloc := &allocationInfo{ServiceCount: len(services)}
	for _, s := range services {
		if s.CPULimit != nil {
			alloc.TotalCPU += *s.CPULimit
		}
		if s.MemoryLimitMB != nil {
			alloc.TotalMemoryMB += *s.MemoryLimitMB
		}
	}

	return cap, alloc, nil
}

// checkCapacity validates resource limits against server capacity and warns on oversubscription.
func checkCapacity(client *api.Client, serverID, serviceID string, req api.UpdateResourcesRequest) error {
	server, err := api.GetServer(client, serverID)
	if err != nil {
		return nil // graceful degradation — skip checks
	}

	if server.CPUCores == nil || server.MemoryBytes == nil {
		return nil // capacity unknown, skip
	}

	serverCPU := *server.CPUCores
	serverMemMB := int(*server.MemoryBytes / (1024 * 1024))

	// Hard reject: single service exceeds total server capacity.
	if req.CPULimit != nil && *req.CPULimit > float64(serverCPU) {
		return fmt.Errorf("CPU limit %.2g exceeds server capacity (%d cores)", *req.CPULimit, serverCPU)
	}
	if req.MemoryLimitMB != nil && *req.MemoryLimitMB > serverMemMB {
		return fmt.Errorf("memory limit %d MB exceeds server capacity (%d MB)", *req.MemoryLimitMB, serverMemMB)
	}

	// Soft warning: total allocation across all services exceeds capacity.
	services, err := api.ListServerServices(client, serverID)
	if err != nil {
		return nil // can't check, proceed anyway
	}

	var totalCPU float64
	var totalMem int
	for _, s := range services {
		if s.ID == serviceID {
			// Use the new values for the service being updated.
			if req.CPULimit != nil {
				totalCPU += *req.CPULimit
			} else if s.CPULimit != nil {
				totalCPU += *s.CPULimit
			}
			if req.MemoryLimitMB != nil {
				totalMem += *req.MemoryLimitMB
			} else if s.MemoryLimitMB != nil {
				totalMem += *s.MemoryLimitMB
			}
		} else {
			if s.CPULimit != nil {
				totalCPU += *s.CPULimit
			}
			if s.MemoryLimitMB != nil {
				totalMem += *s.MemoryLimitMB
			}
		}
	}

	if totalCPU > float64(serverCPU) || totalMem > serverMemMB {
		fmt.Printf("Warning: Server will be oversubscribed — total CPU: %.2g/%d cores, total memory: %d/%d MB\n",
			totalCPU, serverCPU, totalMem, serverMemMB)
	}

	return nil
}

func init() {
	resourcesSetCmd.Flags().Float64("cpu", 0, "CPU limit in vCPU units (e.g., 0.5, 1, 2)")
	resourcesSetCmd.Flags().Int("memory", 0, "Memory limit in MB (e.g., 512, 1024, 2048)")

	resourcesCmd.AddCommand(resourcesSetCmd)
	rootCmd.AddCommand(resourcesCmd)
}
