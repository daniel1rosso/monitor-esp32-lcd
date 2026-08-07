package config

import (
	"os"
	"path/filepath"
	"testing"
)

const validConfig = `
version: 1
platform: {name: Desk Monitor Platform, timezone: America/Argentina/Cordoba, public_url: "http://localhost:8180"}
storage:
  driver: sqlite
  sqlite: {path: desk.db, busy_timeout: 5s, wal: true}
  retention: {market_quotes: 1h, deployments: 1h, alerts: 1h, weather: 1h, telemetry: 1h, webhook_deliveries: 1h, collector_runs: 1h}
http: {read_timeout: 10s, write_timeout: 15s, idle_timeout: 60s, trusted_proxies: [], allowed_origins: []}
devices:
  access_token_ttl: 15m
  alert_duration: 15s
  dashboard_ttl: 15m
  refresh_interval: 5m
  max_screens: 12
  max_dashboard_bytes: 65536
  default_profile: operations
  display: {width: 172, height: 320, orientation: portrait, brightness_percent: 80}
  notification_styles:
    critical: {color: "#FF0000", icon: alert, led_rgb: "#FF0000", sound: future_default}
    warning: {color: "#FFAA00", icon: warning, led_rgb: "#FFAA00", sound: none}
    info: {color: "#0000FF", icon: info, led_rgb: "#0000FF", sound: none}
    success: {color: "#00FF00", icon: success, led_rgb: "#00FF00", sound: none}
products:
  - {key: freshcode, name: FreshCode, enabled: true}
profiles:
  - key: operations
    name: Operations
    screens: []
collectors:
  cotizacion_ya: {enabled: true, interval: 5m, timeout: 10s, base_url: "https://quotes.example/api", symbols: [blue]}
  open_meteo:
    enabled: true
    interval: 15m
    timeout: 10s
    base_url: "https://weather.example/api"
    location: {key: rio-cuarto, name: Rio Cuarto, latitude: -33.1, longitude: -64.3, timezone: America/Argentina/Cordoba}
  http_json: []
integrations:
  github: {webhook_secret: "${secret:GITHUB_WEBHOOK_SECRET}", actions_token: "${secret:GITHUB_ACTIONS_TOKEN}"}
  uptime_kuma: {token: "${secret:UPTIME_KUMA_TOKEN}"}
`

func TestLoadValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Platform.Name != "Desk Monitor Platform" || len(cfg.Products) != 1 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nunknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() accepted an unknown field")
	}
}
