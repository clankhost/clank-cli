# Clank CLI

Command-line interface for deploying and managing apps on [Clank](https://github.com/anaremore/clank).

## Install

### From Release

Download the latest binary from [GitHub Releases](https://github.com/anaremore/clank/releases) and add it to your PATH.

**macOS/Linux:**
```bash
# macOS Apple Silicon
curl -L https://github.com/anaremore/clank/releases/latest/download/clank_darwin_arm64.tar.gz | tar xz
sudo mv clank /usr/local/bin/

# macOS Intel
curl -L https://github.com/anaremore/clank/releases/latest/download/clank_darwin_amd64.tar.gz | tar xz
sudo mv clank /usr/local/bin/

# Linux
curl -L https://github.com/anaremore/clank/releases/latest/download/clank_linux_amd64.tar.gz | tar xz
sudo mv clank /usr/local/bin/
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri https://github.com/anaremore/clank/releases/latest/download/clank_windows_amd64.zip -OutFile clank.zip
Expand-Archive clank.zip -DestinationPath .
# Move clank.exe to a directory in your PATH
```

### Build from Source

Requires Go 1.23+.

```bash
cd apps/cli
go mod tidy
make build
./clank version
```

## Quick Start

```bash
# 1. Point to your Clank instance
clank config set base_url https://your-clank-instance.com

# 2. Log in
clank login

# 3. List your projects
clank projects list

# 4. List services in a project
clank services list --project <project-id>

# 5. Deploy a service
clank deploy <service-id>

# 6. Stream logs
clank logs <service-id>
```

## Commands

### Authentication

```bash
clank login                          # Interactive login (email + password)
clank login --email admin@your-domain.com # Pre-fill email
```

The auth token is stored in `~/.config/clank/config.yaml` (0600 permissions).
On Windows: `%APPDATA%/clank/config.yaml`.

### Configuration

```bash
clank config get                     # Show all config values
clank config get base_url            # Show specific value
clank config set base_url <url>      # Set API base URL
```

### Projects

```bash
clank projects list                  # List all projects
clank projects create "My App"       # Create a project
clank projects delete <id>           # Delete a project
```

### Services

```bash
clank services list --project <id>   # List services
clank services info <service-id>     # Show service details
clank services create --project <id> --name myapp --repo https://github.com/user/repo
clank services delete <service-id>   # Delete a service
```

### Deploy

```bash
clank deploy <service-id>            # Deploy and stream build output
clank deploy <service-id> --no-follow  # Deploy without streaming
```

The deploy command streams build logs and deployment events in real time.
It exits 0 on success ("active") and 1 on failure.

### Rollback

```bash
clank rollback <service-id>                   # Rollback to previous deployment
clank rollback <service-id> --to <deploy-id>  # Rollback to specific deployment
```

### Logs

```bash
clank logs <service-id>              # Stream runtime logs (Ctrl+C to stop)
clank logs <service-id> --tail 500   # Tail more lines
clank logs <service-id> --build <deployment-id>  # Stream build logs
```

### Open in Browser

```bash
clank open <service-id>              # Opens the service URL in your browser
```

### Init (Project Setup)

```bash
clank init                           # Detect project type, generate Dockerfile + clank.yaml
clank init --name myapp --port 3000  # Override detected values
```

Supported project types:
- Node.js SPA (Vite, Next.js, CRA, Vue CLI, Nuxt)
- Node.js Server (Express, Fastify, Koa)
- Python (FastAPI, Flask, Django)
- Static HTML

## Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `session expired or invalid` | JWT expired (24h) | Run `clank login` again |
| `a deployment is already in progress` | Concurrent deploy | Wait for current deploy to finish |
| `no active deployment found` | No running container for logs | Deploy first with `clank deploy` |
| `no previous deployment found` | No deployment to rollback to | Use `--to` to specify a deployment |

## Development

```bash
cd apps/cli
go mod tidy
make test         # Run all tests
make build        # Build for current platform
make build-all    # Cross-compile all platforms
make release-snapshot  # Test GoReleaser locally
```
