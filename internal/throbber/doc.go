// Package throbber is the Phase-wait UI for a Ship Run.
//
// The Run orchestrator calls Port.During around each Agent Phase. Line is the
// production adapter (one status line with a braille throbber on a TTY; a plain
// waiting line otherwise). Silent is the no-op adapter for tests.
package throbber
