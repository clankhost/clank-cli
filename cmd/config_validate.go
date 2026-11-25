package cmd

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/clankhost/clank-cli/internal/output"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

const (
	clankConfigMaxBytes        = 64 * 1024
	clankMaxHealthCheckPathLen = 255
	clankMaxDockerfilePathLen  = 500
	clankMaxPreDeployCmdLen    = 500
	clankMinMemoryMB           = 64
	clankMaxMemoryMB           = 65536
	clankMinCPU                = 0.1
	clankMaxCPU                = 64.0
	clankMaxCronEntries        = 20
	clankMaxCronNameLen        = 100
	clankMaxCronScheduleLen    = 100
	clankMaxCronCommandLen     = 500
	clankMaxRequiredEnvVars    = 100
)

var clankEnvVarNameRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

var clankCronAliases = map[string]string{
	"@hourly":  "0 * * * *",
	"@daily":   "0 0 * * *",
	"@weekly":  "0 0 * * 0",
	"@monthly": "0 0 1 * *",
}

// ClankServiceConfig holds validated [service] settings from clank.toml.
type ClankServiceConfig struct {
	Port             *int    `json:"port,omitempty"`
	HealthCheckPath  *string `json:"health_check_path,omitempty"`
	DockerfilePath   *string `json:"dockerfile_path,omitempty"`
	PreDeployCommand *string `json:"pre_deploy_command,omitempty"`
}

// ClankResourcesConfig holds validated [resources] settings from clank.toml.
type ClankResourcesConfig struct {
	MemoryMB *int     `json:"memory_mb,omitempty"`
	CPU      *float64 `json:"cpu,omitempty"`
}

// ClankCronConfig holds one validated [[cron]] entry from clank.toml.
type ClankCronConfig struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Enabled  bool   `json:"enabled"`
}

// ClankEnvConfig holds validated [env] settings from clank.toml.
type ClankEnvConfig struct {
	Required []string `json:"required,omitempty"`
}

// ClankConfig is the parsed representation of clank.toml.
type ClankConfig struct {
	Service   ClankServiceConfig   `json:"service"`
	Resources ClankResourcesConfig `json:"resources"`
	Cron      []ClankCronConfig    `json:"cron,omitempty"`
	Env       ClankEnvConfig       `json:"env"`
}

type configValidateOutput struct {
	Valid  bool         `json:"valid"`
	Path   string       `json:"path"`
	Config *ClankConfig `json:"config,omitempty"`
	Errors []string     `json:"errors,omitempty"`
}

var configValidateCmd = &cobra.Command{
	Use:   "validate [path]",
	Short: "Validate a clank.toml service configuration file",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "clank.toml"
		if len(args) == 1 {
			path = args[0]
		}
		return runConfigValidate(path)
	},
}

func runConfigValidate(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if output.IsJSON() {
			_ = output.JSON(configValidateOutput{
				Valid:  false,
				Path:   path,
				Errors: []string{fmt.Sprintf("failed to read %s: %v", path, err)},
			})
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	cfg, errs := parseAndValidateClankToml(data)
	if len(errs) > 0 {
		if output.IsJSON() {
			_ = output.JSON(configValidateOutput{
				Valid:  false,
				Path:   path,
				Errors: errs,
			})
			return fmt.Errorf("clank.toml validation failed with %d error(s)", len(errs))
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Invalid %s:\n", path)
		for _, e := range errs {
			fmt.Fprintf(&b, "  - %s\n", e)
		}
		return errors.New(strings.TrimRight(b.String(), "\n"))
	}

	if output.IsJSON() {
		return output.JSON(configValidateOutput{
			Valid:  true,
			Path:   path,
			Config: cfg,
		})
	}

	printConfigSummary(path, cfg)
	return nil
}

func printConfigSummary(path string, cfg *ClankConfig) {
	fmt.Printf("Valid clank.toml: %s\n", path)
	if cfg.Service.Port != nil || cfg.Service.HealthCheckPath != nil || cfg.Service.DockerfilePath != nil || cfg.Service.PreDeployCommand != nil {
		fmt.Println("  [service]")
		if cfg.Service.Port != nil {
			fmt.Printf("    port:               %d\n", *cfg.Service.Port)
		}
		if cfg.Service.HealthCheckPath != nil {
			fmt.Printf("    health_check_path:  %s\n", *cfg.Service.HealthCheckPath)
		}
		if cfg.Service.DockerfilePath != nil {
			fmt.Printf("    dockerfile_path:    %s\n", *cfg.Service.DockerfilePath)
		}
		if cfg.Service.PreDeployCommand != nil {
			fmt.Printf("    pre_deploy_command: %s\n", *cfg.Service.PreDeployCommand)
		}
	}
	if cfg.Resources.MemoryMB != nil || cfg.Resources.CPU != nil {
		fmt.Println("  [resources]")
		if cfg.Resources.MemoryMB != nil {
			fmt.Printf("    memory_mb: %d\n", *cfg.Resources.MemoryMB)
		}
		if cfg.Resources.CPU != nil {
			fmt.Printf("    cpu:       %g\n", *cfg.Resources.CPU)
		}
	}
	if len(cfg.Cron) > 0 {
		fmt.Printf("  [[cron]] (%d job(s))\n", len(cfg.Cron))
		for _, c := range cfg.Cron {
			state := "enabled"
			if !c.Enabled {
				state = "disabled"
			}
			fmt.Printf("    - %s (%s, %s): %s\n", c.Name, c.Schedule, state, c.Command)
		}
	}
	if len(cfg.Env.Required) > 0 {
		fmt.Printf("  [env] required (%d): %s\n", len(cfg.Env.Required), strings.Join(cfg.Env.Required, ", "))
	}
}

func parseAndValidateClankToml(data []byte) (*ClankConfig, []string) {
	if len(data) > clankConfigMaxBytes {
		return nil, []string{fmt.Sprintf("clank.toml exceeds maximum size of %d bytes", clankConfigMaxBytes)}
	}
	if !utf8.Valid(data) {
		return nil, []string{"clank.toml is not valid UTF-8"}
	}

	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, []string{fmt.Sprintf("malformed clank.toml: %v", err)}
	}

	cfg := &ClankConfig{}
	var errs []string

	// [service]
	if svcRaw, ok := raw["service"]; ok {
		svcMap, isMap := svcRaw.(map[string]any)
		if !isMap {
			errs = append(errs, "clank.toml [service] section must be a table")
		} else {
			validateServiceSection(svcMap, &cfg.Service, &errs)
		}
	}

	// [resources]
	if resRaw, ok := raw["resources"]; ok {
		resMap, isMap := resRaw.(map[string]any)
		if !isMap {
			errs = append(errs, "clank.toml [resources] section must be a table")
		} else {
			validateResourcesSection(resMap, &cfg.Resources, &errs)
		}
	}

	// [[cron]]
	if cronRaw, ok := raw["cron"]; ok {
		cronSlice, isSlice := cronRaw.([]any)
		if !isSlice {
			errs = append(errs, "clank.toml [[cron]] section must be an array of tables")
		} else {
			validateCronSection(cronSlice, &cfg.Cron, &errs)
		}
	}

	// [env]
	if envRaw, ok := raw["env"]; ok {
		envMap, isMap := envRaw.(map[string]any)
		if !isMap {
			errs = append(errs, "clank.toml [env] section must be a table")
		} else {
			validateEnvSection(envMap, &cfg.Env, &errs)
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}
	return cfg, nil
}

func containsControlChars(s string) bool {
	return strings.ContainsAny(s, "\x00\n\r")
}

func asStrictInt(v any) (int, bool) {
	switch n := v.(type) {
	case int64:
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}

func asStrictFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func validateServiceSection(m map[string]any, out *ClankServiceConfig, errs *[]string) {
	if v, ok := m["port"]; ok {
		p, isInt := asStrictInt(v)
		if !isInt {
			*errs = append(*errs, "clank.toml [service].port must be an integer")
		} else if p < 1 || p > 65535 {
			*errs = append(*errs, "clank.toml [service].port must be between 1 and 65535")
		} else {
			out.Port = &p
		}
	}

	if v, ok := m["health_check_path"]; ok {
		s, isStr := v.(string)
		if !isStr {
			*errs = append(*errs, "clank.toml [service].health_check_path must be a string")
		} else if containsControlChars(s) {
			*errs = append(*errs, "clank.toml [service].health_check_path must not contain newlines or null bytes")
		} else if !strings.HasPrefix(s, "/") {
			*errs = append(*errs, "clank.toml [service].health_check_path must start with '/'")
		} else if len(s) > clankMaxHealthCheckPathLen {
			*errs = append(*errs, fmt.Sprintf("clank.toml [service].health_check_path must be at most %d characters", clankMaxHealthCheckPathLen))
		} else {
			out.HealthCheckPath = &s
		}
	}

	if v, ok := m["dockerfile_path"]; ok {
		s, isStr := v.(string)
		if !isStr {
			*errs = append(*errs, "clank.toml [service].dockerfile_path must be a string")
		} else if containsControlChars(s) {
			*errs = append(*errs, "clank.toml [service].dockerfile_path must not contain newlines or null bytes")
		} else {
			trimmed := strings.TrimSpace(s)
			if trimmed == "" {
				*errs = append(*errs, "clank.toml [service].dockerfile_path must not be empty")
			} else if len(trimmed) > clankMaxDockerfilePathLen {
				*errs = append(*errs, fmt.Sprintf("clank.toml [service].dockerfile_path must be at most %d characters", clankMaxDockerfilePathLen))
			} else if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "\\") || (len(trimmed) >= 2 && trimmed[1] == ':') {
				*errs = append(*errs, "clank.toml [service].dockerfile_path must be a relative path")
			} else if hasParentTraversal(trimmed) {
				*errs = append(*errs, "clank.toml [service].dockerfile_path must not contain '..'")
			} else {
				out.DockerfilePath = &trimmed
			}
		}
	}

	if v, ok := m["pre_deploy_command"]; ok {
		s, isStr := v.(string)
		if !isStr {
			*errs = append(*errs, "clank.toml [service].pre_deploy_command must be a string")
		} else if containsControlChars(s) {
			*errs = append(*errs, "clank.toml [service].pre_deploy_command must not contain newlines or null bytes")
		} else {
			trimmed := strings.TrimSpace(s)
			if len(trimmed) > clankMaxPreDeployCmdLen {
				*errs = append(*errs, fmt.Sprintf("clank.toml [service].pre_deploy_command must be at most %d characters", clankMaxPreDeployCmdLen))
			} else if trimmed != "" {
				out.PreDeployCommand = &trimmed
			}
		}
	}
}

func hasParentTraversal(p string) bool {
	for _, seg := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return true
		}
	}
	return false
}

func validateResourcesSection(m map[string]any, out *ClankResourcesConfig, errs *[]string) {
	if v, ok := m["memory_mb"]; ok {
		mb, isInt := asStrictInt(v)
		if !isInt {
			*errs = append(*errs, "clank.toml [resources].memory_mb must be an integer")
		} else if mb < clankMinMemoryMB || mb > clankMaxMemoryMB {
			*errs = append(*errs, fmt.Sprintf("clank.toml [resources].memory_mb must be between %d and %d", clankMinMemoryMB, clankMaxMemoryMB))
		} else {
			out.MemoryMB = &mb
		}
	}

	if v, ok := m["cpu"]; ok {
		c, isNum := asStrictFloat(v)
		if !isNum {
			*errs = append(*errs, "clank.toml [resources].cpu must be a number")
		} else if math.IsNaN(c) || math.IsInf(c, 0) {
			*errs = append(*errs, "clank.toml [resources].cpu must be a finite number")
		} else if c < clankMinCPU || c > clankMaxCPU {
			*errs = append(*errs, fmt.Sprintf("clank.toml [resources].cpu must be between %g and %g", clankMinCPU, clankMaxCPU))
		} else {
			out.CPU = &c
		}
	}
}

func validateCronSection(items []any, out *[]ClankCronConfig, errs *[]string) {
	if len(items) > clankMaxCronEntries {
		*errs = append(*errs, fmt.Sprintf("clank.toml [[cron]] supports at most %d entries", clankMaxCronEntries))
		return
	}

	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		label := fmt.Sprintf("clank.toml [[cron]][%d]", i)
		m, isMap := item.(map[string]any)
		if !isMap {
			*errs = append(*errs, fmt.Sprintf("%s must be a table", label))
			continue
		}

		var entry ClankCronConfig
		entry.Enabled = true

		// name
		if rawName, ok := m["name"].(string); !ok {
			*errs = append(*errs, fmt.Sprintf("%s.name is required and must be a string", label))
		} else if containsControlChars(rawName) {
			*errs = append(*errs, fmt.Sprintf("%s.name must not contain newlines or null bytes", label))
		} else {
			name := strings.TrimSpace(rawName)
			if name == "" || len(name) > clankMaxCronNameLen {
				*errs = append(*errs, fmt.Sprintf("%s.name must be between 1 and %d characters", label, clankMaxCronNameLen))
			} else if _, dup := seen[name]; dup {
				*errs = append(*errs, fmt.Sprintf("clank.toml [[cron]] has duplicate job name '%s'", name))
			} else {
				seen[name] = struct{}{}
				entry.Name = name
			}
		}

		// schedule
		if rawSched, ok := m["schedule"].(string); !ok {
			*errs = append(*errs, fmt.Sprintf("%s.schedule is required and must be a string", label))
		} else {
			sched := strings.TrimSpace(rawSched)
			if sched == "" || len(sched) > clankMaxCronScheduleLen {
				*errs = append(*errs, fmt.Sprintf("%s.schedule must be between 1 and %d characters", label, clankMaxCronScheduleLen))
			} else if err := validateCronExpression(sched); err != nil {
				*errs = append(*errs, fmt.Sprintf("%s.schedule is invalid: %v", label, err))
			} else {
				entry.Schedule = sched
			}
		}

		// command
		if rawCmd, ok := m["command"].(string); !ok {
			*errs = append(*errs, fmt.Sprintf("%s.command is required and must be a string", label))
		} else if containsControlChars(rawCmd) {
			*errs = append(*errs, fmt.Sprintf("%s.command must not contain newlines or null bytes", label))
		} else {
			cmd := strings.TrimSpace(rawCmd)
			if cmd == "" || len(cmd) > clankMaxCronCommandLen {
				*errs = append(*errs, fmt.Sprintf("%s.command must be between 1 and %d characters", label, clankMaxCronCommandLen))
			} else {
				entry.Command = cmd
			}
		}

		// enabled
		if rawEnabled, exists := m["enabled"]; exists {
			b, isBool := rawEnabled.(bool)
			if !isBool {
				*errs = append(*errs, fmt.Sprintf("%s.enabled must be a boolean", label))
			} else {
				entry.Enabled = b
			}
		}

		*out = append(*out, entry)
	}
}

func validateEnvSection(m map[string]any, out *ClankEnvConfig, errs *[]string) {
	rawReq, exists := m["required"]
	if !exists {
		return
	}
	reqSlice, isSlice := rawReq.([]any)
	if !isSlice {
		*errs = append(*errs, "clank.toml [env].required must be a list of strings")
		return
	}
	if len(reqSlice) > clankMaxRequiredEnvVars {
		*errs = append(*errs, fmt.Sprintf("clank.toml [env].required supports at most %d entries", clankMaxRequiredEnvVars))
		return
	}

	seen := make(map[string]struct{}, len(reqSlice))
	for i, item := range reqSlice {
		s, isStr := item.(string)
		if !isStr || !clankEnvVarNameRe.MatchString(s) {
			*errs = append(*errs, fmt.Sprintf("clank.toml [env].required[%d] must match ^[A-Z_][A-Z0-9_]*$", i))
			continue
		}
		if _, dup := seen[s]; !dup {
			seen[s] = struct{}{}
			out.Required = append(out.Required, s)
		}
	}
}

func validateCronExpression(expr string) error {
	cleaned := strings.TrimSpace(expr)
	if cleaned == "" {
		return errors.New("cron expression must not be empty")
	}

	var fields []string
	if strings.HasPrefix(cleaned, "@") {
		expanded, ok := clankCronAliases[strings.ToLower(cleaned)]
		if !ok {
			return fmt.Errorf("unknown cron alias '%s' (supported: @daily, @hourly, @monthly, @weekly)", cleaned)
		}
		fields = strings.Fields(expanded)
	} else {
		fields = strings.Fields(cleaned)
		if len(fields) != 5 {
			return fmt.Errorf("expected 5 fields (minute hour dom month dow), got %d", len(fields))
		}
	}

	specs := []struct {
		name string
		min  int
		max  int
	}{
		{"minute", 0, 59},
		{"hour", 0, 23},
		{"day of month", 1, 31},
		{"month", 1, 12},
		{"day of week", 0, 7},
	}

	for i, spec := range specs {
		if err := validateCronField(fields[i], spec.name, spec.min, spec.max); err != nil {
			return err
		}
	}
	return nil
}

func validateCronField(raw, fieldName string, minVal, maxVal int) error {
	if raw == "" {
		return fmt.Errorf("empty %s field", fieldName)
	}
	for _, part := range strings.Split(raw, ",") {
		if part == "" {
			return fmt.Errorf("empty list item in %s field '%s'", fieldName, raw)
		}
		base := part
		hasStep := false
		if strings.Contains(part, "/") {
			pieces := strings.Split(part, "/")
			if len(pieces) != 2 {
				return fmt.Errorf("invalid step syntax '%s' in %s field", part, fieldName)
			}
			base = pieces[0]
			step, err := strconv.Atoi(pieces[1])
			if err != nil || step <= 0 {
				return fmt.Errorf("invalid step '%s' in %s field", pieces[1], fieldName)
			}
			if base == "" {
				return fmt.Errorf("missing range before '/' in %s field '%s'", fieldName, part)
			}
			hasStep = true
		}

		if base == "*" {
			continue
		}
		if strings.Contains(base, "-") {
			bounds := strings.Split(base, "-")
			if len(bounds) != 2 {
				return fmt.Errorf("invalid range '%s' in %s field", base, fieldName)
			}
			start, err := parseCronInt(bounds[0], fieldName, minVal, maxVal)
			if err != nil {
				return err
			}
			end, err := parseCronInt(bounds[1], fieldName, minVal, maxVal)
			if err != nil {
				return err
			}
			if start > end {
				return fmt.Errorf("invalid range '%s' in %s field: start %d > end %d", base, fieldName, start, end)
			}
			continue
		}
		if hasStep {
			return fmt.Errorf("step syntax '%s' in %s field requires '*' or a range 'a-b'", part, fieldName)
		}
		if _, err := parseCronInt(base, fieldName, minVal, maxVal); err != nil {
			return err
		}
	}
	return nil
}

func parseCronInt(token, fieldName string, minVal, maxVal int) (int, error) {
	if token == "" {
		return 0, fmt.Errorf("invalid %s value '%s'", fieldName, token)
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid %s value '%s': expected integer between %d and %d", fieldName, token, minVal, maxVal)
		}
	}
	v, err := strconv.Atoi(token)
	if err != nil || v < minVal || v > maxVal {
		return 0, fmt.Errorf("out-of-range %s value '%s': expected between %d and %d", fieldName, token, minVal, maxVal)
	}
	return v, nil
}
