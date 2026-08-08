package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
)

type MonitorTarget struct{ ProductKey, ServiceKey string }
type UptimeInput struct {
	DeliveryID, MonitorName, MonitorURL, Message string
	Status                                       int
	LatencyMS                                    *int
	ObservedAt                                   time.Time
}

func (app *Service) ProductIDByKey(ctx context.Context, key string) *string {
	product, err := app.products.GetProductByKey(ctx, key)
	if err != nil {
		return nil
	}
	return &product.ID
}

func (app *Service) ListDeployments(ctx context.Context, limit int, cursor string) ([]domain.Deployment, string, error) {
	return app.operations.ListDeployments(ctx, normalizeLimit(limit), cursor)
}
func (app *Service) ListMarketQuotes(ctx context.Context, limit int, symbol string) ([]domain.MarketQuote, error) {
	return app.operations.ListMarketQuotes(ctx, normalizeLimit(limit), symbol)
}
func (app *Service) Weather(ctx context.Context, location string) (*domain.WeatherObservation, error) {
	if location == "" {
		location = "rio-cuarto"
	}
	return app.operations.WeatherByLocation(ctx, location)
}

func (app *Service) HandleUptime(ctx context.Context, target MonitorTarget, input UptimeInput, payload []byte) error {
	registered, err := app.registerDelivery(ctx, "uptime-kuma", input.DeliveryID, payload)
	if err != nil || !registered {
		return err
	}
	completed := false
	defer func() {
		if !completed {
			_ = app.operations.DeleteWebhookDelivery(ctx, "uptime-kuma", input.DeliveryID)
		}
	}()
	product, err := app.products.GetProductByKey(ctx, target.ProductKey)
	if err != nil {
		return err
	}
	service, err := app.services.GetServiceByProductAndKey(ctx, product.ID, target.ServiceKey)
	if err != nil {
		return err
	}
	if input.MonitorURL != "" && (service.Endpoint == nil || *service.Endpoint != input.MonitorURL) {
		endpoint := input.MonitorURL
		endpointPointer := &endpoint
		service, err = app.UpdateService(ctx, service.ID, ServicePatch{Endpoint: &endpointPointer})
		if err != nil {
			return err
		}
	}
	state := domain.ServiceOutage
	if input.Status == 1 {
		state = domain.ServiceOperational
	}
	if err := app.RecordServiceStatus(ctx, service.ID, state, input.LatencyMS, input.Message, input.ObservedAt); err != nil {
		return err
	}
	fingerprint := "uptime:" + product.Key + ":" + service.Key
	if state == domain.ServiceOutage {
		_, findErr := app.alerts.GetOpenAlertByFingerprint(ctx, fingerprint)
		if errors.Is(findErr, domain.ErrNotFound) {
			_, err = app.CreateAlert(ctx, AlertInput{ProductID: &product.ID, ServiceID: &service.ID, Fingerprint: fingerprint, Priority: domain.PriorityCritical, Title: service.Name + " caído", Message: bounded(input.Message, "Uptime Kuma reportó una caída")})
			if err != nil {
				return err
			}
		} else if findErr != nil {
			return findErr
		}
	} else {
		alert, findErr := app.alerts.GetOpenAlertByFingerprint(ctx, fingerprint)
		if findErr == nil {
			_, err = app.ResolveAlert(ctx, alert.ID)
			if err != nil {
				return err
			}
		} else if !errors.Is(findErr, domain.ErrNotFound) {
			return findErr
		}
	}
	err = app.publishGlobal(ctx, "service.status.changed", map[string]any{"product_key": product.Key, "service_key": service.Key, "status": state, "latency_ms": input.LatencyMS, "observed_at": input.ObservedAt})
	completed = err == nil
	return err
}

func (app *Service) HandleDeployment(ctx context.Context, value domain.Deployment, deliveryID string, payload []byte) error {
	registered, err := app.registerDelivery(ctx, value.Provider, deliveryID, payload)
	if err != nil || !registered {
		return err
	}
	completed := false
	defer func() {
		if !completed {
			_ = app.operations.DeleteWebhookDelivery(ctx, value.Provider, deliveryID)
		}
	}()
	if value.ID == "" {
		value.ID = app.ids.New()
	}
	if err := app.operations.SaveDeployment(ctx, value); err != nil {
		return err
	}
	err = app.publishGlobal(ctx, "deployment.updated", value)
	completed = err == nil
	return err
}

func (app *Service) registerDelivery(ctx context.Context, provider, deliveryID string, payload []byte) (bool, error) {
	if deliveryID == "" {
		sum := sha256.Sum256(payload)
		deliveryID = hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(payload)
	processed := time.Now().UTC()
	return app.operations.RegisterWebhookDelivery(ctx, domain.WebhookDelivery{ID: app.ids.New(), Provider: provider, DeliveryID: deliveryID, PayloadHash: hex.EncodeToString(sum[:]), Status: "processed", ReceivedAt: processed, ProcessedAt: &processed})
}

func (app *Service) publishGlobal(ctx context.Context, kind string, data any) error {
	now, eventID := time.Now().UTC(), app.ids.New()
	payload, err := json.Marshal(mqttEnvelope{SchemaVersion: 1, EventID: eventID, Type: kind, OccurredAt: now, CorrelationID: eventID, Data: data})
	if err != nil {
		return err
	}
	return app.operations.CreateOutbox(ctx, domain.OutboxEvent{ID: app.ids.New(), EventID: eventID, Topic: "desk/global/events", Payload: payload, NextAttemptAt: now, CreatedAt: now})
}

func bounded(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if len(value) > 160 {
		value = value[:160]
	}
	return value
}
