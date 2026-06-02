package util

import (
	"runtime"
	"strings"
)

var runtimeGOOS = runtime.GOOS

// CurrentGOOS returns the operating system identifier reported by the runtime.
func CurrentGOOS() string {
	return runtimeGOOS
}

// IsMacOS reports whether the current GOOS is macOS.
func IsMacOS() bool {
	return strings.EqualFold(runtimeGOOS, "darwin")
}

// IsLinux reports whether the current GOOS is Linux.
func IsLinux() bool {
	return strings.EqualFold(runtimeGOOS, "linux")
}

// SetGOOSForTest overrides the runtime-reported GOOS until the returned
// cleanup function is called. Intended for use within tests only.
func SetGOOSForTest(goos string) func() {
	previous := runtimeGOOS
	runtimeGOOS = goos
	return func() {
		runtimeGOOS = previous
	}
}
