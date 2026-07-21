[![CI](https://github.com/maxBRT/ship-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/maxBRT/ship-cli/actions/workflows/ci.yml)

# Ship 🚀

Ship turns labeled GitHub issues into a pull request by driving **your** local coding agent through a fixed sequence of phases.

It is not an agent itself. You bring Cursor Agent, Claude Code, Codex, or pi. Ship queues the tickets, starts a fresh agent for each phase, and opens the PR in whatever checkout you already have.

**Status:** v0.1.0.

## What “agents in a loop” means

One **Run** of `ship` processes a confirmed queue of tickets **one after another**, and for each ticket it starts **two fresh agent sessions** (Implement, then Review). When the queue is done, Ship starts **one more** agent for the Final phase which reviews the branch and opens a PR.

```
Run (`ship`)
├── pick ordered tickets labeled `ship`
├── for each ticket (one Iteration):
│   ├── Implement  → fresh agent: write the work, must commit
│   └── Review     → fresh agent: one pass over those commits
└── Final          → fresh agent: branch review, green bar, open PR
```

Each phase is a **new** agent process with a built-in prompt. Implement commits the ticket; Review may fix or leave the commits alone; Final must open a pull request.

It is recommended to run ship in a seperate worktree. I personnaly use [treehouse](https://github.com/kunchenguid/treehouse) to do manage them.

## Prerequisites

- Authenticated [`gh`](https://cli.github.com/)
- One of: Cursor Agent (`agent`), `pi`, `codex`, or `claude` on your `PATH`
- A git checkout of the GitHub repo whose issues you want to ship

## Install

#### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/maxBRT/ship-cli/main/install.sh | bash
```

The script detects linux/darwin × amd64/arm64, downloads the latest GitHub Release, verifies checksums, and installs `ship` to `~/.local/bin` (or a writable system bin).

#### Windows

Download a binary from [Releases](https://github.com/maxBRT/ship-cli/releases).

## Update

```bash
ship update
```

Downloads the latest GitHub Release, verifies checksums, and replaces the running binary in place. Check what you have with `ship --version`.

## Quickstart

```bash
cd /path/to/your-project
gh issue edit <n> --add-label "ship"
ship
```

On first run, Ship creates `.ship/config.yaml` (or run `ship init`). Pick tickets in the picker, then leave the agent phases running. When Final succeeds, Ship prints the PR URL.

## Configuration

Settings live in `.ship/config.yaml`. Create one with `ship init`, or let the first `ship` run write defaults.

Precedence: YAML → flags.

```yaml
branch: ""            # empty → generate ship/<id>
agent: cursor         # cursor | pi | codex | claude
model: ""             # optional model for the Agent
max_iterations: 10    # max tickets (Implement+Review) before Final
timeout: 20m          # per-Phase timeout (Go duration)
```

One-off overrides:

```bash
ship --agent pi --model gpt-5 --max-iterations 3 --branch ship/my-feature --timeout 30m
```
Useful flags:

```bash
ship --agent cursor          # cursor | pi | codex | claude (default: cursor)
ship --model <name>          # optional model for that agent
ship --max-iterations 5      # cap tickets this run before Final
ship --branch ship/my-feature
```

Run `ship --help` for all flags, env vars, and commands (`init`, `update`, `--version`).

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
