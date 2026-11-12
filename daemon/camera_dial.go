package main

import (
	"errors"
	"net"
	"strconv"
	"syscall"
)

// Camera services are dialed on 127.0.0.2, not 127.0.0.1. Since AXIS OS 13 the
// web server answers 127.0.0.1 in plain HTTP on every port, 443 included, so
// HTTPS through the tunnel failed. Any other loopback address reaches the same
// listeners LAN clients use.
const (
	cameraHost   = "127.0.0.2"
	loopbackHost = "127.0.0.1"
)

// dialCamera connects to a camera port. A port refused on 127.0.0.2 is retried
// on 127.0.0.1, so a served service bound only to 127.0.0.1 stays reachable.
func dialCamera(port int) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", net.JoinHostPort(cameraHost, strconv.Itoa(port)), dialTimeout)
	if err != nil && errors.Is(err, syscall.ECONNREFUSED) {
		return net.DialTimeout("tcp", net.JoinHostPort(loopbackHost, strconv.Itoa(port)), dialTimeout)
	}
	return c, err
}
