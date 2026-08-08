// Package config loads and validates the immutable deployment configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var keyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)
var secretPattern = regexp.MustCompile(`^\$\{secret:[A-Z][A-Z0-9_]*\}$`)
var colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	value, err := time.ParseDuration(node.Value)
	if err != nil || value <= 0 {
		return fmt.Errorf("invalid positive duration %q", node.Value)
	}
	d.Duration = value
	return nil
}

type Config struct {
	Version      int          `yaml:"version"`
	Platform     Platform     `yaml:"platform"`
	Storage      Storage      `yaml:"storage"`
	HTTP         HTTP         `yaml:"http"`
	Devices      Devices      `yaml:"devices"`
	Products     []Product    `yaml:"products"`
	Profiles     []Profile    `yaml:"profiles"`
	Collectors   Collectors   `yaml:"collectors"`
	Integrations Integrations `yaml:"integrations"`
}

type Platform struct {
	Name      string `yaml:"name"`
	Timezone  string `yaml:"timezone"`
	PublicURL string `yaml:"public_url"`
}

type Storage struct {
	Driver    string              `yaml:"driver"`
	SQLite    SQLite              `yaml:"sqlite"`
	Retention map[string]Duration `yaml:"retention"`
}

type SQLite struct {
	Path        string   `yaml:"path"`
	BusyTimeout Duration `yaml:"busy_timeout"`
	WAL         bool     `yaml:"wal"`
}

type HTTP struct {
	ReadTimeout    Duration `yaml:"read_timeout"`
	WriteTimeout   Duration `yaml:"write_timeout"`
	IdleTimeout    Duration `yaml:"idle_timeout"`
	TrustedProxies []string `yaml:"trusted_proxies"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type Devices struct {
	AccessTokenTTL   Duration                     `yaml:"access_token_ttl"`
	AlertDuration    Duration                     `yaml:"alert_duration"`
	DashboardTTL     Duration                     `yaml:"dashboard_ttl"`
	RefreshInterval  Duration                     `yaml:"refresh_interval"`
	MaxScreens       int                          `yaml:"max_screens"`
	MaxDashboardSize int                          `yaml:"max_dashboard_bytes"`
	DefaultProfile   string                       `yaml:"default_profile"`
	Display          Display                      `yaml:"display"`
	Styles           map[string]NotificationStyle `yaml:"notification_styles"`
}

type Display struct {
	Width       int    `yaml:"width"`
	Height      int    `yaml:"height"`
	Orientation string `yaml:"orientation"`
	Brightness  int    `yaml:"brightness_percent"`
}

type NotificationStyle struct {
	Color  string `yaml:"color"`
	Icon   string `yaml:"icon"`
	LEDRGB string `yaml:"led_rgb"`
	Sound  string `yaml:"sound"`
}

type Product struct {
	Key      string              `yaml:"key"`
	Name     string              `yaml:"name"`
	Enabled  bool                `yaml:"enabled"`
	Services []ConfiguredService `yaml:"services"`
}

type ConfiguredService struct {
	Key     string `yaml:"key"`
	Name    string `yaml:"name"`
	Kind    string `yaml:"kind"`
	Enabled bool   `yaml:"enabled"`
}

type Profile struct {
	Key     string          `yaml:"key"`
	Name    string          `yaml:"name"`
	Screens []ProfileScreen `yaml:"screens"`
}

type ProfileScreen struct {
	ID       string         `yaml:"id"`
	Type     string         `yaml:"type"`
	Priority int            `yaml:"priority"`
	Duration Duration       `yaml:"duration"`
	Enabled  bool           `yaml:"enabled"`
	Config   map[string]any `yaml:"config"`
}

type ScheduledCollector struct {
	Enabled  bool     `yaml:"enabled"`
	Interval Duration `yaml:"interval"`
	Timeout  Duration `yaml:"timeout"`
}

type Collectors struct {
	CotizacionYa CotizacionYa `yaml:"cotizacion_ya"`
	OpenMeteo    OpenMeteo    `yaml:"open_meteo"`
	HTTPJSON     []HTTPJSON   `yaml:"http_json"`
}

type CotizacionYa struct {
	ScheduledCollector `yaml:",inline"`
	BaseURL            string   `yaml:"base_url"`
	Symbols            []string `yaml:"symbols"`
}

type OpenMeteo struct {
	ScheduledCollector `yaml:",inline"`
	BaseURL            string   `yaml:"base_url"`
	Location           Location `yaml:"location"`
}

type Location struct {
	Key       string  `yaml:"key"`
	Name      string  `yaml:"name"`
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
	Timezone  string  `yaml:"timezone"`
}

type HTTPJSON struct {
	Key        string            `yaml:"key"`
	Enabled    bool              `yaml:"enabled"`
	ProductKey string            `yaml:"product_key"`
	Interval   Duration          `yaml:"interval"`
	Timeout    Duration          `yaml:"timeout"`
	URL        string            `yaml:"url"`
	Allow      HTTPAllow         `yaml:"allow"`
	Headers    map[string]string `yaml:"headers"`
	Mappings   []Mapping         `yaml:"mappings"`
}

type HTTPAllow struct {
	Hosts []string `yaml:"hosts"`
	CIDRs []string `yaml:"cidrs"`
}

type Mapping struct {
	Target     string `yaml:"target"`
	ProductKey string `yaml:"product_key"`
	ServiceKey string `yaml:"service_key"`
	MetricKey  string `yaml:"metric_key"`
	Field      string `yaml:"field"`
	Path       string `yaml:"path"`
	Type       string `yaml:"type"`
	Required   bool   `yaml:"required"`
}

type Integrations struct {
	GitHub     GitHub     `yaml:"github"`
	UptimeKuma UptimeKuma `yaml:"uptime_kuma"`
}

type GitHub struct {
	WebhookSecret string `yaml:"webhook_secret"`
	ActionsToken  string `yaml:"actions_token"`
}

type UptimeKuma struct {
	Token    string          `yaml:"token"`
	Monitors []UptimeMonitor `yaml:"monitors"`
}

type UptimeMonitor struct {
	Name       string `yaml:"name"`
	ProductKey string `yaml:"product_key"`
	ServiceKey string `yaml:"service_key"`
}

func Load(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func PathFromEnvironment() string {
	if value := os.Getenv("DESK_CONFIG_PATH"); value != "" {
		return value
	}
	return "../contracts/config/platform.example.yaml"
}

func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("version must be 1")
	}
	if strings.TrimSpace(c.Platform.Name) == "" {
		return errors.New("platform.name is required")
	}
	if _, err := time.LoadLocation(c.Platform.Timezone); err != nil {
		return fmt.Errorf("platform.timezone: %w", err)
	}
	if parsed, err := url.ParseRequestURI(c.Platform.PublicURL); err != nil || parsed.Scheme == "" {
		return errors.New("platform.public_url must be an absolute URL")
	}
	if c.Storage.Driver != "sqlite" && c.Storage.Driver != "postgres" {
		return errors.New("storage.driver must be sqlite or postgres")
	}
	if c.Storage.Driver == "sqlite" && strings.TrimSpace(c.Storage.SQLite.Path) == "" {
		return errors.New("storage.sqlite.path is required")
	}
	if c.Storage.Driver == "postgres" && os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL is required for postgres")
	}
	for _, name := range []string{"market_quotes", "deployments", "alerts", "weather", "telemetry", "webhook_deliveries", "collector_runs"} {
		if c.Storage.Retention[name].Duration <= 0 {
			return fmt.Errorf("storage.retention.%s is required", name)
		}
	}
	if c.HTTP.ReadTimeout.Duration <= 0 || c.HTTP.WriteTimeout.Duration <= 0 || c.HTTP.IdleTimeout.Duration <= 0 {
		return errors.New("http timeouts must be positive")
	}
	for _, origin := range c.HTTP.AllowedOrigins {
		parsed, err := url.ParseRequestURI(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("invalid allowed origin %q", origin)
		}
	}
	if c.Devices.AccessTokenTTL.Duration <= 0 || c.Devices.AlertDuration.Duration <= 0 || c.Devices.DashboardTTL.Duration <= 0 || c.Devices.RefreshInterval.Duration <= 0 {
		return errors.New("device durations must be positive")
	}
	if c.Devices.MaxScreens < 1 || c.Devices.MaxScreens > 12 {
		return errors.New("devices.max_screens must be between 1 and 12")
	}
	if c.Devices.MaxDashboardSize < 4096 || c.Devices.MaxDashboardSize > 65536 {
		return errors.New("devices.max_dashboard_bytes must be between 4096 and 65536")
	}
	if c.Devices.Display.Width != 172 || c.Devices.Display.Height != 320 || c.Devices.Display.Brightness < 1 || c.Devices.Display.Brightness > 100 {
		return errors.New("invalid device display dimensions or brightness")
	}
	orientations := map[string]bool{"portrait": true, "landscape": true, "portrait_inverted": true, "landscape_inverted": true}
	if !orientations[c.Devices.Display.Orientation] {
		return errors.New("invalid device display orientation")
	}
	for _, priority := range []string{"critical", "warning", "info", "success"} {
		style, exists := c.Devices.Styles[priority]
		if !exists || !colorPattern.MatchString(style.Color) || !colorPattern.MatchString(style.LEDRGB) || style.Icon == "" || (style.Sound != "none" && style.Sound != "future_default") {
			return fmt.Errorf("invalid notification style %q", priority)
		}
	}
	products := make(map[string]struct{}, len(c.Products))
	productServices := make(map[string]map[string]struct{}, len(c.Products))
	for _, product := range c.Products {
		if !keyPattern.MatchString(product.Key) || strings.TrimSpace(product.Name) == "" {
			return fmt.Errorf("invalid product %q", product.Key)
		}
		if _, exists := products[product.Key]; exists {
			return fmt.Errorf("duplicate product key %q", product.Key)
		}
		products[product.Key] = struct{}{}
		services := map[string]struct{}{}
		for _, service := range product.Services {
			if !keyPattern.MatchString(service.Key) || strings.TrimSpace(service.Name) == "" || strings.TrimSpace(service.Kind) == "" {
				return fmt.Errorf("invalid service %q for product %q", service.Key, product.Key)
			}
			if _, exists := services[service.Key]; exists {
				return fmt.Errorf("duplicate service %q for product %q", service.Key, product.Key)
			}
			services[service.Key] = struct{}{}
		}
		productServices[product.Key] = services
	}
	profiles := make(map[string]struct{}, len(c.Profiles))
	screenTypes := map[string]bool{"overview": true, "product_status": true, "service_status": true, "alert": true, "deployment": true, "market": true, "weather": true, "metric_list": true, "message": true}
	for _, profile := range c.Profiles {
		if !keyPattern.MatchString(profile.Key) || strings.TrimSpace(profile.Name) == "" {
			return fmt.Errorf("invalid profile %q", profile.Key)
		}
		if len(profile.Screens) > c.Devices.MaxScreens {
			return fmt.Errorf("profile %q exceeds max screens", profile.Key)
		}
		if _, exists := profiles[profile.Key]; exists {
			return fmt.Errorf("duplicate profile key %q", profile.Key)
		}
		screens := map[string]struct{}{}
		for _, screen := range profile.Screens {
			if !keyPattern.MatchString(screen.ID) || !screenTypes[screen.Type] || screen.Priority < 0 || screen.Priority > 100 || screen.Duration.Duration <= 0 {
				return fmt.Errorf("invalid screen %q in profile %q", screen.ID, profile.Key)
			}
			if _, exists := screens[screen.ID]; exists {
				return fmt.Errorf("duplicate screen %q in profile %q", screen.ID, profile.Key)
			}
			screens[screen.ID] = struct{}{}
		}
		profiles[profile.Key] = struct{}{}
	}
	if _, exists := profiles[c.Devices.DefaultProfile]; !exists {
		return fmt.Errorf("default profile %q does not exist", c.Devices.DefaultProfile)
	}
	for _, collector := range c.Collectors.HTTPJSON {
		if _, exists := products[collector.ProductKey]; !exists {
			return fmt.Errorf("collector %q references unknown product %q", collector.Key, collector.ProductKey)
		}
		parsed, err := url.Parse(collector.URL)
		if err != nil || parsed.Scheme != "https" || len(collector.Allow.Hosts) == 0 || collector.Interval.Duration <= 0 || collector.Timeout.Duration <= 0 || len(collector.Mappings) == 0 {
			return fmt.Errorf("invalid HTTP JSON collector %q", collector.Key)
		}
	}
	if err := validateScheduled("cotizacion_ya", c.Collectors.CotizacionYa.ScheduledCollector, c.Collectors.CotizacionYa.BaseURL); err != nil {
		return err
	}
	if len(c.Collectors.CotizacionYa.Symbols) == 0 {
		return errors.New("cotizacion_ya.symbols is required")
	}
	if err := validateScheduled("open_meteo", c.Collectors.OpenMeteo.ScheduledCollector, c.Collectors.OpenMeteo.BaseURL); err != nil {
		return err
	}
	if !keyPattern.MatchString(c.Collectors.OpenMeteo.Location.Key) || c.Collectors.OpenMeteo.Location.Name == "" || c.Collectors.OpenMeteo.Location.Latitude < -90 || c.Collectors.OpenMeteo.Location.Latitude > 90 || c.Collectors.OpenMeteo.Location.Longitude < -180 || c.Collectors.OpenMeteo.Location.Longitude > 180 {
		return errors.New("invalid open_meteo location")
	}
	for name, reference := range map[string]string{"github.webhook_secret": c.Integrations.GitHub.WebhookSecret, "github.actions_token": c.Integrations.GitHub.ActionsToken, "uptime_kuma.token": c.Integrations.UptimeKuma.Token} {
		if !secretPattern.MatchString(reference) {
			return fmt.Errorf("invalid secret reference for %s", name)
		}
	}
	monitorNames := map[string]struct{}{}
	for _, monitor := range c.Integrations.UptimeKuma.Monitors {
		if strings.TrimSpace(monitor.Name) == "" || !keyPattern.MatchString(monitor.ProductKey) || !keyPattern.MatchString(monitor.ServiceKey) {
			return fmt.Errorf("invalid uptime monitor mapping %q", monitor.Name)
		}
		if _, exists := products[monitor.ProductKey]; !exists {
			return fmt.Errorf("uptime monitor %q references unknown product %q", monitor.Name, monitor.ProductKey)
		}
		if _, exists := productServices[monitor.ProductKey][monitor.ServiceKey]; !exists {
			return fmt.Errorf("uptime monitor %q references unknown service %q", monitor.Name, monitor.ServiceKey)
		}
		key := strings.ToLower(strings.TrimSpace(monitor.Name))
		if _, exists := monitorNames[key]; exists {
			return fmt.Errorf("duplicate uptime monitor %q", monitor.Name)
		}
		monitorNames[key] = struct{}{}
	}
	return nil
}

func validateScheduled(name string, value ScheduledCollector, baseURL string) error {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" || value.Interval.Duration <= 0 || value.Timeout.Duration <= 0 {
		return fmt.Errorf("invalid scheduled collector %q", name)
	}
	return nil
}

func ResolveSecret(reference string) (string, error) {
	const prefix = "${secret:"
	if !strings.HasPrefix(reference, prefix) || !strings.HasSuffix(reference, "}") {
		return "", fmt.Errorf("invalid secret reference")
	}
	name := strings.TrimSuffix(strings.TrimPrefix(reference, prefix), "}")
	value, exists := os.LookupEnv(name)
	if !exists || value == "" {
		return "", fmt.Errorf("secret %s is not configured", name)
	}
	return value, nil
}
