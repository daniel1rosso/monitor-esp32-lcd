package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/adapters/collectors"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/adapters/httpapi"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/adapters/mqtt"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/adapters/persistence"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/application"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/buildinfo"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/config"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/logging"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/security"
)

// Execute is the process boundary. Runtime commands are added here as their stages land.
func Execute(args []string, stdout, stderr io.Writer, info buildinfo.Info) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "version":
		if err := json.NewEncoder(stdout).Encode(info); err != nil {
			fmt.Fprintf(stderr, "encode version: %v\n", err)
			return 1
		}

		return 0
	case "server":
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		if err := runServer(ctx, info); err != nil {
			fmt.Fprintf(stderr, "run server: %v\n", err)
			return 1
		}

		return 0
	case "probe":
		endpoint := "http://127.0.0.1:9090/api/v1/health/ready"
		if len(args) > 1 {
			endpoint = args[1]
		}
		if err := httpapi.Probe(context.Background(), endpoint); err != nil {
			fmt.Fprintf(stderr, "probe: %v\n", err)
			return 1
		}

		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runServer(ctx context.Context, info buildinfo.Info) error {
	logger := logging.New()
	cfg, err := config.Load(config.PathFromEnvironment())
	if err != nil {
		return err
	}
	dataDirectory := os.Getenv("DESK_DATA_DIR")
	if cfg.Storage.Driver == "sqlite" {
		if dataDirectory == "" {
			dataDirectory = "data"
		}
		cfg.Storage.SQLite.Path = filepath.Join(dataDirectory, filepath.Base(cfg.Storage.SQLite.Path))
	}
	store, err := persistence.Open(ctx, cfg.Storage)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx, cfg.Storage.Driver); err != nil {
		return err
	}
	ids := security.ULIDGenerator{}
	if err := store.Seed(ctx, cfg, ids, time.Now().UTC()); err != nil {
		return fmt.Errorf("seed configuration: %w", err)
	}
	keyPath := os.Getenv("DESK_JWT_PRIVATE_KEY_PATH")
	if keyPath == "" {
		keyPath = filepath.Join(dataDirectory, "jwt-ed25519.pem")
	}
	tokens, err := security.LoadOrCreateJWT(keyPath, "desk-monitor", ids)
	if err != nil {
		return err
	}
	mqttConfig := mqtt.ConfigFromEnvironment()
	credentialManager, mqttErr := mqtt.NewDynamicSecurity(mqttConfig)
	if mqttErr == nil {
		mqttErr = credentialManager.EnsureBackend(ctx)
	}
	if mqttErr != nil {
		logger.Warn("MQTT control unavailable", "error", mqttErr)
	}
	styles := make(map[domain.Priority]application.NotificationStyle, len(cfg.Devices.Styles))
	for priority, style := range cfg.Devices.Styles {
		styles[domain.Priority(priority)] = application.NotificationStyle{Color: style.Color, Icon: style.Icon, LEDRGB: style.LEDRGB, Sound: style.Sound}
	}
	mqttPublicURL := os.Getenv("DESK_MQTT_PUBLIC_URL")
	if mqttPublicURL == "" {
		mqttPublicURL = "ws://localhost:9001/mqtt"
	}
	content := application.DeviceContentConfig{Timezone: cfg.Platform.Timezone, MQTTURL: mqttPublicURL, Orientation: cfg.Devices.Display.Orientation, RefreshInterval: cfg.Devices.RefreshInterval.Duration, DashboardTTL: cfg.Devices.DashboardTTL.Duration, Width: cfg.Devices.Display.Width, Height: cfg.Devices.Display.Height, Brightness: cfg.Devices.Display.Brightness, MaxScreens: cfg.Devices.MaxScreens, MaxBytes: cfg.Devices.MaxDashboardSize}
	service := application.New(application.Dependencies{Products: store, Services: store, Profiles: store, Devices: store, Users: store, Sessions: store, Alerts: store, Outbox: store, Snapshots: store, Operations: store, Hasher: security.Argon2ID{}, Tokens: tokens, IDs: ids, MQTT: credentialManager, DeviceTokenTTL: cfg.Devices.AccessTokenTTL.Duration, AlertDuration: cfg.Devices.AlertDuration.Duration, Styles: styles, Content: content})
	if err := service.BootstrapAdmin(ctx, os.Getenv("DESK_BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("DESK_BOOTSTRAP_ADMIN_PASSWORD")); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	httpConfig := httpapi.ConfigFromEnvironment()
	httpConfig.ReadTimeout = cfg.HTTP.ReadTimeout.Duration
	httpConfig.WriteTimeout = cfg.HTTP.WriteTimeout.Duration
	httpConfig.IdleTimeout = cfg.HTTP.IdleTimeout.Duration
	httpConfig.AllowedOrigins = cfg.HTTP.AllowedOrigins
	httpConfig.PublicURL = cfg.Platform.PublicURL
	integrations := httpapi.IntegrationConfig{
		GitHubWebhookSecret: optionalSecret(cfg.Integrations.GitHub.WebhookSecret, logger),
		GitHubActionsToken:  optionalSecret(cfg.Integrations.GitHub.ActionsToken, logger),
		UptimeToken:         optionalSecret(cfg.Integrations.UptimeKuma.Token, logger),
		UptimeMonitors:      make(map[string]application.MonitorTarget, len(cfg.Integrations.UptimeKuma.Monitors)),
	}
	for _, monitor := range cfg.Integrations.UptimeKuma.Monitors {
		integrations.UptimeMonitors[monitor.Name] = application.MonitorTarget{ProductKey: monitor.ProductKey, ServiceKey: monitor.ServiceKey}
	}
	collectors.Run(ctx, store, ids, logger, collectors.NewCotizacionYa(cfg.Collectors.CotizacionYa, ids), collectors.NewOpenMeteo(cfg.Collectors.OpenMeteo, ids))
	if mqttErr == nil {
		publisher, err := mqtt.NewPublisher(mqttConfig)
		if err != nil {
			logger.Warn("MQTT publisher unavailable; outbox will retain events", "error", err)
		} else {
			go application.RunOutbox(ctx, store, publisher, 500*time.Millisecond)
		}
	}
	logger.Info("backend starting", "version", info.Version, "storage", cfg.Storage.Driver)
	return httpapi.Run(ctx, httpConfig, httpapi.Dependencies{Application: service, Health: store, Logger: logger, Build: info, Integrations: integrations})
}

func optionalSecret(reference string, logger interface{ Warn(string, ...any) }) string {
	value, err := config.ResolveSecret(reference)
	if err != nil {
		logger.Warn("optional integration disabled", "reference", reference, "error", err)
		return ""
	}
	return value
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "Desk Monitor Backend")
	fmt.Fprintln(output, "usage: desk-monitor <server|version|probe> [arguments]")
}
