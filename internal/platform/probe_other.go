//go:build !linux

package platform

// Probe reports nothing on systems other than Linux: the sandbox needs Linux.
func Probe() Report { return Report{UserNSErr: "not Linux"} }
