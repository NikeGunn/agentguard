package cli

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.RetentionDays != 30 || c.Telemetry != "off" || c.Theme != "system" || c.CloudSync {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadConfigMissingReturnsDefaults(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg != DefaultConfig() {
		t.Fatalf("missing file should yield defaults, got %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	want := Config{RetentionDays: 7, Telemetry: "anonymous", Theme: "dark", CloudSync: true}
	if err := SaveConfig(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("roundtrip: got %+v want %+v", got, want)
	}
}

func TestConfigSetValid(t *testing.T) {
	c := DefaultConfig()
	cases := map[string]string{
		"retention.days": "90",
		"telemetry":      "anonymous",
		"theme":          "light",
		"cloud.sync":     "true",
	}
	for k, v := range cases {
		if err := c.set(k, v); err != nil {
			t.Fatalf("set %s=%s: %v", k, v, err)
		}
	}
	if c.RetentionDays != 90 || c.Telemetry != "anonymous" || c.Theme != "light" || !c.CloudSync {
		t.Fatalf("set produced %+v", c)
	}
}

func TestConfigSetInvalid(t *testing.T) {
	c := DefaultConfig()
	bad := [][2]string{
		{"retention.days", "-1"},
		{"retention.days", "abc"},
		{"telemetry", "verbose"},
		{"theme", "neon"},
		{"cloud.sync", "maybe"},
		{"unknown.key", "x"},
	}
	for _, kv := range bad {
		if err := c.set(kv[0], kv[1]); err == nil {
			t.Fatalf("set %s=%s should have errored", kv[0], kv[1])
		}
	}
}

func TestConfigAsMap(t *testing.T) {
	m := Config{RetentionDays: 15, Telemetry: "off", Theme: "dark", CloudSync: false}.asMap()
	if m["retention.days"] != "15" || m["theme"] != "dark" || m["cloud.sync"] != "false" {
		t.Fatalf("asMap = %+v", m)
	}
}
