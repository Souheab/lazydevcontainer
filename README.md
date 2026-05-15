# lazydc

lazydc is a terminal UI control panel for devcontainers.

## Features

- Lists all Docker containers, including stopped containers.
- Marks devcontainers with a `DEV` badge and regular Docker containers with a `DOCKER` badge.
- Sorts devcontainers first by default.
- Shows the best-known devcontainer workspace/mount path when available.
- Filters between all containers, devcontainers only, and ordinary containers only.
- Searches across container name, image, status, ID, labels, and mount paths.
- Starts, stops, and restarts selected containers after confirmation.
- Opens an interactive shell in the selected container.
- Opens a detected devcontainer workspace path in `$EDITOR`.
- Browses built-in devcontainer templates for common languages and stacks.
- Previews and writes `.devcontainer/devcontainer.json` templates into the current directory.
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
| `y` / `n` | Answer confirmation prompts |
| `enter` | Confirm modal selection, or default "no" on `[y/N]` prompts |
| `esc` | Close modal or clear an active search |
| `t` | Open templates tab |
| `c` | Open containers tab |
| `a` | Show all containers |
| `d` | Show only devcontainers |
| `o` | Show only ordinary Docker containers |
| `tab` / `l` / `→` | Next filter |
| `shift+tab` / `h` / `←` | Previous filter |
| `r` | Refresh containers |
| `s` | Start stopped selected container or stop running selected container |
| `R` | Restart selected container |
| `x` | Open an interactive shell in the selected container |
| `e` | Open the selected devcontainer workspace path in `$EDITOR` |
| `?` | Toggle full help |
| `q` / `ctrl+c` | Quit |

Mouse wheel scrolling and click-to-select are enabled.

## Devcontainer templates

Press `t` to open the Templates tab. The left pane lists built-in single-file templates and the right pane previews the `devcontainer.json` that will be written. Press `/` in the Templates tab to search by language, stack, tag, or description.

Press `enter` on a template to create `.devcontainer/devcontainer.json` in the directory where `lazydc` was launched. If the file already exists, lazydc shows an overwrite confirmation before replacing it. Version 1 templates only write `devcontainer.json`; they do not create Dockerfiles, Compose files, lockfiles, or app helper files.

Built-in templates:

- Base Ubuntu
- Go
- Node/TypeScript
- Python
- Rust
- Java
- .NET
- PHP
- Ruby
- C/C++
- Nix/Base

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

## Pending fixes
 - On features modal j/k for input means can't input those charcters in search

## Future development roadmap

Features being considered for future implementation:

- Custom user-defined templates
- Template repository instead of hardcoded custom templates
- Better modal UI/UX
- Devcontainer.json editor (Add features/configurations)
- All actions modal menu
