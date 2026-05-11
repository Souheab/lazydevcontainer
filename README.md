# lazydc

lazydc is a read-only terminal UI for viewing Docker containers with devcontainers highlighted first.

The v0 goal is intentionally small: list the containers visible to the configured Docker daemon, identify likely Dev Containers, show their workspace or mount path, and provide comfortable navigation, filtering, and search.

## Features

- Lists all Docker containers, including stopped containers.
- Marks devcontainers with a `DEV` badge and regular Docker containers with a `DOCKER` badge.
- Sorts devcontainers first by default.
- Shows the best-known devcontainer workspace/mount path when available.
- Filters between all containers, devcontainers only, and ordinary containers only.
- Searches across container name, image, status, ID, labels, and mount paths.
- Supports keyboard, Vim-style movement, and mouse input.
- Refreshes without leaving the TUI.

## Requirements

- Go 1.23 or newer.
- Docker Engine reachable from the environment where lazydc runs.

lazydc uses Docker environment configuration such as `DOCKER_HOST`. If running inside a devcontainer, the host Docker socket must already be mounted and accessible; this project does not change the devcontainer configuration for socket access.

## Build

```sh
go build -o lazydc ./cmd/lazydc
```

Or use Make:

```sh
make build
```

## Run

```sh
./lazydc
```

During development:

```sh
make run
```

## Keybindings

| Key | Action |
| --- | --- |
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `pgup` / `b` | Page up |
| `pgdn` | Page down |
| `g` / `home` | Jump to top |
| `G` / `end` | Jump to bottom |
| `/` | Open search modal |
| `f` | Open filter modal |
| `enter` | Confirm modal selection |
| `esc` | Close modal or clear an active search |
| `a` | Show all containers |
| `d` | Show only devcontainers |
| `o` | Show only ordinary Docker containers |
| `tab` / `l` / `→` | Next filter |
| `shift+tab` / `h` / `←` | Previous filter |
| `r` | Refresh containers |
| `?` | Toggle full help |
| `q` / `ctrl+c` | Quit |

Mouse wheel scrolling and click-to-select are enabled.

## Devcontainer detection

lazydc treats these signals as devcontainer indicators:

1. Dev Containers labels such as `devcontainer.local_folder`, `devcontainer.config_file`, `devcontainer.metadata`, and legacy VS Code labels such as `vsch.local.folder`.
2. Mounts that strongly suggest a devcontainer workspace, such as `/workspaces/...` targets or paths involving `.devcontainer`.

Path display prefers explicit label paths, then workspace bind mount sources, then paths derived from `.devcontainer` config files.

## Development

```sh
make fmt
make tidy
make test
make vet
make build
```

## Scope

v0 is view-only. It does not start, stop, exec into, attach to, or remove containers.
