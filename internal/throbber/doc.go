// Package throbber is the Phase-wait UI for a Ship Run.
//
// The Run orchestrator calls Port.During around each Agent Phase. Tableau is
// the production adapter (full-screen rocket on a TTY; plain lines otherwise).
// Silent is the no-op adapter for tests.
package throbber
