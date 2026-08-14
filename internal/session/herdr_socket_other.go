//go:build !unix

package session

import (
	"fmt"
	"net"
	"time"
)

func dialHerdrSocket(path string, timeout time.Duration) (net.Conn, error) {
	return nil, fmt.Errorf("Herdr API sockets are unsupported on this platform: %s", path)
}
