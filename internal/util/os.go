package util

import (
	"runtime"
)

var runtimeGOOS = runtime.GOOS

// CurrentGOOS returns the operating system identifier reported by the runtime.
func CurrentGOOS() string {
	return runtimeGOOS
}
