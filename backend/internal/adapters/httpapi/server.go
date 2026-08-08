// Package httpapi owns the Gin HTTP transport boundary.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/application"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/buildinfo"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
	"github.com/gin-gonic/gin"
)

const apiVersion = "v1"
const refreshCookieName = "desk_refresh"

type Config struct {
	PublicAddress, ManagementAddress                        string
	ReadTimeout, WriteTimeout, IdleTimeout, ShutdownTimeout time.Duration
	AllowedOrigins                                          []string
	PublicURL                                               string
}

type Dependencies struct {
	Application  *application.Service
	Health       ports.HealthChecker
	Logger       *slog.Logger
	Build        buildinfo.Info
	Integrations IntegrationConfig
}

type IntegrationConfig struct {
	GitHubWebhookSecret string
	GitHubActionsToken  string
	UptimeToken         string
	UptimeMonitors      map[string]application.MonitorTarget
}

type API struct {
	app          *application.Service
	health       ports.HealthChecker
	logger       *slog.Logger
	build        buildinfo.Info
	requests     atomic.Uint64
	integrations IntegrationConfig
}

type healthResponse struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}
type readinessResponse struct {
	Status       string                      `json:"status"`
	CheckedAt    time.Time                   `json:"checked_at"`
	Dependencies map[string]dependencyHealth `json:"dependencies"`
}
type dependencyHealth struct {
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Detail    string `json:"detail,omitempty"`
}
type versionResponse struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuiltAt    string `json:"built_at"`
	APIVersion string `json:"api_version"`
}
type problem struct {
	Type      string              `json:"type"`
	Title     string              `json:"title"`
	Status    int                 `json:"status"`
	Detail    string              `json:"detail,omitempty"`
	Instance  string              `json:"instance,omitempty"`
	RequestID string              `json:"request_id"`
	Errors    map[string][]string `json:"errors,omitempty"`
}

func ConfigFromEnvironment() Config {
	return Config{PublicAddress: envOrDefault("DESK_HTTP_ADDRESS", ":8080"), ManagementAddress: envOrDefault("DESK_MANAGEMENT_ADDRESS", ":9090"), ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, ShutdownTimeout: 10 * time.Second}
}

func Run(ctx context.Context, config Config, dependencies Dependencies) error {
	api := &API{app: dependencies.Application, health: dependencies.Health, logger: dependencies.Logger, build: dependencies.Build, integrations: dependencies.Integrations}
	publicServer := newServer(config.PublicAddress, api.publicHandler(config), config)
	managementServer := newServer(config.ManagementAddress, api.managementHandler(), config)
	errorsChannel := make(chan error, 2)
	var servers sync.WaitGroup
	for _, server := range []*http.Server{publicServer, managementServer} {
		servers.Add(1)
		go func(server *http.Server) {
			defer servers.Done()
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errorsChannel <- err
			}
		}(server)
	}
	var runError error
	select {
	case <-ctx.Done():
	case runError = <-errorsChannel:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()
	for _, server := range []*http.Server{publicServer, managementServer} {
		if err := server.Shutdown(shutdown); err != nil && runError == nil {
			runError = fmt.Errorf("shutdown %s: %w", server.Addr, err)
		}
	}
	servers.Wait()
	return runError
}

func Probe(ctx context.Context, endpoint string) error {
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(probe, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", response.Status)
	}
	return nil
}

func (api *API) publicHandler(config Config) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(api.requestID(), api.recovery(), api.logging(), api.cors(config.AllowedOrigins))
	v1 := router.Group("/api/v1")
	v1.GET("/health", api.healthPublic)
	v1.GET("/version", api.version)
	v1.POST("/auth/login", api.login(config.PublicURL))
	v1.POST("/auth/refresh", api.refresh(config.PublicURL))
	v1.POST("/auth/logout", api.authRequired(application.UserAudience), api.logout(config.PublicURL))
	v1.GET("/auth/me", api.authRequired(application.UserAudience), api.me)
	v1.POST("/device/auth/token", api.deviceToken(config.PublicURL))
	v1.POST("/events/github", api.githubWebhook)
	v1.POST("/events/github/actions", api.githubActions)
	v1.POST("/events/uptime", api.uptimeEvent)
	device := v1.Group("/device", api.authRequired(application.DeviceAudience))
	device.GET("/dashboard", api.deviceDashboard)
	device.GET("/config", api.deviceConfig)
	products := v1.Group("/products", api.authRequired(application.UserAudience))
	products.GET("", api.listProducts)
	products.GET("/:productId", api.getProduct)
	products.POST("", api.adminRequired(), api.createProduct)
	products.PATCH("/:productId", api.adminRequired(), api.updateProduct)
	products.DELETE("/:productId", api.adminRequired(), api.archiveProduct)
	services := v1.Group("/services", api.authRequired(application.UserAudience))
	services.GET("", api.listServices)
	services.GET("/:serviceId", api.getService)
	services.POST("", api.adminRequired(), api.createService)
	services.PATCH("/:serviceId", api.adminRequired(), api.updateService)
	services.DELETE("/:serviceId", api.adminRequired(), api.archiveService)
	alerts := v1.Group("/alerts", api.authRequired(application.UserAudience))
	alerts.GET("", api.listAlerts)
	alerts.POST("", api.adminRequired(), api.createAlert)
	alerts.POST("/:alertId/acknowledge", api.adminRequired(), api.acknowledgeAlert)
	alerts.POST("/:alertId/resolve", api.adminRequired(), api.resolveAlert)
	deployments := v1.Group("/deployments", api.authRequired(application.UserAudience))
	deployments.GET("", api.listDeployments)
	markets := v1.Group("/markets", api.authRequired(application.UserAudience))
	markets.GET("/quotes", api.listMarketQuotes)
	v1.GET("/weather", api.authRequired(application.UserAudience), api.getWeather)
	devices := v1.Group("/devices", api.authRequired(application.UserAudience), api.adminRequired())
	devices.GET("", api.listDevices)
	devices.POST("", api.createDevice)
	devices.GET("/:deviceId", api.getDevice)
	devices.PATCH("/:deviceId", api.updateDevice)
	devices.POST("/:deviceId/revoke", api.revokeDevice)
	devices.POST("/:deviceId/rotate-credentials", api.rotateDevice)
	profiles := v1.Group("/profiles", api.authRequired(application.UserAudience))
	profiles.GET("", api.listProfiles)
	profiles.GET("/:profileId", api.getProfile)
	profiles.POST("", api.adminRequired(), api.createProfile)
	profiles.PUT("/:profileId", api.adminRequired(), api.replaceProfile)
	profiles.DELETE("/:profileId", api.adminRequired(), api.deleteProfile)
	router.NoRoute(func(c *gin.Context) { api.writeProblem(c, domain.ErrNotFound) })
	return router
}

func (api *API) managementHandler() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(api.requestID(), api.recovery())
	router.GET("/api/v1/health/live", func(c *gin.Context) { c.JSON(http.StatusOK, healthResponse{Status: "ok", CheckedAt: time.Now().UTC()}) })
	router.GET("/api/v1/health/ready", api.readiness)
	router.GET("/metrics", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		c.String(http.StatusOK, "# HELP desk_monitor_up Whether the backend process is running.\n# TYPE desk_monitor_up gauge\ndesk_monitor_up 1\n# HELP desk_monitor_http_requests_total Total HTTP requests.\n# TYPE desk_monitor_http_requests_total counter\ndesk_monitor_http_requests_total %d\n", api.requests.Load())
	})
	return router
}

func newServer(address string, handler http.Handler, config Config) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: config.ReadTimeout, ReadTimeout: config.ReadTimeout, WriteTimeout: config.WriteTimeout, IdleTimeout: config.IdleTimeout}
}

func (api *API) healthPublic(c *gin.Context) {
	status := "ok"
	code := http.StatusOK
	if api.health != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		if api.health.Ping(ctx) != nil {
			status = "unavailable"
			code = http.StatusServiceUnavailable
		}
	}
	c.JSON(code, healthResponse{Status: status, CheckedAt: time.Now().UTC()})
}
func (api *API) readiness(c *gin.Context) {
	started := time.Now()
	status := "ok"
	detail := dependencyHealth{Status: "ok"}
	code := http.StatusOK
	if api.health != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := api.health.Ping(ctx); err != nil {
			status = "unavailable"
			detail.Status = status
			detail.Detail = "database unavailable"
			code = http.StatusServiceUnavailable
		}
	}
	detail.LatencyMS = time.Since(started).Milliseconds()
	c.JSON(code, readinessResponse{Status: status, CheckedAt: time.Now().UTC(), Dependencies: map[string]dependencyHealth{"database": detail}})
}
func (api *API) version(c *gin.Context) {
	c.JSON(http.StatusOK, versionResponse{Version: api.build.Version, Commit: api.build.Commit, BuiltAt: api.build.BuiltAt, APIVersion: apiVersion})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (api *API) login(publicURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request loginRequest
		if !api.decode(c, &request, 4<<10) || request.Email == "" || len(request.Password) < 8 {
			api.writeProblem(c, domain.ErrInvalid)
			return
		}
		result, err := api.app.Login(c.Request.Context(), request.Email, request.Password)
		if err != nil {
			api.writeProblem(c, err)
			return
		}
		setRefreshCookie(c, result.RefreshToken, publicURL, 7*24*time.Hour)
		api.writeAccessToken(c, result)
	}
}

func (api *API) refresh(publicURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current, err := c.Cookie(refreshCookieName)
		if err != nil {
			api.writeProblem(c, domain.ErrUnauthorized)
			return
		}
		result, err := api.app.Refresh(c.Request.Context(), current)
		if err != nil {
			clearRefreshCookie(c, publicURL)
			api.writeProblem(c, err)
			return
		}
		setRefreshCookie(c, result.RefreshToken, publicURL, 7*24*time.Hour)
		api.writeAccessToken(c, result)
	}
}

func (api *API) logout(publicURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current, _ := c.Cookie(refreshCookieName)
		if err := api.app.Logout(c.Request.Context(), current); err != nil {
			api.writeProblem(c, err)
			return
		}
		clearRefreshCookie(c, publicURL)
		c.Status(http.StatusNoContent)
	}
}

func (api *API) writeAccessToken(c *gin.Context, result application.AccessToken) {
	c.JSON(http.StatusOK, gin.H{"access_token": result.Value, "token_type": "Bearer", "expires_in": 900, "user": userDTO(*result.User)})
}
func (api *API) me(c *gin.Context) {
	claims := claimsFrom(c)
	user, err := api.app.CurrentUser(c.Request.Context(), claims.Subject)
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, userDTO(user))
}

type deviceTokenRequest struct {
	DeviceID        string `json:"device_id"`
	Secret          string `json:"secret"`
	FirmwareVersion string `json:"firmware_version"`
}

func (api *API) deviceToken(publicURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request deviceTokenRequest
		if !api.decode(c, &request, 4<<10) || request.DeviceID == "" || len(request.Secret) < 32 {
			api.writeProblem(c, domain.ErrInvalid)
			return
		}
		result, err := api.app.ExchangeDeviceSecret(c.Request.Context(), request.DeviceID, request.Secret, request.FirmwareVersion)
		if err != nil {
			api.writeProblem(c, err)
			return
		}
		mqttURL := strings.Replace(publicURL, "http://", "ws://", 1)
		mqttURL = strings.Replace(mqttURL, "https://", "wss://", 1)
		mqttURL = strings.TrimSuffix(mqttURL, "/") + "/mqtt"
		c.JSON(http.StatusOK, gin.H{"access_token": result.Value, "token_type": "Bearer", "expires_in": int(time.Until(result.ExpiresAt).Seconds()), "mqtt": gin.H{"url": mqttURL, "client_id": request.DeviceID, "username": result.MQTTUsername, "password": nil}})
	}
}

func (api *API) deviceDashboard(c *gin.Context) {
	value, err := api.app.DeviceDashboard(c.Request.Context(), claimsFrom(c).Subject)
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	api.writeRevisioned(c, value.Revision, value)
}

func (api *API) deviceConfig(c *gin.Context) {
	value, err := api.app.DeviceConfig(c.Request.Context(), claimsFrom(c).Subject)
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	api.writeRevisioned(c, value.Revision, value)
}

func (api *API) writeRevisioned(c *gin.Context, revision string, value any) {
	etag := `"` + revision + `"`
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, no-cache")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.JSON(http.StatusOK, value)
}

type productWrite struct {
	Key         *string `json:"key"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
}

func (api *API) createProduct(c *gin.Context) {
	var request productWrite
	if !api.decode(c, &request, 16<<10) || request.Key == nil || request.Name == nil {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	description := ""
	if request.Description != nil {
		description = *request.Description
	}
	product, err := api.app.CreateProduct(c.Request.Context(), application.ProductInput{Key: *request.Key, Name: *request.Name, Description: description, Enabled: enabled})
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusCreated, productDTO(product))
}
func (api *API) getProduct(c *gin.Context) {
	product, err := api.app.GetProduct(c.Request.Context(), c.Param("productId"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, productDTO(product))
}
func (api *API) listProducts(c *gin.Context) {
	items, next, err := api.app.ListProducts(c.Request.Context(), queryLimit(c), c.Query("cursor"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	values := make([]gin.H, len(items))
	for index, item := range items {
		values[index] = productDTO(item)
	}
	var cursor any
	if next != "" {
		cursor = next
	}
	c.JSON(http.StatusOK, gin.H{"items": values, "next_cursor": cursor, "has_more": next != ""})
}
func (api *API) updateProduct(c *gin.Context) {
	var request productWrite
	if !api.decode(c, &request, 16<<10) || request.Key != nil {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	product, err := api.app.UpdateProduct(c.Request.Context(), c.Param("productId"), application.ProductPatch{Name: request.Name, Description: request.Description, Enabled: request.Enabled})
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, productDTO(product))
}
func (api *API) archiveProduct(c *gin.Context) {
	if err := api.app.ArchiveProduct(c.Request.Context(), c.Param("productId")); err != nil {
		api.writeProblem(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type deviceCreate struct {
	DeviceID  string `json:"device_id"`
	Name      string `json:"name"`
	ProfileID string `json:"profile_id"`
}

func (api *API) createDevice(c *gin.Context) {
	var request deviceCreate
	if !api.decode(c, &request, 8<<10) {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	result, err := api.app.CreateDevice(c.Request.Context(), application.DeviceProvision{DeviceID: request.DeviceID, Name: request.Name, ProfileID: request.ProfileID})
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"device": deviceDTO(result.Device), "device_secret": result.DeviceSecret, "mqtt_username": result.MQTTUsername, "mqtt_password": result.MQTTPassword})
}
func (api *API) getDevice(c *gin.Context) {
	value, err := api.app.GetDevice(c.Request.Context(), c.Param("deviceId"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, deviceDTO(value))
}
func (api *API) listDevices(c *gin.Context) {
	items, next, err := api.app.ListDevices(c.Request.Context(), queryLimit(c), c.Query("cursor"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	values := make([]gin.H, len(items))
	for index, item := range items {
		values[index] = deviceDTO(item)
	}
	var cursor any
	if next != "" {
		cursor = next
	}
	c.JSON(http.StatusOK, gin.H{"items": values, "next_cursor": cursor, "has_more": next != ""})
}
func (api *API) revokeDevice(c *gin.Context) {
	if err := api.app.RevokeDevice(c.Request.Context(), c.Param("deviceId")); err != nil {
		api.writeProblem(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (api *API) rotateDevice(c *gin.Context) {
	result, err := api.app.RotateDeviceCredentials(c.Request.Context(), c.Param("deviceId"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"device_secret": result.DeviceSecret, "mqtt_username": result.MQTTUsername, "mqtt_password": result.MQTTPassword})
}

type deviceUpdate struct {
	Name      *string             `json:"name"`
	ProfileID *string             `json:"profile_id"`
	State     *domain.DeviceState `json:"state"`
	Overrides map[string]any      `json:"overrides"`
}

func (api *API) updateDevice(c *gin.Context) {
	var request deviceUpdate
	if !api.decode(c, &request, 32<<10) {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	var raw []byte
	hasOverrides := request.Overrides != nil
	if hasOverrides {
		raw, _ = json.Marshal(request.Overrides)
	}
	device, err := api.app.UpdateDevice(c.Request.Context(), c.Param("deviceId"), application.DevicePatch{Name: request.Name, ProfileID: request.ProfileID, State: request.State, OverridesJSON: raw, HasOverrides: hasOverrides})
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, deviceDTO(device))
}

type profileScreenWrite struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Priority   int            `json:"priority"`
	DurationMS int            `json:"duration_ms"`
	Enabled    bool           `json:"enabled"`
	Config     map[string]any `json:"config"`
}
type profileWrite struct {
	Key     string               `json:"key"`
	Name    string               `json:"name"`
	Screens []profileScreenWrite `json:"screens"`
}

func profileInput(request profileWrite) (application.ProfileInput, error) {
	screens := make([]domain.Screen, len(request.Screens))
	for index, item := range request.Screens {
		raw, err := json.Marshal(item.Config)
		if err != nil {
			return application.ProfileInput{}, err
		}
		screens[index] = domain.Screen{ID: item.ID, Type: item.Type, Priority: item.Priority, DurationMS: item.DurationMS, Enabled: item.Enabled, ConfigJSON: raw}
	}
	return application.ProfileInput{Key: request.Key, Name: request.Name, Screens: screens}, nil
}
func (api *API) createProfile(c *gin.Context) {
	var request profileWrite
	if !api.decode(c, &request, 64<<10) {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	input, err := profileInput(request)
	if err != nil {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	profile, err := api.app.CreateProfile(c.Request.Context(), input)
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusCreated, profileDTO(profile))
}
func (api *API) getProfile(c *gin.Context) {
	profile, err := api.app.GetProfile(c.Request.Context(), c.Param("profileId"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, profileDTO(profile))
}
func (api *API) listProfiles(c *gin.Context) {
	items, next, err := api.app.ListProfiles(c.Request.Context(), queryLimit(c), c.Query("cursor"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	values := make([]gin.H, len(items))
	for index, item := range items {
		values[index] = profileDTO(item)
	}
	var cursor any
	if next != "" {
		cursor = next
	}
	c.JSON(http.StatusOK, gin.H{"items": values, "next_cursor": cursor, "has_more": next != ""})
}
func (api *API) replaceProfile(c *gin.Context) {
	var request profileWrite
	if !api.decode(c, &request, 64<<10) {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	input, err := profileInput(request)
	if err != nil {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	profile, err := api.app.ReplaceProfile(c.Request.Context(), c.Param("profileId"), input)
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, profileDTO(profile))
}
func (api *API) deleteProfile(c *gin.Context) {
	if err := api.app.DeleteProfile(c.Request.Context(), c.Param("profileId")); err != nil {
		api.writeProblem(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *API) authRequired(audience string) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			api.writeProblem(c, domain.ErrUnauthorized)
			c.Abort()
			return
		}
		claims, err := api.app.Authenticate(parts[1], audience)
		if err != nil {
			api.writeProblem(c, err)
			c.Abort()
			return
		}
		c.Set("claims", claims)
		c.Next()
	}
}
func (api *API) adminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if claimsFrom(c).Role != "admin" {
			api.writeProblem(c, domain.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}
func claimsFrom(c *gin.Context) ports.AccessClaims {
	value, _ := c.Get("claims")
	claims, _ := value.(ports.AccessClaims)
	return claims
}

func (api *API) requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" || len(id) > 128 {
			id = randomID()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}
func (api *API) recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				api.logger.Error("panic recovered", "request_id", requestID(c), "panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
				api.writeStatusProblem(c, http.StatusInternalServerError, "Internal Server Error", "unexpected server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
func (api *API) logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		api.requests.Add(1)
		api.logger.Info("http request", "request_id", requestID(c), "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
	}
}
func (api *API) cors(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (api *API) decode(c *gin.Context, target any, maximum int64) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximum)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func (api *API) writeProblem(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		api.writeStatusProblem(c, http.StatusBadRequest, "Bad Request", "request validation failed")
	case errors.Is(err, domain.ErrUnauthorized):
		api.writeStatusProblem(c, http.StatusUnauthorized, "Unauthorized", "authentication failed")
	case errors.Is(err, domain.ErrForbidden):
		api.writeStatusProblem(c, http.StatusForbidden, "Forbidden", "insufficient permissions")
	case errors.Is(err, domain.ErrNotFound):
		api.writeStatusProblem(c, http.StatusNotFound, "Not Found", "resource does not exist")
	case errors.Is(err, domain.ErrConflict):
		api.writeStatusProblem(c, http.StatusConflict, "Conflict", "resource already exists")
	default:
		api.logger.Error("request failed", "request_id", requestID(c), "error", err)
		api.writeStatusProblem(c, http.StatusInternalServerError, "Internal Server Error", "unexpected server error")
	}
}
func (api *API) writeStatusProblem(c *gin.Context, status int, title, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, problem{Type: "https://desk-monitor.local/problems/" + strconv.Itoa(status), Title: title, Status: status, Detail: detail, Instance: c.Request.URL.Path, RequestID: requestID(c)})
}

func productDTO(value domain.Product) gin.H {
	return gin.H{"id": value.ID, "key": value.Key, "name": value.Name, "description": value.Description, "enabled": value.Enabled, "health": value.Health, "created_at": value.CreatedAt, "updated_at": value.UpdatedAt}
}
func deviceDTO(value domain.Device) gin.H {
	return gin.H{"id": value.ID, "device_id": value.DeviceID, "name": value.Name, "profile_id": value.ProfileID, "state": value.State, "firmware_version": value.FirmwareVersion, "last_seen_at": value.LastSeenAt, "created_at": value.CreatedAt}
}
func profileDTO(value domain.ScreenProfile) gin.H {
	screens := make([]gin.H, len(value.Screens))
	for index, item := range value.Screens {
		configValue := map[string]any{}
		if len(item.ConfigJSON) > 0 {
			_ = json.Unmarshal(item.ConfigJSON, &configValue)
		}
		screens[index] = gin.H{"id": item.ID, "type": item.Type, "priority": item.Priority, "duration_ms": item.DurationMS, "enabled": item.Enabled, "config": configValue}
	}
	return gin.H{"id": value.ID, "key": value.Key, "name": value.Name, "revision": value.Revision, "screens": screens}
}
func userDTO(value domain.User) gin.H {
	return gin.H{"id": value.ID, "email": value.Email, "role": value.Role, "created_at": value.CreatedAt}
}
func setRefreshCookie(c *gin.Context, value, publicURL string, ttl time.Duration) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, value, int(ttl.Seconds()), "/api/v1/auth", "", strings.HasPrefix(publicURL, "https://"), true)
}
func clearRefreshCookie(c *gin.Context, publicURL string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1, "/api/v1/auth", "", strings.HasPrefix(publicURL, "https://"), true)
}
func queryLimit(c *gin.Context) int {
	value, err := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if err != nil {
		return 25
	}
	return value
}
func requestID(c *gin.Context) string {
	value, _ := c.Get("request_id")
	id, _ := value.(string)
	return id
}
func randomID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(value)
}
func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
