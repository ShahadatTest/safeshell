// Package snapshot stores and restores copies of the paths a command will change.
// Backends: btrfs/zfs when available, otherwise a path-level copy store.
package snapshot
