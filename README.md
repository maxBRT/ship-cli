# ship

CLI that runs a sequential Ticket Run in your existing git checkout: confirm a ship queue from ship-labeled GitHub issues, drive Cursor’s `agent` through Implement → Review → Final, and open a pull request.

**Status:** v0.0.1 (experimental).

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/maxBRT/ship-cli/main/install.sh | bash
```

The script detects linux/darwin × amd64/arm64, downloads the latest GitHub Release, verifies checksums, and installs `ship` to `~/.local/bin` (or a writable system bin). Windows release assets are on the same [Releases](https://github.com/maxBRT/ship-cli/releases) page for manual download.

## Update

```bash
ship update
```

Downloads the latest GitHub Release, verifies checksums, and replaces the running binary in place. Check what you have with `ship --version`.

## Prerequisites

- Git checkout (Ship does not create worktrees)
- Authenticated [`gh`](https://cli.github.com/)
- [Cursor Agent CLI](https://cursor.com/docs/cli/installation) (`agent` on `PATH`)

## Quickstart

```bash
cd /path/to/your-project
gh issue edit <n> --add-label "ship"
ship
```

Domain language: [CONTEXT.md](CONTEXT.md). Run `ship --help` for flags and commands.

## Install from source (Go developers)

```bash
go install github.com/maxBRT/ship-cli/cmd/ship@latest
```

Or clone and build locally:

```bash
git clone git@github.com:maxBRT/ship-cli.git
cd ship-cli
go build -o ~/.local/bin/ship ./cmd/ship
```
