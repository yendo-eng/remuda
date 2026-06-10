package util

import (
	"runtime"
)

var runtimeGOOS = runtime.GOOS

// CurrentGOOS returns the operating system identifier reported by the runtime.
func CurrentGOOS() string {
	return runtimeGOOS
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
