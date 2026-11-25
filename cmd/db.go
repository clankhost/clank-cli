package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

type dbEngine string

const (
	dbEnginePostgres dbEngine = "postgres"
	dbEngineMySQL    dbEngine = "mysql"
	dbEngineMongo    dbEngine = "mongo"
	dbEngineRedis    dbEngine = "redis"
)

// dbShellInput allows tests to supply a custom reader for REPL mode.
var dbShellInput io.Reader

type dbExecResult struct {
	Engine    string `json:"engine"`
	ServiceID string `json:"service_id"`
	*api.ExecResponse
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database developer tools",
}

var dbShellCmd = &cobra.Command{
	Use:   "shell <service>",
	Short: "Open a database shell or run a one-shot query inside a database service",
	Long: `Auto-detects the database engine from the service's container image
(postgres, mysql/mariadb, mongo, redis) and executes queries inside the
running container using its configured credentials.

Use -c to run a single query non-interactively, or omit -c to open an
interactive REPL session. Use --engine to override engine auto-detection.

Examples:
  clank db shell my-postgres -c "SELECT version();"
  clank db shell my-postgres -c "SELECT count(*) FROM users;" -o json
  clank db shell my-redis -c "PING"
  clank db shell my-mysql --engine mysql -c "SHOW DATABASES;"
  clank db shell my-postgres`,
	Args: cobra.ExactArgs(1),
	RunE: runDBShell,
}

// detectDBEngine infers the database engine from a container image reference.
func detectDBEngine(image string) (dbEngine, error) {
	img := strings.TrimSpace(strings.ToLower(image))
	if img == "" {
		return "", fmt.Errorf("service has no container image configured (use --engine to specify one of: postgres, mysql, mongo, redis)")
	}

	// Strip digest (@sha256:...) and tag (:tag) without stripping registry port.
	if idx := strings.Index(img, "@"); idx != -1 {
		img = img[:idx]
	}
	lastSlash := strings.LastIndex(img, "/")
	if colon := strings.LastIndex(img, ":"); colon > lastSlash {
		img = img[:colon]
	}

	repo := img
	if lastSlash != -1 {
		repo = img[lastSlash+1:]
	}

	switch {
	case strings.Contains(repo, "postgres") ||
		strings.Contains(repo, "postgis") ||
		strings.Contains(repo, "timescaledb") ||
		strings.Contains(repo, "pgvector"):
		return dbEnginePostgres, nil
	case strings.Contains(repo, "mysql") || strings.Contains(repo, "mariadb"):
		return dbEngineMySQL, nil
	case strings.Contains(repo, "mongo"):
		return dbEngineMongo, nil
	case strings.Contains(repo, "redis") || strings.Contains(repo, "valkey"):
		return dbEngineRedis, nil
	default:
		return "", fmt.Errorf("unsupported database image %q (supported: postgres, mysql/mariadb, mongo, redis; use --engine to override)", image)
	}
}

// normalizeDBEngine validates and normalizes an explicit --engine flag value.
func normalizeDBEngine(raw string) (dbEngine, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "postgres", "postgresql", "psql", "pg":
		return dbEnginePostgres, nil
	case "mysql", "mariadb":
		return dbEngineMySQL, nil
	case "mongo", "mongodb", "mongosh":
		return dbEngineMongo, nil
	case "redis", "valkey", "redis-cli":
		return dbEngineRedis, nil
	default:
		return "", fmt.Errorf("unsupported --engine %q (supported: postgres, mysql, mongo, redis)", raw)
	}
}

// buildDBQueryCommand constructs a safe container exec argv slice for executing
// a non-interactive query. The query is ALWAYS passed as a positional argument
// ($1 after "--") and never interpolated into the shell script.
func buildDBQueryCommand(engine dbEngine, query string) ([]string, error) {
	var script string
	switch engine {
	case dbEnginePostgres:
		script = `exec psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postgres}" -c "$1"`
	case dbEngineMySQL:
		script = `bin=mysql; command -v mysql >/dev/null 2>&1 || bin=mariadb; ` +
			`if [ -n "${MYSQL_ROOT_PASSWORD:-}" ]; then MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec "$bin" -u root ${MYSQL_DATABASE:+"$MYSQL_DATABASE"} -e "$1"; ` +
			`elif [ -n "${MARIADB_ROOT_PASSWORD:-}" ]; then MYSQL_PWD="${MARIADB_ROOT_PASSWORD}" exec "$bin" -u root ${MARIADB_DATABASE:+"$MARIADB_DATABASE"} -e "$1"; ` +
			`else exec "$bin" -u root ${MYSQL_DATABASE:+"$MYSQL_DATABASE"} -e "$1"; fi`
	case dbEngineMongo:
		script = `bin=mongosh; command -v mongosh >/dev/null 2>&1 || bin=mongo; ` +
			`if [ -n "${MONGO_INITDB_ROOT_USERNAME:-}" ] && [ -n "${MONGO_INITDB_ROOT_PASSWORD:-}" ]; then ` +
			`exec "$bin" --quiet -u "${MONGO_INITDB_ROOT_USERNAME}" -p "${MONGO_INITDB_ROOT_PASSWORD}" --authenticationDatabase admin --eval "$1"; ` +
			`else exec "$bin" --quiet --eval "$1"; fi`
	case dbEngineRedis:
		script = `if [ -n "${REDIS_PASSWORD:-}" ]; then export REDISCLI_AUTH="${REDIS_PASSWORD}"; fi; ` +
			`printf '%s\n' "$1" | exec redis-cli --no-auth-warning`
	default:
		return nil, fmt.Errorf("unsupported database engine %q", engine)
	}

	return []string{"sh", "-c", script, "--", query}, nil
}

func runDBShell(cmd *cobra.Command, args []string) error {
	timeout, _ := cmd.Flags().GetInt("timeout")
	if timeout < 1 || timeout > 120 {
		return fmt.Errorf("--timeout must be between 1 and 120 seconds")
	}

	engineFlag, _ := cmd.Flags().GetString("engine")
	query, _ := cmd.Flags().GetString("command")
	oneShot := cmd.Flags().Changed("command") || query != ""

	serviceRef := args[0]
	client := newClient()
	serviceID, err := resolveServiceID(client, serviceRef)
	if err != nil {
		return err
	}

	var engine dbEngine
	if strings.TrimSpace(engineFlag) != "" {
		engine, err = normalizeDBEngine(engineFlag)
		if err != nil {
			return err
		}
	} else {
		svc, err := api.GetService(client, serviceID)
		if err != nil {
			return err
		}
		image := ""
		if svc.Image != nil {
			image = *svc.Image
		}
		engine, err = detectDBEngine(image)
		if err != nil {
			return err
		}
	}

	// One-shot query mode (-c).
	if oneShot {
		if strings.TrimSpace(query) == "" {
			return fmt.Errorf("query cannot be empty")
		}
		argv, err := buildDBQueryCommand(engine, query)
		if err != nil {
			return err
		}
		resp, err := api.ExecService(client, serviceID, api.ExecRequest{
			Command:        argv,
			TimeoutSeconds: timeout,
		})
		if err != nil {
			return err
		}

		if output.IsJSON() {
			if err := output.JSON(dbExecResult{
				Engine:       string(engine),
				ServiceID:    serviceID,
				ExecResponse: resp,
			}); err != nil {
				return err
			}
		} else {
			if resp.Stdout != "" {
				fmt.Fprint(os.Stdout, resp.Stdout)
			}
			if resp.Stderr != "" {
				fmt.Fprint(os.Stderr, resp.Stderr)
			}
			if resp.Truncated {
				fmt.Fprintln(os.Stderr, "\n[output truncated]")
			}
		}

		if resp.ExitCode != 0 {
			return fmt.Errorf("query exited with status %d", resp.ExitCode)
		}
		return nil
	}

	// Interactive REPL mode (no -c).
	in := dbShellInput
	if in == nil {
		in = os.Stdin
	}

	fmt.Fprintf(os.Stdout, "Connected to %s (%s). Type '\\q' or 'exit' to quit.\n", serviceRef, engine)
	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprintf(os.Stdout, "%s(%s)> ", engine, serviceRef)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		switch strings.ToLower(line) {
		case "\\q", "exit", "quit":
			return nil
		}

		argv, err := buildDBQueryCommand(engine, line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}

		resp, err := api.ExecService(client, serviceID, api.ExecRequest{
			Command:        argv,
			TimeoutSeconds: timeout,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}

		if output.IsJSON() {
			if err := output.JSON(dbExecResult{
				Engine:       string(engine),
				ServiceID:    serviceID,
				ExecResponse: resp,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			continue
		}

		if resp.Stdout != "" {
			fmt.Fprint(os.Stdout, resp.Stdout)
			if !strings.HasSuffix(resp.Stdout, "\n") {
				fmt.Fprintln(os.Stdout)
			}
		}
		if resp.Stderr != "" {
			fmt.Fprint(os.Stderr, resp.Stderr)
			if !strings.HasSuffix(resp.Stderr, "\n") {
				fmt.Fprintln(os.Stderr)
			}
		}
		if resp.Truncated {
			fmt.Fprintln(os.Stderr, "[output truncated]")
		}
		if resp.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "[exit %d]\n", resp.ExitCode)
		}
	}
}

func init() {
	dbShellCmd.Flags().StringP("command", "c", "", "execute a single query non-interactively")
	dbShellCmd.Flags().String("engine", "", "override database engine (postgres, mysql, mongo, redis)")
	dbShellCmd.Flags().Int("timeout", 30, "query timeout in seconds (1-120)")

	dbCmd.AddCommand(dbShellCmd)
	rootCmd.AddCommand(dbCmd)
}
