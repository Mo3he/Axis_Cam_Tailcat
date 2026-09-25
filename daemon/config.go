package main

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Exit node modes.
const (
	ExitNodeOff        = "off"
	ExitNodeRestricted = "restricted"
	ExitNodeFull       = "full"
)

// Auto-stop modes.
const (
	AutoStopOff      = "off"
	AutoStopIdle     = "idle"
	AutoStopAbsolute = "absolute"
)

// Config is the persisted user configuration, stored as JSON in localdata (not
// axparameter) so it works on devices without /axis-cgi/param.cgi.
type Config struct {
	AutoStart bool  `json:"autoStart"`
	Ports     []int `json:"ports"`

	// PinAddress persists the private key so the address survives restarts;
	// otherwise each start uses a fresh ephemeral key.
	PinAddress bool `json:"pinAddress"`

	ExitNodeMode  string   `json:"exitNodeMode"`
	ExitNodeAllow []string `json:"exitNodeAllow"`

	AllowedClients []string `json:"allowedClients"`

	AutoStopMode    string `json:"autoStopMode"`
	AutoStopMinutes int    `json:"autoStopMinutes"`

	DERPMapURL string `json:"derpMapUrl"`
	RegionID   int    `json:"regionId"`

	Verbose bool `json:"verbose"`
}

// DefaultConfig is safe by default: nothing is exposed until Start, the
// address dies with the tunnel, and there is no exit node.
func DefaultConfig() Config {
	return Config{
		AutoStart:       false,
		Ports:           []int{80, 443, 554, 22},
		PinAddress:      false,
		ExitNodeMode:    ExitNodeOff,
		ExitNodeAllow:   []string{},
		AllowedClients:  []string{},
		AutoStopMode:    AutoStopOff,
		AutoStopMinutes: 60,
		Verbose:         false,
	}
}

// Validate normalises the config and rejects values that would produce a
// surprising tunnel. Applied to API input and to the stored config on load.
func (c *Config) Validate() error {
	seen := map[int]bool{}
	ports := make([]int, 0, len(c.Ports))
	for _, p := range c.Ports {
		if p < 1 || p > 65535 {
			return fmt.Errorf("port %d out of range", p)
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		ports = append(ports, p)
	}
	slices.Sort(ports)
	c.Ports = ports

	switch c.ExitNodeMode {
	case ExitNodeOff, ExitNodeRestricted, ExitNodeFull:
	default:
		return fmt.Errorf("unknown exitNodeMode %q", c.ExitNodeMode)
	}

	if c.ExitNodeMode == ExitNodeRestricted && len(c.ExitNodeAllow) == 0 {
		return fmt.Errorf("restricted exit node needs at least one allowed prefix")
	}
	for _, entry := range c.ExitNodeAllow {
		if _, err := netip.ParsePrefix(strings.TrimSpace(entry)); err != nil {
			return fmt.Errorf("exitNodeAllow %q is not a CIDR prefix: %w", entry, err)
		}
	}

	switch c.AutoStopMode {
	case AutoStopOff, AutoStopIdle, AutoStopAbsolute:
	default:
		return fmt.Errorf("unknown autoStopMode %q", c.AutoStopMode)
	}
	if c.AutoStopMode != AutoStopOff && c.AutoStopMinutes < 1 {
		return fmt.Errorf("autoStopMinutes must be at least 1")
	}

	if c.DERPMapURL != "" && !strings.HasPrefix(c.DERPMapURL, "https://") {
		return fmt.Errorf("derpMapUrl must be an https URL")
	}

	for i, k := range c.AllowedClients {
		c.AllowedClients[i] = strings.TrimSpace(k)
	}
	c.AllowedClients = slices.DeleteFunc(c.AllowedClients, func(s string) bool { return s == "" })
	return nil
}

// Store persists Config as JSON and guards concurrent access.
type Store struct {
	path string

	mu  sync.RWMutex
	cfg Config
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	s := &Store{path: filepath.Join(dir, "config.json"), cfg: DefaultConfig()}

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, s.save()
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("stored config is invalid: %w", err)
	}
	s.cfg = cfg
	return s, nil
}

func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.clone()
}

func (s *Store) Set(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
	return s.save()
}

// save must be called with s.mu held, or before the store is shared.
func (s *Store) save() error {
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Rename(tmp, s.path)
}

func (c Config) clone() Config {
	out := c
	out.Ports = slices.Clone(c.Ports)
	out.ExitNodeAllow = slices.Clone(c.ExitNodeAllow)
	out.AllowedClients = slices.Clone(c.AllowedClients)
	return out
}
