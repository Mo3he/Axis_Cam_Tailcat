package main

import (
	"net"
	"strconv"
)

// Camera services are dialed on [::1]. The web server's loopback virtual hosts
// differ by AXIS OS version: AXIS OS 13 answers 127.0.0.1 in plain HTTP on every
// port, 443 included, and AXIS OS 12 does the same for other 127.x addresses.
// Both serve [::1] like the network, with TLS on 443.
const (
	cameraHost   = "::1"
	loopbackHost = "127.0.0.1"
)

// dialCamera connects to a camera port. A port that cannot be reached on [::1]
// is retried on 127.0.0.1, so a served service that listens only there (such
// as port 80 on an HTTPS-only device) stays reachable.
func dialCamera(port int) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", net.JoinHostPort(cameraHost, strconv.Itoa(port)), dialTimeout)
	if err != nil {
		return net.DialTimeout("tcp", net.JoinHostPort(loopbackHost, strconv.Itoa(port)), dialTimeout)
	}
	return c, err
}
