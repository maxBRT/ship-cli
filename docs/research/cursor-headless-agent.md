# Cursor headless agent CLI — research for Ship adapter

**Researched:** 2026-07-18  
**Local CLI version:** `2026.07.16-899851b` (`agent --version`)  
**Binary path (this machine):** `/home/max/.local/bin/agent` (`which agent`)

This document records the **current, primary-source** invocation contract for Cursor’s headless/`--print` agent CLI so Ship can implement a Cursor agent adapter without guessing.

---

## TL;DR for Ship

Each Ship **Phase** (Implement, Review, Final) should spawn a **new** `agent` process in **print mode** with unattended flags, pointed at the run checkout:

```bash
cd "$CHECKOUT" && \
  agent -p \
    --trust \
    --force \
    --approve-mcps \
    --workspace "$CHECKOUT" \
    --model "$MODEL" \
    --output-format stream-json \
    --stream-partial-output \
    "$PROMPT"
```

- **Do not** pass `--resume`, `--continue`, or `agent resume` — Ship needs a fresh agent per phase.  
- **Do** use `--force` whenever the phase may edit files or run shell commands unattended.  
- **Do** use `--trust` and `--approve-mcps` to avoid interactive workspace/MCP prompts in headless mode.  
- **Success:** exit code `0`; with JSON formats, a terminal `result` event / object with `"subtype": "success"`.  
- **Failure:** non-zero exit; error on **stderr**; no well-formed terminal JSON.  
- **Timeouts:** no native agent-run timeout flag — Ship must enforce externally (e.g. `timeout(1)`, process kill, Go `context`).

---

## 1. CLI entrypoint, install, and auth

### Binary name

| Item | Value | Source |
|------|-------|--------|
| Command | `agent` | `agent --help` (local, 2026-07-18) |
| Default install path | `~/.local/bin/agent` | [Installation](https://cursor.com/docs/cli/installation) |
| Verify | `agent --version` | [Installation](https://cursor.com/docs/cli/installation) |
| System info | `agent about` (`--format json` supported) | `agent --help`, `agent about --format json` (local) |

### Install

```bash
# macOS, Linux, WSL
curl https://cursor.com/install -fsS | bash

# Windows (PowerShell)
irm 'https://cursor.com/install?win32=true' | iex
```

Source: [Installation](https://cursor.com/docs/cli/installation)

Post-install, add `~/.local/bin` to `PATH`. Updates: `agent update` (auto-update is default).

### Authentication

Two supported methods:

| Method | Usage | Source |
|--------|-------|--------|
| Browser login (interactive/dev) | `agent login` | [Authentication](https://cursor.com/docs/cli/reference/authentication) |
| API key (automation/CI) | `export CURSOR_API_KEY=...` or `agent --api-key ...` | [Authentication](https://cursor.com/docs/cli/reference/authentication), `agent --help` |
| Check status | `agent status` / `agent whoami` (`--format json`) | [Authentication](https://cursor.com/docs/cli/reference/authentication) |

Notes from docs:

- `NO_OPEN_BROWSER=1 agent login` prints the login URL without opening a browser.  
- API keys are generated from **Cursor Dashboard → API Keys**.  
- Only `CURSOR_API_KEY` is documented as an auth env var in official CLI docs (`agent --help` mentions it explicitly). No documented `CURSOR_MODEL` env var.

---

## 2. Headless / non-interactive mode (`-p` / `--print`)

### Primary flag

`-p` / `--print` — “Print responses to console (for scripts or non-interactive use). Has access to all tools, including write and shell.”  
Source: `agent --help`, [Parameters](https://cursor.com/docs/cli/reference/parameters), [Using Agent in CLI — Non-interactive mode](https://cursor.com/docs/cli/using)

### Print mode can be inferred

`--output-format` works when:

1. `--print` is passed explicitly, **or**
2. Print mode is **inferred** from **non-TTY stdout** or **piped stdin**.

Source: [Output format](https://cursor.com/docs/cli/reference/output-format)

Ship should pass `-p` explicitly for clarity in subprocess invocation.

### Related unattended flags

| Flag | Purpose | Headless? | Source |
|------|---------|-----------|--------|
| `-f` / `--force` | “Force allow commands unless explicitly denied”; also required for **file writes** in print scripts | Yes | `agent --help`, [Headless CLI](https://cursor.com/docs/cli/headless) |
| `--yolo` | Alias for `--force` | Yes | `agent --help` |
| `--trust` | “Trust the current workspace without prompting (**only works with --print/headless mode**)” | Yes | `agent --help`, [Parameters](https://cursor.com/docs/cli/reference/parameters) |
| `--approve-mcps` | “Automatically approve all MCP servers” | Yes | `agent --help`, [Parameters](https://cursor.com/docs/cli/reference/parameters) |
| `--sandbox enabled\|disabled` | Override sandbox mode for command execution | Yes | `agent --help`, [Parameters](https://cursor.com/docs/cli/reference/parameters) |
| `--auto-review` | “Smart Auto”: classifier auto-runs safe tool calls, prompts for the rest | Interactive bias | `agent --help` (local only; not in web params table) |

### File modifications in print mode

From [Headless CLI](https://cursor.com/docs/cli/headless):

- **`agent -p --force "..."`** — agent may apply file edits and run tools unattended.  
- **`agent -p "..."`** (no `--force`) — “changes are only proposed, not applied.”

Interactive mode asks `(y/n)` before shell commands ([Using Agent in CLI — Command approval](https://cursor.com/docs/cli/using)). In print mode, Cursor “has full write access” ([Using Agent in CLI — Non-interactive mode](https://cursor.com/docs/cli/using)), but **`--force` is still required for actual file mutations** per the headless doc.

---

## 3. Passing the prompt

### Documented mechanisms

| Mechanism | Syntax | Notes | Source |
|-----------|--------|-------|--------|
| Positional args | `agent -p "do the thing"` or `agent -p word1 word2 ...` | `prompt` is `[prompt...]` — multiple words become one prompt | `agent --help`, [Parameters — Arguments](https://cursor.com/docs/cli/reference/parameters) |
| Stdin (piped) | `echo "prompt" \| agent -p` | Works when **no** positional prompt is given | [Output format](https://cursor.com/docs/cli/reference/output-format) (print inferred from piped stdin); **verified locally** |
| File paths in prompt text | `agent -p "Read src/foo.go and ..."` | Agent reads files via tool calls; no dedicated `--prompt-file` flag | [Headless CLI — Working with images/files](https://cursor.com/docs/cli/headless) |
| Interactive `@` mentions | `@file` in TUI | Documented for interactive CLI, not headless-specific | [Using Agent in CLI — Selecting context](https://cursor.com/docs/cli/using) |

### Empirical checks (local, 2026-07-18)

```bash
# Stdin only → uses stdin
printf 'Reply with exactly: STDIN_OK' | agent -p --trust --output-format text
# → STDIN_OK, exit 0

# Positional + piped stdin → positional wins
printf 'Reply with exactly: STDIN_WINS' | agent -p --trust --output-format text "Reply with exactly: ARG_WINS"
# → ARG_WINS, exit 0
```

**Ship recommendation:** for long phase prompts, either:

- pipe/redirect stdin: `agent -p ... < "$PROMPT_FILE"`, or  
- pass expanded content: `agent -p ... "$(cat "$PROMPT_FILE")"` (watch shell arg length limits).

There is **no** documented `@promptfile` or `--prompt-file` CLI flag.

---

## 4. Model selection

| Mechanism | Example | Source |
|-----------|---------|--------|
| `--model <id>` | `--model gpt-5.3-codex-high` | `agent --help`, [Parameters](https://cursor.com/docs/cli/reference/parameters) |
| List models | `agent models` or `agent --list-models` | `agent --help` |
| Parameterized overrides | `'claude-opus-4-8[context=1m,effort=high,fast=false]'` | `agent --help` (local) |
| Default | `auto` | `agent models` (local output) |

Invalid model (local test):

```bash
agent -p --trust --output-format json --model invalid-model-xyz "say hi"
# stderr: Cannot use this model: invalid-model-xyz. Available models: ...
# exit 1, no JSON on stdout
```

No official env-var override for model besides passing `--model`.

---

## 5. Working directory and workspace

| Mechanism | Behavior | Source |
|-----------|----------|--------|
| Process `cwd` | Agent runs with the shell’s current working directory | [Output format — system init event `cwd`](https://cursor.com/docs/cli/reference/output-format), [Using — CLI worktrees](https://cursor.com/docs/cli/using) |
| `--workspace <path-or-name>` | “Workspace directory or saved workspace name to use (**defaults to current working directory**)” | `agent --help`, [Parameters](https://cursor.com/docs/cli/reference/parameters) |
| `--add-dir <path>` | Additional workspace roots (repeatable) | `agent --help` (local) |

From [Using — CLI worktrees](https://cursor.com/docs/cli/using):

> Combine `--workspace` when you need an explicit repository root. Otherwise the CLI uses the current working directory.

**Ship recommendation:** `cd "$CHECKOUT"` **and** pass `--workspace "$CHECKOUT"` so cwd and workspace stay aligned even if Ship’s process cwd differs.

**Not for Ship (per ADR):** `--worktree` creates an isolated git worktree under `~/.cursor/worktrees/...` — Ship owns the checkout; do not use `--worktree` unless that decision changes.

---

## 6. Fresh session vs resume

Ship requires a **fresh agent per Phase** ([CONTEXT.md](../../CONTEXT.md): “A single fresh agent invocation within a run”).

### Start fresh (default)

Invoke `agent -p ...` **without**:

- `--resume [chatId]`
- `--continue` (documented alias for `--resume=-1`)
- `agent resume`
- `agent ls` (interactive picker)

Each invocation returns a new `session_id` in JSON/stream output ([Output format](https://cursor.com/docs/cli/reference/output-format)).

### Resume (avoid for Ship phases)

| Mechanism | Effect | Source |
|-----------|--------|--------|
| `--resume [chatId]` | Load prior thread | [Parameters](https://cursor.com/docs/cli/reference/parameters), [Using — History](https://cursor.com/docs/cli/using) |
| `--continue` | Continue previous session (`--resume=-1`) | [Parameters](https://cursor.com/docs/cli/reference/parameters) |
| `agent resume` | Resume latest chat | `agent --help` |
| `agent create-chat` | Creates empty chat, prints chat ID | `agent create-chat` (local: returns UUID on stdout) |

`create-chat` + `--resume <id>` would continue a specific thread — opposite of “fresh per phase.” Ship should simply spawn a new `agent -p` subprocess per phase.

---

## 7. Output formats (`--output-format`)

Only valid with `--print` (or inferred print mode). Default: **`text`**.

Source: [Output format](https://cursor.com/docs/cli/reference/output-format), `agent --help`

| Format | stdout | Use case |
|--------|--------|----------|
| `text` | Final assistant message only | Simple logs |
| `json` | Single JSON object on success | Easy parse after completion |
| `stream-json` | NDJSON events + terminal `result` | Progress, tool telemetry |
| `--stream-partial-output` | With `stream-json` only — character deltas | Live streaming UI |

### Success JSON shape (`--output-format json`)

```json
{
  "type": "result",
  "subtype": "success",
  "is_error": false,
  "duration_ms": 1234,
  "duration_api_ms": 1234,
  "result": "<full assistant text>",
  "session_id": "<uuid>",
  "request_id": "<optional>"
}
```

Source: [Output format — JSON format](https://cursor.com/docs/cli/reference/output-format)

`stream-json` emits `system`/`user`/`assistant`/`tool_call`/`result` events; see the same doc for schemas.

**Notes:**

- `thinking` events are suppressed in print mode.  
- `duration_ms` is reported on success — useful for metrics, not a timeout cap.

---

## 8. Exit codes and success/failure signaling

### Documented contract

From [Output format](https://cursor.com/docs/cli/reference/output-format):

| Outcome | Exit code | stdout | stderr |
|---------|-----------|--------|--------|
| Success | **0** (implied by examples using `$? -eq 0`) | Well-formed output per format | — |
| Failure | **Non-zero** | For `json`: **no** well-formed JSON object; for `stream-json`: stream may end early **without** terminal `result` event | Error message |

[Headless CLI example](https://cursor.com/docs/cli/headless) checks `if [ $? -eq 0 ]` after `agent -p`.

### Observed locally (2026-07-18)

| Case | Exit code |
|------|-----------|
| Successful `-p` run | `0` |
| Invalid `--model` | `1` (error text on stderr) |

**Official docs do not enumerate specific non-zero exit codes** (e.g. 1 vs 2). Ship should treat **any non-zero** as phase failure.

### Distinguishing agent failure vs tool failure

The CLI does not document a separate exit code when the agent completes but reports task failure in natural language. With `--output-format json`, success responses have `"is_error": false`. Ship may additionally parse the final `result` text for domain-specific signals if needed — that is Ship logic, not CLI contract.

---

## 9. Timeouts

### Agent run (whole phase)

**No native timeout flag** exists on `agent` (`agent --help` has no `--timeout`; [Parameters](https://cursor.com/docs/cli/reference/parameters) lists none).

Ship must enforce phase deadlines externally:

- Unix: `timeout 3600 agent -p ...`  
- Go: `exec.CommandContext(ctx, "agent", ...)` and kill on cancel

On external timeout, the process receives a signal; expect non-zero exit (typically `124` from GNU `timeout` — that is `timeout(1)` behavior, not Cursor’s).

### Shell command timeout inside agent (not the same thing)

[Shell Mode](https://cursor.com/docs/cli/shell-mode) documents a **30-second** limit per shell command in **interactive Shell Mode**, and states it is **not configurable**. That applies to Shell Mode in the TUI, **not** documented as a global cap on headless agent tool/shell execution. Do not conflate with Ship phase timeout.

---

## 10. Blockers for unattended runs

| Blocker | Mitigation | Source |
|---------|------------|--------|
| Workspace trust prompt | `--trust` (headless only) | `agent --help` |
| MCP server approval | `--approve-mcps`, or pre-approve via `agent mcp enable <id>`, or disable with `agent mcp disable <id>` | `agent --help`, [Parameters — MCP](https://cursor.com/docs/cli/reference/parameters) |
| Shell command approval (interactive) | Use `-p`; add `--force` for unattended execution | [Using — Command approval](https://cursor.com/docs/cli/using), `agent --help` |
| File write restrictions | `--force` + permissions in `~/.cursor/cli-config.json` or `<project>/.cursor/cli.json` | [Headless CLI](https://cursor.com/docs/cli/headless), [Permissions](https://cursor.com/docs/cli/reference/permissions) |
| Web fetch approval | Add `WebFetch(...)` to permissions `allow` | [Permissions](https://cursor.com/docs/cli/reference/permissions) |
| Denied commands/files | `permissions.deny` overrides allow; `--force` is “unless explicitly denied” | `agent --help`, [Permissions](https://cursor.com/docs/cli/reference/permissions) |
| Missing auth | `CURSOR_API_KEY` or prior `agent login` | [Authentication](https://cursor.com/docs/cli/reference/authentication) |
| `--auto-review` | May still prompt for non-“safe” tools | `agent --help` (local) — prefer `--force` for full unattended |

### Sandbox

- Global override: `--sandbox enabled|disabled` on the agent invocation.  
- Persistent config: `agent sandbox enable|disable|reset`.  
- `agent sandbox run` wraps **individual shell commands** in a sandbox — separate from the main agent subprocess Ship would spawn.

Sources: `agent --help`, [Parameters — Sandbox](https://cursor.com/docs/cli/reference/parameters), `agent sandbox run --help` (local)

---

## 11. Modes (`--mode`, `--plan`)

| Mode | Flag | Writes? | Source |
|------|------|---------|--------|
| Agent (default) | (none) | Yes (with `--force` in print mode) | [Using — Modes](https://cursor.com/docs/cli/using) |
| Plan | `--plan` / `--mode=plan` | Read-only / planning | [Using — Modes](https://cursor.com/docs/cli/using) |
| Ask | `--mode=ask` | Read-only Q&A | [Using — Modes](https://cursor.com/docs/cli/using) |

Ship phases:

- **Implement** — default agent mode (not plan/ask).  
- **Review** — default agent mode (review may adjust commits; `--mode=ask` would forbid edits).  
- **Final** — default agent mode.

Rules/context: CLI loads `.cursor/rules`, `AGENTS.md`, `CLAUDE.md` automatically ([Using — Rules](https://cursor.com/docs/cli/using)).

---

## 12. Recommended invocation patterns for Ship phases

### Shared base command

```bash
AGENT="${AGENT:-agent}"          # or absolute path ~/.local/bin/agent
CHECKOUT="/path/to/repo"
MODEL="composer-2.5"               # example; use agent models to list IDs
PROMPT_FILE="/path/to/phase-prompt.md"

cd "$CHECKOUT" || exit 1

"$AGENT" -p \
  --trust \
  --force \
  --approve-mcps \
  --workspace "$CHECKOUT" \
  --model "$MODEL" \
  --output-format stream-json \
  --stream-partial-output \
  < "$PROMPT_FILE"
```

Environment for CI/unattended:

```bash
export CURSOR_API_KEY="..."   # required if not using stored login session
# optional: NO_OPEN_BROWSER=1 for login flows on developer machines
```

### Per-phase notes

| Phase | Mode | `--force` | Output suggestion |
|-------|------|-----------|-------------------|
| **Implement** | default agent | **Required** (edits + commands) | `stream-json` for Ship progress logs |
| **Review** | default agent | **Required** (may adjust commits) | `stream-json` or `json` |
| **Final** | default agent | **Required** (tests, PR creation) | `stream-json` |

### Fresh session guarantee

- One subprocess per phase.  
- Never pass `--resume` / `--continue`.  
- Do not reuse `session_id` across phases.

### Success detection (adapter logic)

```text
1. Wait for process exit
2. If exit != 0 → phase failed (read stderr)
3. If using json/stream-json:
   - json: parse stdout JSON; expect type=result, subtype=success, is_error=false
   - stream-json: require terminal event type=result, subtype=success
4. Optionally capture session_id / duration_ms for logging
```

### Timeout wrapper (Ship-owned)

```bash
timeout "${PHASE_TIMEOUT_SECS}" "$AGENT" -p --trust --force ...
# GNU timeout: exit 124 on timeout (not a Cursor code)
```

---

## 13. Cursor SDK vs CLI (not the same entrypoint)

The **Cursor SDK** (`@cursor/sdk`, `cursor-sdk`) runs agents programmatically via `Agent.prompt()` / cloud APIs ([SDK docs](https://cursor.com/docs/sdk/typescript)). That is a **library/HTTP** integration path.

Ship’s Cursor adapter ticket targets the **`agent` subprocess CLI** documented at [cursor.com/docs/cli](https://cursor.com/docs/cli/overview). The SDK is an alternative architecture, not a wrapper around `agent -p`.

For advanced custom clients, Cursor also documents **`agent acp`** (ACP server over stdio) — [ACP](https://cursor.com/docs/cli/acp). ACP requires the client to answer `session/request_permission` or execution blocks. The `-p` CLI path is simpler for Ship’s subprocess model.

---

## 14. Local verification log

Commands run on 2026-07-18 against `/home/max/dev/ship-cli`:

| Command | Result |
|---------|--------|
| `which agent` | `/home/max/.local/bin/agent` |
| `agent --version` | `2026.07.16-899851b` |
| `agent about` | CLI version, model, subscription, OS |
| `agent create-chat` | Printed UUID (empty chat id) |
| Stdin-only `-p` prompt | Honored stdin |
| Positional + stdin | Positional wins |
| Invalid `--model` | Exit `1`, stderr lists available models |
| `printf ... \| agent -p --trust --output-format text` | Exit `0` |

---

## Primary source index

| Topic | URL / command |
|-------|----------------|
| Installation | https://cursor.com/docs/cli/installation |
| Authentication | https://cursor.com/docs/cli/reference/authentication |
| Using / non-interactive | https://cursor.com/docs/cli/using |
| Headless / print scripts | https://cursor.com/docs/cli/headless |
| Parameters reference | https://cursor.com/docs/cli/reference/parameters |
| Output format / exit behavior | https://cursor.com/docs/cli/reference/output-format |
| Permissions | https://cursor.com/docs/cli/reference/permissions |
| Shell Mode (30s limit, interactive) | https://cursor.com/docs/cli/shell-mode |
| ACP (alternative integration) | https://cursor.com/docs/cli/acp |
| Local CLI help | `agent --help`, `agent about`, `agent sandbox run --help` |
