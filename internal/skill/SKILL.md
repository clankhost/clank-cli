---
name: clank
description: Manage Clank platform — projects, services, deploys, env vars, resources, endpoints, domains, backups, servers, and more. All operations use the CLI directly.
---

# /clank

Manage the Clank PaaS platform from Claude Code. All operations use the `clank` CLI.

## Usage

```
/clank <natural language request>
```

Examples:
- `/clank list my projects`
- `/clank deploy the ghost service`
- `/clank set DATABASE_URL on service abc123`
- `/clank restart the wordpress service`
- `/clank show servers`

## Before Any Operation

1. **Verify auth**: Run `clank whoami` to confirm authentication
2. If unauthorized, run `clank login` to authenticate via device code flow

## CLI Commands Reference

Use these `clank` commands directly. Add `-o json` when chaining output into another step.

### Projects
```bash
clank projects list                          # List all projects
clank projects create "My Project"           # Create project
clank projects delete <project-id>           # Delete project
```

### Services
```bash
clank services list --project <project-id>   # List services in project
clank services info <service-id>             # Service details
clank services create --project <id> --name "web" --repo "user/repo" --port 3000
clank services delete <service-id>
```

### Deploy & Rollback
```bash
clank deploy <service-id>                    # Deploy latest (streams logs)
clank deploy <service-id> --no-follow        # Deploy without log streaming
clank rollback <service-id>                  # Rollback to previous deployment
clank rollback <service-id> --to <deploy-id> # Rollback to specific deployment
```

### Logs
```bash
clank logs <service-id>                      # Stream runtime logs
clank logs <service-id> --tail 100           # Last 100 lines
clank logs <service-id> --build <deploy-id>  # Build logs for a deployment
```

### Environment Variables
```bash
clank env list <service-id>                  # List env vars (secrets masked)
clank env set <service-id> KEY=VALUE         # Set a variable
clank env set <service-id> KEY=VALUE --secret # Set as secret
clank env set <service-id> K1=V1 K2=V2       # Set multiple at once
clank env set <service-id> -f .env           # Load from .env file
clank env reveal <service-id> KEY            # Show secret value
clank env delete <service-id> KEY            # Delete a variable
```

### Service Control
```bash
clank restart <service-id>                   # Restart the container
clank stop <service-id>                      # Stop the container
clank start <service-id>                     # Start a stopped container
```

### Custom Domains
```bash
clank domains list <service-id>              # List domains
clank domains add <service-id> example.com   # Add a domain
clank domains add <service-id> example.com --primary  # Add as primary
clank domains remove <domain-id>             # Remove a domain
clank domains recheck <domain-id>            # Re-check DNS verification
```

### Deployment History
```bash
clank deployments list <service-id>          # List all deployments (alias: deps)
clank deployments info <deployment-id>       # Deployment details
clank deployments events <deployment-id>     # Lifecycle events
```

### Backups
```bash
clank backups list <service-id>              # List backups
clank backups create <service-id>            # Create a backup
clank backups delete <service-id> <backup-id> # Delete a backup
```

### Resources (CPU & Memory)
```bash
clank resources <service-id>                 # View CPU/memory limits + server capacity
clank resources set <service-id> --cpu 1     # Set CPU limit (vCPU)
clank resources set <service-id> --memory 2048  # Set memory limit (MB)
clank resources set <service-id> --cpu 0.5 --memory 1024  # Set both
```

### Endpoints
```bash
clank endpoints <service-id>                 # List endpoints (alias: clank ep)
```

### Servers (Agent Hosts)
```bash
clank servers list                           # List enrolled servers
clank servers add "my-server"                # Register server, get install command
clank servers remove <server-id>             # Decommission server
```

### Teams
```bash
clank team list                              # List teams
clank team create "Team Name"                # Create team
clank team switch <name-or-id>               # Switch active team
clank team current                           # Show active team
clank team members                           # List members
clank team invite user@example.com           # Invite member
clank team invite user@example.com --role admin  # Invite with role
clank team role user@example.com developer   # Change role
clank team remove user@example.com           # Remove member
```

### Other
```bash
clank whoami                                 # Current user info
clank open <service-id>                      # Open service URL in browser
clank version                                # CLI version
clank update                                 # Self-update CLI
clank config get <key>                       # Get config value (base_url, token, team_id)
clank config set <key> <value>               # Set config value
clank skill install                          # Install /clank Claude Code skill
```

## API Calls (Advanced)

For operations the CLI doesn't cover yet, use `curl` with auth from the CLI config.

### Auth Header Pattern

```bash
BASE=$(clank config get base_url)
TOKEN=$(clank config get token)
TEAM=$(clank config get team_id)

curl -s -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" "$BASE/api/..."
```

If TOKEN starts with `clank_` (API key), use `Authorization: Bearer $TOKEN` instead of Cookie.

### Push Local Image to Registry
```bash
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/deployments/{deployment_id}/push-to-registry"
```

## Compound Workflows

For multi-step operations, chain CLI commands:

### Set Up New Service
1. `clank services create --project <id> --name "web" --repo "user/repo" --port 3000 -o json` → get service_id
2. `clank env set <service-id> DB_URL=postgres://... API_KEY=sk-123 --secret`
3. `clank deploy <service-id>` → deploy and stream logs

### Clone Env Vars Between Services
1. `clank env list <source-service-id> -o json` → get key/value pairs
2. For each secret: `clank env reveal <source-service-id> KEY`
3. `clank env set <target-service-id> KEY1=VAL1 KEY2=VAL2`

### Deploy and Monitor
1. `clank deploy <service-id> --no-follow -o json` → get deployment_id
2. `clank deployments info <deployment-id>` → check status
3. If failed: `clank deployments events <deployment-id>` → see what went wrong

## Critical Rules

- **Always verify auth first** with `clank whoami` before running commands
- **Use `-o json`** when piping CLI output into another command or parsing it
- **Use table format** (default) when showing results to the user
- **Never expose secret env var values** in output unless the user explicitly asks to reveal them
- **Service IDs are UUIDs** — when the user refers to a service by name, first list services to find the ID
- **Agent servers** connect outbound via gRPC — if a server shows offline, the agent may have lost connection
