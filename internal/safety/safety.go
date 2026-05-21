// Package safety provides safety mechanisms for Forge agents,
// combining environment checkpoints, universal undo, graceful shutdown,
// and clean process termination.
//
// Sub-packages:
//   - safety/snapshot: Environment checkpoints with create, list, restore, diff, delete
//   - safety/undo: Universal agent undo for reverting agent actions
//   - safety/graceful: Graceful shutdown with state persistence and drain
//   - safety/shutdown: SIGTERM/SIGINT handling with session resumption
package safety
