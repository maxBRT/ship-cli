// Package gitops prepares the Run branch and undoes Ticket commits on Abort.
//
// Ship operates on the current checkout only: it does not create, pool, or
// destroy worktrees (see ADR 0001).
//
// UndoToRestorePoint uses a hard reset. MVP assumes a clean working tree or
// ship-controlled dirty state when Abort undoes a Ticket's commits.
package gitops
