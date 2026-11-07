//go:build !(linux && (amd64 || arm64))

// Package netwatch records network attempts made by a sandboxed command. It is
// only implemented on Linux amd64 and arm64.
package netwatch
