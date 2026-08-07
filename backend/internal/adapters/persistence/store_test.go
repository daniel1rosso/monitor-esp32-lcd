package persistence_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/adapters/persistence"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/application"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/config"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/security"
)

func TestSQLiteBootstrapAndDeviceAuthentication(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{
		Storage:  config.Storage{Driver: "sqlite", SQLite: config.SQLite{BusyTimeout: config.Duration{Duration: time.Second}, WAL: true}},
		Devices:  config.Devices{AccessTokenTTL: config.Duration{Duration: time.Minute}, DefaultProfile: "operations"},
		Products: []config.Product{{Key: "potrerito", Name: "Potrerito", Enabled: true}, {Key: "freshcode", Name: "FreshCode", Enabled: true}, {Key: "facturas", Name: "Facturas", Enabled: true}},
		Profiles: []config.Profile{{Key: "operations", Name: "Operations", Screens: []config.ProfileScreen{{ID: "overview-main", Type: "overview", Priority: 50, Duration: config.Duration{Duration: 8 * time.Second}, Enabled: true}}}},
	}
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "desk-monitor.db")
	store, err := persistence.Open(ctx, cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx, "sqlite"); err != nil {
		t.Fatal(err)
	}
	ids := security.ULIDGenerator{}
	if err := store.Seed(ctx, cfg, ids, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := store.Seed(ctx, cfg, ids, time.Now().UTC()); err != nil {
		t.Fatalf("idempotent seed failed: %v", err)
	}
	products, _, err := store.ListProducts(ctx, 10, "")
	if err != nil || len(products) != 3 {
		t.Fatalf("products = %d, error = %v", len(products), err)
	}
	profile, err := store.GetProfileByKey(ctx, cfg.Devices.DefaultProfile)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := security.LoadOrCreateJWT(filepath.Join(t.TempDir(), "jwt.pem"), "desk-monitor-test", ids)
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(application.Dependencies{Products: store, Services: store, Profiles: store, Devices: store, Users: store, Sessions: store, Alerts: store, Outbox: store, Hasher: security.Argon2ID{}, Tokens: tokens, IDs: ids, DeviceTokenTTL: time.Minute, AlertDuration: 15 * time.Second})
	created, err := service.CreateDevice(ctx, application.DeviceProvision{DeviceID: "desk-test-001", Name: "Test Desk", ProfileID: profile.ID})
	if err != nil {
		t.Fatal(err)
	}
	if created.DeviceSecret == "" || created.MQTTPassword == "" {
		t.Fatal("one-time credentials are empty")
	}
	access, err := service.ExchangeDeviceSecret(ctx, "desk-test-001", created.DeviceSecret, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(access.Value, application.DeviceAudience); err != nil {
		t.Fatalf("issued token rejected: %v", err)
	}
	if _, err := service.ExchangeDeviceSecret(ctx, "desk-test-001", "wrong-secret-that-is-at-least-32-bytes", "0.1.0"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("wrong secret error = %v", err)
	}
}
