// Package herdr reports Phase lifecycle state to Herdr (and compatible
// multiplexers) so Agents sidebars show Ship as working while a Phase runs.
//
// The Run orchestrator calls Port.Working at Phase start and Port.Idle at
// Phase end (success, abort, or timeout). Reporter is the production adapter:
// it no-ops unless HERDR_ENV / HERDR_SOCKET_PATH / HERDR_PANE_ID are set.
package herdr
