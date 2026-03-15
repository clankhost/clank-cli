---
name: clank
description: Manage Clank platform — projects, services, deploys, env vars, endpoints, servers, and more. Uses the CLI for supported operations and API calls for advanced features.
---

# /clank

Manage the Clank PaaS platform from Claude Code. Execute operations through the `clank` CLI and direct API calls.

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
3. For API calls, extract credentials:
   ```bash
   BASE=$(clank config get base_url)
   TOKEN=$(clank config get token)
   TEAM=$(clank config get team_id)
   ```

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
```

## API Calls (Features Not Yet in CLI)

For operations the CLI doesn't support, use `curl` with auth from the CLI config.

### Auth Header Pattern

```bash
BASE=$(clank config get base_url)
TOKEN=$(clank config get token)
TEAM=$(clank config get team_id)

# GET request
curl -s -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" "$BASE/api/..."

# POST with JSON body
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" -H "Content-Type: application/json" \
  -d '{"key": "value"}' "$BASE/api/..."
```

If TOKEN starts with `clank_` (API key), use `Authorization: Bearer $TOKEN` instead of Cookie.

### Environment Variables

```bash
# List env vars for a service
curl -s -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/{service_id}/env-vars"

# Set a single env var
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" -H "Content-Type: application/json" \
  -d '{"key": "DATABASE_URL", "value": "postgres://...", "is_secret": true}' \
  "$BASE/api/services/{service_id}/env-vars"

# Bulk set env vars
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" -H "Content-Type: application/json" \
  -d '{"vars": [{"key": "K1", "value": "V1"}, {"key": "K2", "value": "V2", "is_secret": true}]}' \
  "$BASE/api/services/{service_id}/env-vars/bulk"

# Reveal a secret env var value
curl -s -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/env-vars/{var_id}/reveal"

# Delete an env var
curl -s -X DELETE -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/env-vars/{var_id}"
```

### Service Control (Restart / Stop / Start)

```bash
# Restart a service
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/{service_id}/restart"

# Stop a service
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/{service_id}/stop"

# Start a service
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/{service_id}/start"
```

### Domains

```bash
# List domains for a service
curl -s -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/{service_id}/domains"

# Add a domain
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" -H "Content-Type: application/json" \
  -d '{"domain": "app.example.com", "is_primary": true}' \
  "$BASE/api/services/{service_id}/domains"

# Recheck domain DNS
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/domains/{domain_id}/recheck"

# Delete a domain
curl -s -X DELETE -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/domains/{domain_id}"
```

### Deployment History

```bash
# List deployments for a service
curl -s -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/{service_id}/deployments"

# Get deployment details
curl -s -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/deployments/{deployment_id}"

# Get deployment events
curl -s -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/deployments/{deployment_id}/events"

# Push local image to registry
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/deployments/{deployment_id}/push-to-registry"
```

### Backups

```bash
# Create a backup
curl -s -X POST -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/{service_id}/backups"

# List backups
curl -s -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/{service_id}/backups"

# Delete a backup
curl -s -X DELETE -H "Cookie: clank_session=$TOKEN" -H "X-Team-Id: $TEAM" \
  -H "X-Requested-With: XMLHttpRequest" \
  "$BASE/api/services/{service_id}/backups/{backup_id}"
```

## Compound Workflows

For multi-step operations, chain CLI + API calls:

### Set Up New Service
1. `clank services create --project <id> --name "web" --repo "user/repo" --port 3000 -o json` → get service_id
2. Set env vars via bulk API call
3. `clank deploy <service-id>` → deploy and stream logs

### Clone Env Vars Between Services
1. List source env vars via API → get key/value pairs
2. Reveal secrets via API → get plaintext values
3. Bulk set on target service via API

### Deploy and Monitor
1. `clank deploy <service-id> --no-follow` → get deployment_id
2. Poll deployment status via API until terminal state
3. If failed, fetch deployment events for error details

## Critical Rules

- **Always verify auth first** with `clank whoami` before running commands
- **Use `-o json`** when piping CLI output into another command or parsing it
- **Use table format** (default) when showing results to the user
- **Never expose secret env var values** in output unless the user explicitly asks to reveal them
- **Include `X-Requested-With: XMLHttpRequest`** on all API calls — the API requires it
- **Include `X-Team-Id`** header on all API calls — without it, requests fail
- **Service IDs are UUIDs** — when the user refers to a service by name, first list services to find the ID
- **Agent servers** connect outbound via gRPC — if a server shows offline, the agent may have lost connection
