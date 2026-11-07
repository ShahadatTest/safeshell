// Package engine is the application core. It orchestrates Check, Simulate, Run
// and Undo by calling ports (interfaces) that the adapters below implement.
//
// Ports are declared here, on the consumer side. May import: domain, classify, and
// its own port interfaces. Must NOT import sandbox, snapshot, ai, daemon or ipc
// implementations; cmd/ss wires those in (the composition root).
package engine
