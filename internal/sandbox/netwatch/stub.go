//go:build linux && !(amd64 || arm64)

// Package netwatch records network attempts made by a sandboxed command. On
// this architecture it is not available: the sandbox still blocks the network
// (it has no route out), it just cannot list what was attempted.
package netwatch

import (
	"errors"
	"os"

	"github.com/samarthsinh2660/safeshell/internal/sandbox"
)

var errUnsupported = errors.New("network recording is not supported on this architecture")

func Install() (*os.File, error)                    { return nil, errUnsupported }
func Offer(sock, listener *os.File) error           { return errUnsupported }
func Take(sock *os.File, pid int) (*os.File, error) { return nil, errUnsupported }
func Supervise(fd *os.File) []sandbox.NetAttempt    { return nil }
