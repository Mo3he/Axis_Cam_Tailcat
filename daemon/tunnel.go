package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tailscale/tailcat"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
	"tailscale.com/types/logger"
	"tailscale.com/wgengine/filter"
)

// dialTimeout bounds the connect to the camera's own service. Loopback, so a
// slow answer means the service is down rather than far away.
const dialTimeout = 5 * time.Second

// ClientInfo describes one connected tailcat client for the UI. The path tells
// an operator whether traffic is peer-to-peer or going through a rate-limited
// public DERP relay, which is the usual explanation for poor video.
type ClientInfo struct {
	NodeKey  string `json:"nodeKey"`
	Path     string `json:"path"`
	Relayed  bool   `json:"relayed"`
	LastSeen string `json:"lastSeen,omitempty"`
}

// PortInfo reports whether the camera is actually listening on a forwarded
// port. Port 22 is commonly closed because AXIS OS ships SSH disabled.
type PortInfo struct {
	Port      int  `json:"port"`
	Listening bool `json:"listening"`
}

// Status is the tunnel state reported to the web UI. Address is included
// because the UI is the intended way to read it; it is never logged.
type Status struct {
	Running     bool         `json:"running"`
	Address     string       `json:"address,omitempty"`
	Pinned      bool         `json:"pinned"`
	StartedAt   string       `json:"startedAt,omitempty"`
	StopsAt     string       `json:"stopsAt,omitempty"`
	Clients     []ClientInfo `json:"clients"`
	Ports       []PortInfo   `json:"ports"`
	ExitNode    string       `json:"exitNode"`
	AnyRelayed  bool         `json:"anyRelayed"`
	LastError   string       `json:"lastError,omitempty"`
	TailcatVers string       `json:"tailcatVersion"`
}

// Manager owns the tailcat server lifecycle.
type Manager struct {
	store   *Store
	keyPath string

	mu        sync.Mutex
	srv       *tailcat.Server
	addr      string
	startedAt time.Time
	stopsAt   time.Time
	pinned    bool
	lastErr   string
	stopTimer *time.Timer
	lastSeen  time.Time
}

func NewManager(store *Store, stateDir string) *Manager {
	return &Manager{store: store, keyPath: filepath.Join(stateDir, "key.json")}
}

// Start brings the tunnel up using the current configuration. It is a no-op if
// the tunnel is already running.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.srv != nil {
		return nil
	}

	cfg := m.store.Get()

	logf := logger.Discard
	if cfg.Verbose {
		// Never logs the address: that string is the tunnel credential and
		// would otherwise reach syslog and any server report.
		logf = func(format string, args ...any) { log.Printf(format, args...) }
	}

	priv, pinned, err := m.loadKey(cfg.PinAddress)
	if err != nil {
		m.lastErr = err.Error()
		return err
	}

	srv := &tailcat.Server{
		Key:          priv.Private,
		PresharedKey: priv.Public.PresharedKey,
		Logf:         logf,
		DERPMapURL:   cfg.DERPMapURL,
	}
	if cfg.RegionID > 0 {
		srv.RegionID = tailcfg.DERPRegionID(cfg.RegionID)
	}

	for _, p := range cfg.Ports {
		srv.ServedTCPPorts = append(srv.ServedTCPPorts,
			filter.PortRange{First: uint16(p), Last: uint16(p)})
	}

	allowed := make(map[uint16]bool, len(cfg.Ports))
	for _, p := range cfg.Ports {
		allowed[uint16(p)] = true
	}
	srv.OnTCP = func(port uint16) func(net.Conn) {
		if !allowed[port] {
			return nil
		}
		m.touch()
		return func(c net.Conn) { proxyTCP(c, fmt.Sprintf("127.0.0.1:%d", port), logf) }
	}

	if cfg.ExitNodeMode != ExitNodeOff {
		prefixes, err := parsePrefixes(cfg.ExitNodeAllow)
		if err != nil {
			m.lastErr = err.Error()
			return err
		}
		restricted := cfg.ExitNodeMode == ExitNodeRestricted
		permit := func(dst netip.AddrPort) bool {
			if !restricted {
				return true
			}
			for _, p := range prefixes {
				if p.Contains(dst.Addr()) {
					return true
				}
			}
			return false
		}
		srv.AllowProxy = permit
		srv.OnTCPForward = func(dst netip.AddrPort) func(net.Conn) {
			if !permit(dst) {
				return nil
			}
			m.touch()
			return func(c net.Conn) { proxyTCP(c, dst.String(), logf) }
		}
		srv.OnUDPForward = func(dst netip.AddrPort) func(tailcat.ConnPacketConn) {
			if !permit(dst) {
				return nil
			}
			m.touch()
			return func(c tailcat.ConnPacketConn) {
				up, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(dst))
				if err != nil {
					c.Close()
					return
				}
				tailcat.ProxyPacketConns(c, up)
			}
		}
	}

	for _, raw := range cfg.AllowedClients {
		var k key.NodePublic
		if err := k.UnmarshalText([]byte(raw)); err != nil {
			m.lastErr = fmt.Sprintf("invalid client key %q", raw)
			return fmt.Errorf("invalid client key %q: %w", raw, err)
		}
		srv.AllowedClients = append(srv.AllowedClients, k)
	}

	if err := srv.Start(); err != nil {
		srv.Close()
		m.lastErr = err.Error()
		return fmt.Errorf("start tunnel: %w", err)
	}

	m.srv = srv
	m.addr = string(srv.TailcatAddr())
	m.startedAt = time.Now()
	m.lastSeen = m.startedAt
	m.pinned = pinned
	m.lastErr = ""
	m.armAutoStopLocked(cfg)
	return nil
}

// Stop tears the tunnel down. With an ephemeral key the address is dead
// permanently once this returns.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	if m.stopTimer != nil {
		m.stopTimer.Stop()
		m.stopTimer = nil
	}
	if m.srv == nil {
		return
	}
	m.srv.Close()
	m.srv = nil
	m.addr = ""
	m.stopsAt = time.Time{}
}

// Restart applies configuration changes to a running tunnel.
func (m *Manager) Restart() error {
	m.Stop()
	return m.Start()
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := m.store.Get()
	st := Status{
		Running:     m.srv != nil,
		Pinned:      m.pinned,
		ExitNode:    cfg.ExitNodeMode,
		LastError:   m.lastErr,
		TailcatVers: tailcatVersion,
		Clients:     []ClientInfo{},
		Ports:       []PortInfo{},
	}
	for _, p := range cfg.Ports {
		st.Ports = append(st.Ports, PortInfo{Port: p, Listening: portListening(p)})
	}
	if m.srv == nil {
		return st
	}

	st.Address = m.addr
	st.StartedAt = m.startedAt.UTC().Format(time.RFC3339)
	if !m.stopsAt.IsZero() {
		st.StopsAt = m.stopsAt.UTC().Format(time.RFC3339)
	}

	for _, peer := range m.srv.Status().Peer {
		info := ClientInfo{NodeKey: peer.PublicKey.String()}
		switch {
		case peer.CurAddr != "":
			info.Path = "direct " + peer.CurAddr
		case peer.Relay != "":
			info.Path = "relayed via " + peer.Relay
			info.Relayed = true
			st.AnyRelayed = true
		default:
			info.Path = "connecting"
		}
		if !peer.LastSeen.IsZero() {
			info.LastSeen = peer.LastSeen.UTC().Format(time.RFC3339)
		}
		st.Clients = append(st.Clients, info)
	}
	return st
}

// touch records activity for the idle auto-stop timer.
func (m *Manager) touch() {
	m.mu.Lock()
	m.lastSeen = time.Now()
	m.mu.Unlock()
}

// armAutoStopLocked schedules the auto-stop timer. m.mu must be held.
func (m *Manager) armAutoStopLocked(cfg Config) {
	if m.stopTimer != nil {
		m.stopTimer.Stop()
		m.stopTimer = nil
	}
	if cfg.AutoStopMode == AutoStopOff {
		m.stopsAt = time.Time{}
		return
	}

	d := time.Duration(cfg.AutoStopMinutes) * time.Minute
	m.stopsAt = time.Now().Add(d)
	m.stopTimer = time.AfterFunc(d, func() { m.onAutoStop(cfg) })
}

func (m *Manager) onAutoStop(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.srv == nil {
		return
	}
	if cfg.AutoStopMode == AutoStopIdle {
		idle := time.Since(m.lastSeen)
		want := time.Duration(cfg.AutoStopMinutes) * time.Minute
		if idle < want {
			// Activity since the timer was armed: wait out the remainder.
			remaining := want - idle
			m.stopsAt = time.Now().Add(remaining)
			m.stopTimer = time.AfterFunc(remaining, func() { m.onAutoStop(cfg) })
			return
		}
	}
	log.Printf("auto-stop (%s) reached, closing tunnel", cfg.AutoStopMode)
	m.stopLocked()
}

// loadKey returns the key to serve with. A pinned key is persisted 0600 in
// localdata and never placed in the camera parameter store, since key plus
// address is complete tunnel access.
func (m *Manager) loadKey(pin bool) (*tailcat.PrivateKey, bool, error) {
	if !pin {
		return tailcat.NewPrivateKey(), false, nil
	}

	data, err := os.ReadFile(m.keyPath)
	switch {
	case err == nil:
		pk := &tailcat.PrivateKey{}
		if err := json.Unmarshal(data, pk); err != nil {
			return nil, false, fmt.Errorf("stored key is unreadable: %w", err)
		}
		return pk, true, nil
	case errors.Is(err, os.ErrNotExist):
		pk := tailcat.NewPrivateKey()
		out, err := json.Marshal(pk)
		if err != nil {
			return nil, false, err
		}
		if err := os.WriteFile(m.keyPath, out, 0o600); err != nil {
			return nil, false, fmt.Errorf("save key: %w", err)
		}
		return pk, true, nil
	default:
		return nil, false, fmt.Errorf("read key: %w", err)
	}
}

// ForgetKey deletes the pinned key, permanently killing its address.
func (m *Manager) ForgetKey() error {
	err := os.Remove(m.keyPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m.mu.Lock()
	m.pinned = false
	m.mu.Unlock()
	return nil
}

// proxyTCP splices a tunnel connection onto a local service, propagating
// half-close so request/response protocols that signal with EOF still work.
func proxyTCP(client net.Conn, target string, logf logger.Logf) {
	defer client.Close()

	upstream, err := net.DialTimeout("tcp", target, dialTimeout)
	if err != nil {
		logf("dial %s: %v", target, err)
		return
	}
	defer upstream.Close()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(upstream, client)
		closeWrite(upstream)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(client, upstream)
		closeWrite(client)
		done <- struct{}{}
	}()
	<-done
	<-done
}

type closeWriter interface{ CloseWrite() error }

func closeWrite(c net.Conn) {
	if cw, ok := c.(closeWriter); ok {
		cw.CloseWrite()
	}
}

func portListening(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func parsePrefixes(entries []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(entries))
	for _, e := range entries {
		p, err := netip.ParsePrefix(strings.TrimSpace(e))
		if err != nil {
			return nil, fmt.Errorf("bad prefix %q: %w", e, err)
		}
		out = append(out, p)
	}
	return out, nil
}
