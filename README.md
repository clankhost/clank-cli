# Clank CLI

Command-line interface for [Clank](https://clank.host), a self-hosted platform for deploying containerized applications.

## Install

**macOS / Linux:**

```sh
curl -fsSL https://clank.host/install-cli.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://clank.host/install-cli.ps1 | iex
```

**Go:**

```sh
go install github.com/clankhost/clank-cli@latest
```

## Quick Start

```sh
# Authenticate
clank login

# List projects
clank projects list

# Deploy a service
clank deploy <service-id>

# Stream logs
clank logs <service-id>
```

## Commands

| Command | Description |
|---------|-------------|
| `clank login` | Authenticate via browser |
| `clank projects list/create/delete` | Manage projects |
| `clank services list/info/create/delete` | Manage services |
| `clank deploy <service-id>` | Deploy with live build logs |
| `clank rollback <service-id>` | Rollback to previous deployment |
| `clank logs <service-id>` | Stream runtime logs |
| `clank env list/set/delete/reveal` | Manage environment variables |
| `clank restart/stop/start` | Service lifecycle control |
| `clank domains list/add/remove/recheck` | Custom domain management |
| `clank deployments list/info/events` | Deployment history |
| `clank backups list/create/delete` | Backup management |
| `clank endpoints <service-id>` | View service endpoints |
| `clank servers list/add/remove` | Manage agent servers |
| `clank team list/create/switch/members` | Team management |
| `clank open <service-id>` | Open service URL in browser |
| `clank whoami` | Current user info |
| `clank config get/set` | CLI configuration |
| `clank init` | Initialize a new project from current directory |
| `clank update` | Self-update the CLI |

## Shell Completion

```sh
# Bash
clank completion bash > /etc/bash_completion.d/clank

# Zsh
clank completion zsh > "${fpath[1]}/_clank"

# Fish
clank completion fish > ~/.config/fish/completions/clank.fish

# PowerShell
clank completion powershell | Out-String | Invoke-Expression
```

## Build from Source

```sh
git clone https://github.com/clankhost/clank-cli.git
cd clank-cli
go build -o clank .
```

## Documentation

Full documentation is available at [docs.clank.host](https://docs.clank.host).

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
