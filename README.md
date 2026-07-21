# Ship 🚀

A CLI that runs agents sequentially.

Confirm a queue from ship-labeled GitHub issues, then spin agents through an Implement → Review loop for each of the tickets and open a pull request.

**Status:** v0.1.0.

## Install

#### Linux/MacOS

```bash
curl -fsSL https://raw.githubusercontent.com/maxBRT/ship-cli/main/install.sh | bash
```

#### Windows

[Releases](https://github.com/maxBRT/ship-cli/releases) page for manual download.


## Prerequisites

- Authenticated [`gh`](https://cli.github.com/)
- Any of `cursor`, `codex`, `claude caude`, `pi` CLI installed

## Quickstart

```bash
cd /path/to/your-project
gh issue edit <n> --add-label "ship"
ship
```

Run `ship --help` for flags and commands.

## Install from source

```bash
go install github.com/maxBRT/ship-cli/cmd/ship@latest
```

Or clone and build locally:

```bash
git clone git@github.com:maxBRT/ship-cli.git
cd ship-cli
go build -o ~/.local/bin/ship ./cmd/ship
```
