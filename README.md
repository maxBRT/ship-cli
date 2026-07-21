# ship

CLI that runs a sequential Ticket Run in your existing git checkout: confirm a ship queue from ship-labeled GitHub issues, drive Cursor’s `agent` through Implement → Review → Final, and open a pull request.

**Status:** v0.0.1 (experimental).

## User guide

Open **[guide.html](guide.html)** in a browser for install, update, how a Run works, examples, flags, and troubleshooting.

```bash
xdg-open guide.html   # or open guide.html from a file manager
```

## Quick install (today)

```bash
git clone git@github.com:maxBRT/ship-cli.git
cd ship-cli
go build -o ~/.local/bin/ship ./cmd/ship
ship --help
```

Or: `go install github.com/maxBRT/ship-cli/cmd/ship@latest`

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

See [guide.html](guide.html) for flags, env vars, and more examples. Domain language: [CONTEXT.md](CONTEXT.md).
