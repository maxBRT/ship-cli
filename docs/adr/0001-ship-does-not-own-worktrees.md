# Ship does not own worktrees

Ship runs inside whatever checkout the developer already has (for example a treehouse worktree). It does not create, pool, lease, or destroy worktrees. That keeps the CLI a pure ticket/phase orchestrator and avoids competing with tools whose job is workspace isolation.
