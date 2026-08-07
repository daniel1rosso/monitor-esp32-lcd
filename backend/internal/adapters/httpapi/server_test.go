package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/adapters/persistence"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/application"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/buildinfo"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/config"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/security"
)

func TestPublicHealth(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	recorder := httptest.NewRecorder()
	api := &API{build: buildinfo.New("dev", "unknown", "unknown"), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	api.publicHandler(Config{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response healthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" || response.CheckedAt.IsZero() {
		t.Fatalf("response = %#v", response)
	}
}

func TestLoginAndListProducts(t *testing.T) {
	ctx := context.Background()
	storage := config.Storage{Driver: "sqlite", SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "api.db"), BusyTimeout: config.Duration{Duration: time.Second}, WAL: true}}
	store, err := persistence.Open(ctx, storage)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx, "sqlite"); err != nil {
		t.Fatal(err)
	}
	ids := security.ULIDGenerator{}
	seed := config.Config{Products: []config.Product{{Key: "freshcode", Name: "FreshCode", Enabled: true}}, Profiles: []config.Profile{{Key: "operations", Name: "Operations"}}}
	if err := store.Seed(ctx, seed, ids, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	tokens, err := security.LoadOrCreateJWT(filepath.Join(t.TempDir(), "jwt.pem"), "test", ids)
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(application.Dependencies{Products: store, Services: store, Profiles: store, Devices: store, Users: store, Sessions: store, Alerts: store, Outbox: store, Hasher: security.Argon2ID{}, Tokens: tokens, IDs: ids, DeviceTokenTTL: time.Minute, AlertDuration: 15 * time.Second})
	if err := service.BootstrapAdmin(ctx, "admin@example.com", "a-secure-test-password"); err != nil {
		t.Fatal(err)
	}
	api := &API{app: service, health: store, build: buildinfo.New("test", "test", "2026-08-06T00:00:00Z"), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	router := api.publicHandler(Config{PublicURL: "http://localhost:8180"})

	loginBody := []byte(`{"email":"admin@example.com","password":"a-secure-test-password"}`)
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	var login struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(loginResponse.Body).Decode(&login); err != nil {
		t.Fatal(err)
	}
	loginCookies := loginResponse.Result().Cookies()
	if len(loginCookies) != 1 || loginCookies[0].Name != refreshCookieName || !loginCookies[0].HttpOnly || loginCookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("invalid refresh cookie: %#v", loginCookies)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", login.AccessToken))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("products status = %d, body = %s", response.Code, response.Body.String())
	}
	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("products count = %d", len(page.Items))
	}
	refreshRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refreshRequest.AddCookie(loginCookies[0])
	refreshResponse := httptest.NewRecorder()
	router.ServeHTTP(refreshResponse, refreshRequest)
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", refreshResponse.Code, refreshResponse.Body.String())
	}
	rotatedCookies := refreshResponse.Result().Cookies()
	if len(rotatedCookies) != 1 {
		t.Fatalf("rotated cookies = %#v", rotatedCookies)
	}
	reuseRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	reuseRequest.AddCookie(loginCookies[0])
	reuseResponse := httptest.NewRecorder()
	router.ServeHTTP(reuseResponse, reuseRequest)
	if reuseResponse.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh status = %d", reuseResponse.Code)
	}
	familyRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	familyRequest.AddCookie(rotatedCookies[0])
	familyResponse := httptest.NewRecorder()
	router.ServeHTTP(familyResponse, familyRequest)
	if familyResponse.Code != http.StatusUnauthorized {
		t.Fatalf("compromised family status = %d", familyResponse.Code)
	}
}

func TestMetrics(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	api := &API{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	api.managementHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.Len() == 0 {
		t.Fatal("metrics response is empty")
	}
}
