package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
)

type ServiceInput struct {
	ProductID, Key, Name, Kind string
	Endpoint                   *string
	Enabled                    bool
}

func (app *Service) CreateService(ctx context.Context, input ServiceInput) (domain.Service, error) {
	if _, err := app.products.GetProduct(ctx, input.ProductID); err != nil {
		return domain.Service{}, err
	}
	value, err := domain.NewService(app.ids.New(), input.ProductID, input.Key, input.Name, input.Kind, input.Endpoint, input.Enabled, time.Now())
	if err != nil {
		return domain.Service{}, err
	}
	if err := app.services.CreateService(ctx, value); err != nil {
		return domain.Service{}, err
	}
	return value, nil
}
func (app *Service) GetService(ctx context.Context, id string) (domain.Service, error) {
	return app.services.GetService(ctx, id)
}
func (app *Service) ListServices(ctx context.Context, limit int, cursor, productID string) ([]domain.Service, string, error) {
	return app.services.ListServices(ctx, normalizeLimit(limit), cursor, productID)
}

type ServicePatch struct {
	Name, Kind *string
	Endpoint   **string
	Enabled    *bool
}

func (app *Service) UpdateService(ctx context.Context, id string, patch ServicePatch) (domain.Service, error) {
	current, err := app.services.GetService(ctx, id)
	if err != nil {
		return domain.Service{}, err
	}
	if patch.Name != nil {
		current.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.Kind != nil {
		current.Kind = *patch.Kind
	}
	if patch.Endpoint != nil {
		current.Endpoint = *patch.Endpoint
	}
	if patch.Enabled != nil {
		current.Enabled = *patch.Enabled
	}
	validated, err := domain.NewService(current.ID, current.ProductID, current.Key, current.Name, current.Kind, current.Endpoint, current.Enabled, current.CreatedAt)
	if err != nil {
		return domain.Service{}, err
	}
	validated.CreatedAt = current.CreatedAt
	validated.UpdatedAt = time.Now().UTC()
	validated.Status = current.Status
	validated.LatencyMS = current.LatencyMS
	if err := app.services.UpdateService(ctx, validated); err != nil {
		return domain.Service{}, err
	}
	return validated, nil
}
func (app *Service) ArchiveService(ctx context.Context, id string) error {
	return app.services.ArchiveService(ctx, id, time.Now().UTC())
}
func (app *Service) RecordServiceStatus(ctx context.Context, serviceID string, state domain.ServiceState, latency *int, reason string, observedAt time.Time) error {
	if _, err := app.services.GetService(ctx, serviceID); err != nil {
		return err
	}
	valid := state == domain.ServiceOperational || state == domain.ServiceDegraded || state == domain.ServiceOutage || state == domain.ServiceMaintenance || state == domain.ServiceUnknown
	if !valid {
		return domain.ErrInvalid
	}
	return app.services.RecordServiceStatus(ctx, domain.ServiceStatus{ID: app.ids.New(), ServiceID: serviceID, Status: state, LatencyMS: latency, Reason: reason, ObservedAt: observedAt.UTC()})
}

type AlertInput struct {
	ProductID, ServiceID *string
	Fingerprint          string
	Priority             domain.Priority
	Title, Message       string
	ExpiresAt            *time.Time
}

func (app *Service) CreateAlert(ctx context.Context, input AlertInput) (domain.Alert, error) {
	if input.ProductID != nil {
		if _, err := app.products.GetProduct(ctx, *input.ProductID); err != nil {
			return domain.Alert{}, err
		}
	}
	if input.ServiceID != nil {
		service, err := app.services.GetService(ctx, *input.ServiceID)
		if err != nil {
			return domain.Alert{}, err
		}
		if input.ProductID != nil && service.ProductID != *input.ProductID {
			return domain.Alert{}, domain.ErrInvalid
		}
	}
	now := time.Now().UTC()
	alert, err := domain.NewAlert(app.ids.New(), input.ProductID, input.ServiceID, input.Fingerprint, input.Priority, input.Title, input.Message, input.ExpiresAt, now)
	if err != nil {
		return domain.Alert{}, err
	}
	events, err := app.alertOutbox(ctx, alert, "alert.raised", now)
	if err != nil {
		return domain.Alert{}, err
	}
	if err := app.alerts.CreateAlertWithOutbox(ctx, alert, events); err != nil {
		return domain.Alert{}, err
	}
	return alert, nil
}
func (app *Service) GetAlert(ctx context.Context, id string) (domain.Alert, error) {
	return app.alerts.GetAlert(ctx, id)
}
func (app *Service) ListAlerts(ctx context.Context, limit int, cursor string, filter ports.AlertFilter) ([]domain.Alert, string, error) {
	return app.alerts.ListAlerts(ctx, normalizeLimit(limit), cursor, filter)
}
func (app *Service) AcknowledgeAlert(ctx context.Context, id string) (domain.Alert, error) {
	return app.alerts.TransitionAlertWithOutbox(ctx, id, domain.AlertAcknowledged, time.Now().UTC(), nil)
}
func (app *Service) ResolveAlert(ctx context.Context, id string) (domain.Alert, error) {
	alert, err := app.alerts.GetAlert(ctx, id)
	if err != nil {
		return domain.Alert{}, err
	}
	now := time.Now().UTC()
	events, err := app.alertOutbox(ctx, alert, "alert.resolved", now)
	if err != nil {
		return domain.Alert{}, err
	}
	return app.alerts.TransitionAlertWithOutbox(ctx, id, domain.AlertResolved, now, events)
}

type mqttEnvelope struct {
	SchemaVersion int        `json:"schema_version"`
	EventID       string     `json:"event_id"`
	Type          string     `json:"type"`
	OccurredAt    time.Time  `json:"occurred_at"`
	ExpiresAt     *time.Time `json:"expires_at"`
	CorrelationID string     `json:"correlation_id"`
	Data          any        `json:"data"`
}
type alertData struct {
	Kind       string            `json:"kind"`
	AlertID    string            `json:"alert_id"`
	Severity   domain.Priority   `json:"severity"`
	Title      string            `json:"title"`
	Message    string            `json:"message"`
	DurationMS int               `json:"duration_ms"`
	Style      NotificationStyle `json:"style"`
}

func (app *Service) alertOutbox(ctx context.Context, alert domain.Alert, eventType string, now time.Time) ([]domain.OutboxEvent, error) {
	style, exists := app.styles[alert.Priority]
	if !exists {
		return nil, fmt.Errorf("notification style not configured for %s", alert.Priority)
	}
	devices, err := app.allActiveDevices(ctx)
	if err != nil {
		return nil, err
	}
	events := make([]domain.OutboxEvent, 0, len(devices))
	for _, device := range devices {
		eventID := app.ids.New()
		expires := alert.ExpiresAt
		if expires == nil {
			value := now.Add(app.alertDuration)
			expires = &value
		}
		raw, err := json.Marshal(mqttEnvelope{SchemaVersion: 1, EventID: eventID, Type: eventType, OccurredAt: now, ExpiresAt: expires, CorrelationID: alert.ID, Data: alertData{Kind: "alert", AlertID: alert.ID, Severity: alert.Priority, Title: alert.Title, Message: alert.Message, DurationMS: int(app.alertDuration.Milliseconds()), Style: style}})
		if err != nil {
			return nil, err
		}
		events = append(events, domain.OutboxEvent{ID: app.ids.New(), EventID: eventID, Topic: fmt.Sprintf("desk/device/%s/alerts", device.DeviceID), Payload: raw, NextAttemptAt: now, CreatedAt: now})
	}
	return events, nil
}
func (app *Service) allActiveDevices(ctx context.Context) ([]domain.Device, error) {
	var result []domain.Device
	cursor := ""
	for {
		items, next, err := app.devices.ListDevices(ctx, 100, cursor)
		if err != nil {
			return nil, err
		}
		for _, device := range items {
			if device.State != domain.DeviceDisabled && device.State != domain.DeviceRevoked {
				result = append(result, device)
			}
		}
		if next == "" {
			return result, nil
		}
		cursor = next
	}
}

func RunOutbox(ctx context.Context, repository ports.OutboxRepository, publisher ports.EventPublisher, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer publisher.Close()
	dispatch := func() {
		events, err := repository.PendingOutbox(ctx, 50, time.Now().UTC())
		if err != nil {
			return
		}
		for _, event := range events {
			if err := publisher.Publish(ctx, event.Topic, event.Payload); err != nil {
				backoff := time.Second * time.Duration(1<<min(event.Attempts, 6))
				_ = repository.MarkOutboxFailed(ctx, event.ID, time.Now().UTC().Add(backoff))
				continue
			}
			_ = repository.MarkOutboxPublished(ctx, event.ID, time.Now().UTC())
		}
	}
	dispatch()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dispatch()
		}
	}
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
