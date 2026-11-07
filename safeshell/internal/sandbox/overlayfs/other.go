//go:build !linux

package overlayfs

import (
	"context"
	"errors"

	"safeshell/internal/sandbox"
)

// Run is unavailable: the sandbox needs Linux namespaces and OverlayFS.
func Run(context.Context, sandbox.Spec) (sandbox.Result, error) {
	return sandbox.Result{}, errors.New("the sandbox needs Linux")
}

// Init is unavailable on this system.
func Init() int { return 1 }
