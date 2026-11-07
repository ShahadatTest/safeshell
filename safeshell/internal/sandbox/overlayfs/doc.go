// Package overlayfs runs a command inside rootless namespaces with an OverlayFS
// root and reports the observed filesystem diff (created, modified, deleted).
// Uses the re-exec pattern: the binary starts itself as 'ss sandbox-init'.
package overlayfs
