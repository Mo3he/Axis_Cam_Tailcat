package main

import (
	"net"
	"testing"
)

func TestDialCamera(t *testing.T) {
	if ln, err := net.Listen("tcp6", "[::1]:0"); err != nil {
		t.Skip("no IPv6 loopback:", err)
	} else {
		ln.Close()
	}
	listen := func(addr string) int {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ln.Close() })
		return ln.Addr().(*net.TCPAddr).Port
	}

	// A service on all addresses, like the camera's web server, is reached on
	// [::1]; one bound only to 127.0.0.1 through the fallback.
	for addr, want := range map[string]string{":0": cameraHost, "127.0.0.1:0": loopbackHost} {
		c, err := dialCamera(listen(addr))
		if err != nil {
			t.Fatalf("service on %s: %v", addr, err)
		}
		if got := c.RemoteAddr().(*net.TCPAddr).IP.String(); got != want {
			t.Errorf("service on %s dialed on %s, want %s", addr, got, want)
		}
		c.Close()
	}
}
