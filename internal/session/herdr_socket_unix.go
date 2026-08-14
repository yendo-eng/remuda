//go:build unix

package session

import (
	"context"
	"net"
	"time"
)

func dialHerdrSocket(path string, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{Timeout: timeout}
	return dialer.DialContext(context.Background(), "unix", path)
}
