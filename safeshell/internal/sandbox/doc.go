// Package sandbox defines the Simulator port and shared helpers (diff walking,
// effect assembly). Implementations live in subpackages:
//
//	overlayfs  Linux user/mount/PID/net namespaces + OverlayFS, observed file diff
//	effects    per-effect adapters: network, process, service, database, git, pkg, disk
//
// Linux-only code carries //go:build linux.
package sandbox
