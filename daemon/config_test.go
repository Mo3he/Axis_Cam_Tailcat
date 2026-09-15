package main

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestDefaultConfigIsSafe(t *testing.T) {
	c := DefaultConfig()
	if c.AutoStart {
		t.Error("tunnel must not start on its own by default")
	}
	if c.PinAddress {
		t.Error("key must be ephemeral by default")
	}
	if c.ExitNodeMode != ExitNodeOff {
		t.Errorf("exit node must default to off, got %q", c.ExitNodeMode)
	}
	if len(c.AllowedClients) != 0 {
		t.Error("allow-list must default to empty")
	}
	if c.AutoStopMode != AutoStopOff {
		t.Errorf("auto-stop must default to off, got %q", c.AutoStopMode)
	}
	if c.Verbose {
		t.Error("verbose logging must default to off")
	}
	if want := []int{80, 443, 554, 22}; !slices.Equal(c.Ports, want) {
		t.Errorf("default ports = %v, want %v", c.Ports, want)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("default config must validate: %v", err)
	}
}

func TestValidateRejectsUnsafeOrBrokenConfigs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"port zero", func(c *Config) { c.Ports = []int{0} }},
		{"port too high", func(c *Config) { c.Ports = []int{65536} }},
		{"unknown exit node mode", func(c *Config) { c.ExitNodeMode = "maybe" }},
		{"restricted exit node with no prefixes", func(c *Config) {
			c.ExitNodeMode = ExitNodeRestricted
			c.ExitNodeAllow = nil
		}},
		{"restricted exit node with bad prefix", func(c *Config) {
			c.ExitNodeMode = ExitNodeRestricted
			c.ExitNodeAllow = []string{"192.168.1.1"}
		}},
		{"unknown auto-stop mode", func(c *Config) { c.AutoStopMode = "soon" }},
		{"auto-stop without minutes", func(c *Config) {
			c.AutoStopMode = AutoStopIdle
			c.AutoStopMinutes = 0
		}},
		{"plaintext derp url", func(c *Config) { c.DERPMapURL = "http://example.com/d.json" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := DefaultConfig()
			tt.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Fatal("expected validation to fail, got nil")
			}
		})
	}
}

func TestValidateNormalises(t *testing.T) {
	c := DefaultConfig()
	c.Ports = []int{443, 80, 443, 22}
	c.AllowedClients = []string{"  nodekey:aa  ", "", "   "}

	if err := c.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if want := []int{22, 80, 443}; !slices.Equal(c.Ports, want) {
		t.Errorf("ports = %v, want deduplicated and sorted %v", c.Ports, want)
	}
	if want := []string{"nodekey:aa"}; !slices.Equal(c.AllowedClients, want) {
		t.Errorf("allowedClients = %q, want %q", c.AllowedClients, want)
	}
}

func TestStoreRoundTripsAndIsolatesCopies(t *testing.T) {
	dir := t.TempDir()

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if _, err := filepath.Abs(dir); err != nil {
		t.Fatal(err)
	}

	cfg := store.Get()
	cfg.Ports = []int{8080}
	cfg.AutoStopMode = AutoStopAbsolute
	cfg.AutoStopMinutes = 30
	if err := store.Set(cfg); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Mutating a returned copy must not affect stored state.
	got := store.Get()
	got.Ports[0] = 9999
	if store.Get().Ports[0] != 8080 {
		t.Error("Get returned a slice aliasing internal state")
	}

	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if p := reopened.Get().Ports; !slices.Equal(p, []int{8080}) {
		t.Errorf("ports after reload = %v, want [8080]", p)
	}
	if reopened.Get().AutoStopMinutes != 30 {
		t.Error("auto-stop minutes did not persist")
	}
}
