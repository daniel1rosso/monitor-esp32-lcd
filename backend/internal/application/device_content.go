package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
)

type DeviceContentConfig struct {
	Timezone, MQTTURL, Orientation                  string
	RefreshInterval, DashboardTTL                   time.Duration
	Width, Height, Brightness, MaxScreens, MaxBytes int
}

type Dashboard struct {
	SchemaVersion int               `json:"schema_version"`
	Revision      string            `json:"revision"`
	GeneratedAt   time.Time         `json:"generated_at"`
	ExpiresAt     time.Time         `json:"expires_at"`
	Screens       []DashboardScreen `json:"screens"`
}
type DashboardScreen struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Priority   int    `json:"priority"`
	DurationMS int    `json:"duration_ms"`
	Title      string `json:"title"`
	Payload    any    `json:"payload"`
}

type EffectiveDeviceConfig struct {
	SchemaVersion          int    `json:"schema_version"`
	Revision               string `json:"revision"`
	Timezone               string `json:"timezone"`
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
	Display                struct {
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Orientation string `json:"orientation"`
		Brightness  int    `json:"brightness_percent"`
	} `json:"display"`
	MQTT struct {
		URL              string            `json:"url"`
		KeepAliveSeconds int               `json:"keep_alive_seconds"`
		Topics           map[string]string `json:"topics"`
	} `json:"mqtt"`
	NotificationStyles map[domain.Priority]NotificationStyle `json:"notification_styles"`
}

func (app *Service) DeviceDashboard(ctx context.Context, deviceID string) (Dashboard, error) {
	device, err := app.devices.GetDeviceByID(ctx, deviceID)
	if err != nil {
		return Dashboard{}, err
	}
	if device.State == domain.DeviceDisabled || device.State == domain.DeviceRevoked {
		return Dashboard{}, domain.ErrForbidden
	}
	profile, err := app.profiles.GetProfileByID(ctx, device.ProfileID)
	if err != nil {
		return Dashboard{}, err
	}
	products, _, err := app.products.ListProducts(ctx, 100, "")
	if err != nil {
		return Dashboard{}, err
	}
	services, _, err := app.services.ListServices(ctx, 100, "", "")
	if err != nil {
		return Dashboard{}, err
	}
	alerts, _, err := app.alerts.ListAlerts(ctx, 100, "", ports.AlertFilter{State: string(domain.AlertOpen)})
	if err != nil {
		return Dashboard{}, err
	}
	var deployment *domain.Deployment
	var quotes []domain.MarketQuote
	var weather *domain.WeatherObservation
	if app.snapshots != nil {
		deployment, err = app.snapshots.LatestDeployment(ctx)
		if err != nil {
			return Dashboard{}, err
		}
		quotes, err = app.snapshots.LatestMarketQuotes(ctx, 10)
		if err != nil {
			return Dashboard{}, err
		}
		weather, err = app.snapshots.LatestWeather(ctx)
		if err != nil {
			return Dashboard{}, err
		}
	}
	screens := make([]DashboardScreen, 0, len(profile.Screens))
	for _, screen := range profile.Screens {
		if !screen.Enabled {
			continue
		}
		value := app.buildScreen(screen, products, services, alerts, deployment, quotes, weather)
		screens = append(screens, value)
		if len(screens) >= app.content.MaxScreens {
			break
		}
	}
	sort.SliceStable(screens, func(i, j int) bool { return screens[i].Priority > screens[j].Priority })
	revision := contentRevision(profile.Revision, screens)
	now := time.Now().UTC()
	result := Dashboard{SchemaVersion: 1, Revision: revision, GeneratedAt: now, ExpiresAt: now.Add(app.content.DashboardTTL), Screens: screens}
	for len(result.Screens) > 1 {
		raw, _ := json.Marshal(result)
		if len(raw) <= app.content.MaxBytes {
			break
		}
		result.Screens = result.Screens[:len(result.Screens)-1]
		result.Revision = contentRevision(profile.Revision, result.Screens)
	}
	return result, nil
}

func (app *Service) buildScreen(screen domain.Screen, products []domain.Product, services []domain.Service, alerts []domain.Alert, deployment *domain.Deployment, quotes []domain.MarketQuote, weather *domain.WeatherObservation) DashboardScreen {
	result := DashboardScreen{ID: screen.ID, Type: screen.Type, Priority: screen.Priority, DurationMS: screen.DurationMS, Title: screenTitle(screen.Type), Payload: map[string]any{}}
	config := map[string]any{}
	_ = json.Unmarshal(screen.ConfigJSON, &config)
	switch screen.Type {
	case "overview":
		state := overallState(services)
		result.Payload = map[string]any{"status": state, "products": len(products), "services": len(services), "open_alerts": len(alerts), "subtitle": "Estado de la plataforma"}
	case "product_status":
		product := selectProduct(products, stringConfig(config, "product_key"))
		related := servicesFor(services, product.ID)
		up := countOperational(related)
		result.Title = product.Name
		result.Payload = map[string]any{"product_key": product.Key, "name": product.Name, "status": overallState(related), "services_up": up, "services_total": len(related), "detail": fmt.Sprintf("%d de %d servicios operativos", up, len(related))}
	case "service_status":
		service := selectService(services, stringConfig(config, "service_key"))
		result.Title = service.Name
		result.Payload = map[string]any{"name": service.Name, "status": defaultState(service.Status), "latency_ms": service.LatencyMS, "last_change_at": nil, "detail": "Última comprobación disponible"}
	case "alert":
		alert := selectAlert(alerts)
		style := app.styles[alert.Priority]
		result.Title = alert.Title
		result.Payload = map[string]any{"alert_id": alert.ID, "severity": alert.Priority, "message": alert.Message, "source": "Desk Monitor", "style": style}
	case "deployment":
		if deployment == nil {
			result.Payload = map[string]any{"repository": "Sin despliegues", "branch": "-", "workflow": "-", "status": "queued", "commit_short": "-", "author": "-", "occurred_at": time.Now().UTC()}
		} else {
			result.Payload = map[string]any{"repository": deployment.Repository, "branch": deployment.Branch, "workflow": deployment.Workflow, "status": deployment.Status, "commit_short": short(deployment.CommitSHA, 12), "author": deployment.Author, "occurred_at": deployment.StartedAt}
		}
	case "market":
		quote := selectQuote(quotes, stringConfig(config, "symbol"))
		result.Title = quote.Symbol
		result.Payload = map[string]any{"symbol": quote.Symbol, "display_value": fmt.Sprintf("%.2f", quote.Last), "bid": numberString(quote.Bid), "ask": numberString(quote.Ask), "change_percent": quote.ChangePercent, "trend": trend(quote.ChangePercent), "stale": quote.ObservedAt.IsZero() || time.Since(quote.ObservedAt) > 30*time.Minute}
	case "weather":
		if weather == nil {
			result.Payload = map[string]any{"location": "Sin datos", "temperature_c": 0, "apparent_temperature_c": 0, "condition_icon": "cloud-off", "daily_min_c": 0, "daily_max_c": 0, "precipitation_probability_percent": 0}
		} else {
			result.Title = "Clima " + weather.LocationName
			result.Payload = map[string]any{"location": weather.LocationName, "temperature_c": weather.TemperatureC, "apparent_temperature_c": weather.ApparentTemperatureC, "condition_icon": weatherIcon(weather.WeatherCode), "daily_min_c": floatValue(weather.DailyMinC), "daily_max_c": floatValue(weather.DailyMaxC), "precipitation_probability_percent": int(floatValue(weather.PrecipitationProbability))}
		}
	case "metric_list":
		title := stringConfig(config, "title")
		items := []any{}
		if stringConfig(config, "source") == "markets" {
			if title == "" {
				title = "Mercado"
			}
			for _, symbol := range []string{"BTCUSDT", "XRPUSDT", "USD_BLUE"} {
				quote := selectQuote(quotes, symbol)
				items = append(items, map[string]any{"label": marketLabel(symbol), "value": fmt.Sprintf("%.2f", quote.Last), "trend": trend(quote.ChangePercent), "color": trendColor(quote.ChangePercent)})
			}
		} else {
			if title == "" {
				title = "Servicios"
			}
			productNames := map[string]string{}
			for _, product := range products {
				productNames[product.ID] = product.Name
			}
			offset := intConfig(config, "offset")
			for i, service := range services {
				if i < offset {
					continue
				}
				if len(items) >= 5 {
					break
				}
				label := productNames[service.ProductID]
				if label == "" {
					label = service.Name
				}
				state := defaultState(service.Status)
				items = append(items, map[string]any{"label": label, "value": stateLabel(state), "color": stateColor(state)})
			}
		}
		result.Title = title
		result.Payload = map[string]any{"items": items}
	case "message":
		result.Payload = map[string]any{"body": stringConfig(config, "body"), "icon": stringConfig(config, "icon")}
	}
	return result
}

func (app *Service) DeviceConfig(ctx context.Context, id string) (EffectiveDeviceConfig, error) {
	device, err := app.devices.GetDeviceByID(ctx, id)
	if err != nil {
		return EffectiveDeviceConfig{}, err
	}
	profile, err := app.profiles.GetProfileByID(ctx, device.ProfileID)
	if err != nil {
		return EffectiveDeviceConfig{}, err
	}
	c := EffectiveDeviceConfig{SchemaVersion: 1, Timezone: app.content.Timezone, RefreshIntervalSeconds: int(app.content.RefreshInterval.Seconds()), NotificationStyles: app.styles}
	c.Display.Width = app.content.Width
	c.Display.Height = app.content.Height
	c.Display.Orientation = app.content.Orientation
	c.Display.Brightness = app.content.Brightness
	c.MQTT.URL = app.content.MQTTURL
	c.MQTT.KeepAliveSeconds = 60
	prefix := "desk/device/" + device.DeviceID + "/"
	c.MQTT.Topics = map[string]string{"alerts": prefix + "alerts", "notifications": prefix + "notifications", "commands": prefix + "commands", "status": prefix + "status", "telemetry": prefix + "telemetry", "acks": prefix + "acks", "global_events": "desk/global/events"}
	c.Revision = contentRevision(profile.Revision, struct {
		Device    string
		Overrides string
		Config    any
	}{device.DeviceID, string(device.OverridesJSON), c})
	return c, nil
}

func contentRevision(profile int, value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(append([]byte(fmt.Sprint(profile)+":"), raw...))
	return hex.EncodeToString(sum[:12])
}
func screenTitle(kind string) string {
	return map[string]string{"overview": "Resumen", "product_status": "Producto", "service_status": "Servicio", "alert": "Alerta", "deployment": "Deploy", "market": "Mercados", "weather": "Clima", "metric_list": "Métricas", "message": "Mensaje"}[kind]
}
func overallState(items []domain.Service) domain.ServiceState {
	state := domain.ServiceOperational
	if len(items) == 0 {
		return domain.ServiceUnknown
	}
	for _, v := range items {
		if v.Status == domain.ServiceOutage {
			return domain.ServiceOutage
		}
		if v.Status == domain.ServiceDegraded || v.Status == domain.ServiceUnknown {
			state = domain.ServiceDegraded
		}
	}
	return state
}
func defaultState(v domain.ServiceState) domain.ServiceState {
	if v == "" {
		return domain.ServiceUnknown
	}
	return v
}
func selectProduct(v []domain.Product, key string) domain.Product {
	for _, x := range v {
		if x.Key == key {
			return x
		}
	}
	if len(v) > 0 {
		return v[0]
	}
	return domain.Product{Key: "none", Name: "Sin productos", Health: domain.ServiceUnknown}
}
func selectService(v []domain.Service, key string) domain.Service {
	for _, x := range v {
		if x.Key == key {
			return x
		}
	}
	if len(v) > 0 {
		return v[0]
	}
	return domain.Service{Name: "Sin servicios", Status: domain.ServiceUnknown}
}
func selectAlert(v []domain.Alert) domain.Alert {
	if len(v) > 0 {
		return v[0]
	}
	return domain.Alert{ID: "none", Priority: domain.PriorityInfo, Title: "Sin alertas", Message: "Todo está funcionando correctamente"}
}
func selectQuote(v []domain.MarketQuote, symbol string) domain.MarketQuote {
	for _, x := range v {
		if strings.EqualFold(x.Symbol, symbol) {
			return x
		}
	}
	if len(v) > 0 {
		return v[0]
	}
	return domain.MarketQuote{Symbol: "N/D"}
}
func servicesFor(v []domain.Service, id string) []domain.Service {
	r := []domain.Service{}
	for _, x := range v {
		if x.ProductID == id {
			r = append(r, x)
		}
	}
	return r
}
func countOperational(v []domain.Service) int {
	n := 0
	for _, x := range v {
		if x.Status == domain.ServiceOperational {
			n++
		}
	}
	return n
}
func stringConfig(v map[string]any, key string) string { s, _ := v[key].(string); return s }
func short(v string, n int) string {
	if len(v) <= n {
		return v
	}
	return v[:n]
}
func numberString(v *float64) any {
	if v == nil {
		return nil
	}
	return fmt.Sprintf("%.2f", *v)
}
func trend(v float64) string {
	if v > 0 {
		return "up"
	}
	if v < 0 {
		return "down"
	}
	return "flat"
}
func floatValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
func marketLabel(symbol string) string {
	switch symbol {
	case "BTCUSDT":
		return "BTC"
	case "XRPUSDT":
		return "XRP"
	case "USD_BLUE":
		return "Dólar blue"
	}
	return symbol
}
func trendColor(v float64) string {
	if v > 0 {
		return "#30D158"
	}
	if v < 0 {
		return "#FF3B30"
	}
	return "#8E8E93"
}
func intConfig(v map[string]any, key string) int {
	if n, ok := v[key].(float64); ok {
		return int(n)
	}
	return 0
}
func stateLabel(v domain.ServiceState) string {
	switch v {
	case domain.ServiceOperational:
		return "Operativo"
	case domain.ServiceDegraded:
		return "Degradado"
	case domain.ServiceOutage:
		return "Caído"
	default:
		return "Desconocido"
	}
}
func stateColor(v domain.ServiceState) string {
	switch v {
	case domain.ServiceOperational:
		return "#30D158"
	case domain.ServiceDegraded:
		return "#FFCC00"
	case domain.ServiceOutage:
		return "#FF3B30"
	default:
		return "#8E8E93"
	}
}
func weatherIcon(code int) string {
	if code == 0 {
		return "sun"
	}
	if code < 4 {
		return "cloud-sun"
	}
	if code < 70 {
		return "cloud-rain"
	}
	return "cloud"
}
