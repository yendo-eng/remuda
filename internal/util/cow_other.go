//go:build !darwin && !linux

package util

// cloneFile has no copy-on-write primitive to call on this platform.
func cloneFile(_, _ string) error {
	return errCoWUnsupported
}
