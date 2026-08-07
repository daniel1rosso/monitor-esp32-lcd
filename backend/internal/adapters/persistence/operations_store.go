package persistence

import (
	"context"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
	"gorm.io/gorm"
)

type serviceRow struct {
	ID, ProductID, Key, Name, Kind string
	Endpoint                       *string
	Enabled                        bool
	CreatedAt, UpdatedAt           time.Time
	ArchivedAt                     *time.Time
}

func (serviceRow) TableName() string { return "services" }

type serviceStatusRow struct {
	ID, ServiceID, Status string
	LatencyMS             *int
	Reason                string
	ObservedAt            time.Time
}

func (serviceStatusRow) TableName() string { return "service_statuses" }

func serviceDomain(row serviceRow, status *serviceStatusRow) domain.Service {
	value := domain.Service{ID: row.ID, ProductID: row.ProductID, Key: row.Key, Name: row.Name, Kind: row.Kind, Endpoint: row.Endpoint, Enabled: row.Enabled, Status: domain.ServiceUnknown, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ArchivedAt: row.ArchivedAt}
	if status != nil {
		value.Status = domain.ServiceState(status.Status)
		value.LatencyMS = status.LatencyMS
	}
	return value
}
func (s *Store) latestServiceStatus(ctx context.Context, id string) (*serviceStatusRow, error) {
	var row serviceStatusRow
	err := s.db.WithContext(ctx).Where("service_id = ?", id).Order("observed_at DESC").First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, translate(err)
	}
	return &row, nil
}
func (s *Store) CreateService(ctx context.Context, value domain.Service) error {
	return translate(s.db.WithContext(ctx).Create(serviceRow{ID: value.ID, ProductID: value.ProductID, Key: value.Key, Name: value.Name, Kind: value.Kind, Endpoint: value.Endpoint, Enabled: value.Enabled, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}).Error)
}
func (s *Store) GetService(ctx context.Context, id string) (domain.Service, error) {
	var row serviceRow
	if err := s.db.WithContext(ctx).Where("id = ? AND archived_at IS NULL", id).First(&row).Error; err != nil {
		return domain.Service{}, translate(err)
	}
	status, err := s.latestServiceStatus(ctx, id)
	if err != nil {
		return domain.Service{}, err
	}
	return serviceDomain(row, status), nil
}
func (s *Store) ListServices(ctx context.Context, limit int, cursor, productID string) ([]domain.Service, string, error) {
	query := s.db.WithContext(ctx).Where("archived_at IS NULL")
	if cursor != "" {
		query = query.Where("id > ?", cursor)
	}
	if productID != "" {
		query = query.Where("product_id = ?", productID)
	}
	var rows []serviceRow
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", translate(err)
	}
	next := ""
	if len(rows) > limit {
		next = rows[limit-1].ID
		rows = rows[:limit]
	}
	items := make([]domain.Service, 0, len(rows))
	for _, row := range rows {
		status, err := s.latestServiceStatus(ctx, row.ID)
		if err != nil {
			return nil, "", err
		}
		items = append(items, serviceDomain(row, status))
	}
	return items, next, nil
}
func (s *Store) UpdateService(ctx context.Context, value domain.Service) error {
	result := s.db.WithContext(ctx).Model(&serviceRow{}).Where("id = ? AND archived_at IS NULL", value.ID).Updates(map[string]any{"name": value.Name, "kind": value.Kind, "endpoint": value.Endpoint, "enabled": value.Enabled, "updated_at": value.UpdatedAt})
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
func (s *Store) ArchiveService(ctx context.Context, id string, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&serviceRow{}).Where("id = ? AND archived_at IS NULL", id).Updates(map[string]any{"archived_at": at, "enabled": false, "updated_at": at})
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
func (s *Store) RecordServiceStatus(ctx context.Context, value domain.ServiceStatus) error {
	return translate(s.db.WithContext(ctx).Create(serviceStatusRow{ID: value.ID, ServiceID: value.ServiceID, Status: string(value.Status), LatencyMS: value.LatencyMS, Reason: value.Reason, ObservedAt: value.ObservedAt}).Error)
}

type alertRow struct {
	ID                                           string
	ProductID, ServiceID                         *string
	Fingerprint, Priority, State, Title, Message string
	OpenedAt, UpdatedAt                          time.Time
	AcknowledgedAt, ResolvedAt, ExpiresAt        *time.Time
}

func (alertRow) TableName() string { return "alerts" }
func alertDomain(row alertRow) domain.Alert {
	return domain.Alert{ID: row.ID, ProductID: row.ProductID, ServiceID: row.ServiceID, Fingerprint: row.Fingerprint, Priority: domain.Priority(row.Priority), State: domain.AlertState(row.State), Title: row.Title, Message: row.Message, OpenedAt: row.OpenedAt, UpdatedAt: row.UpdatedAt, AcknowledgedAt: row.AcknowledgedAt, ResolvedAt: row.ResolvedAt, ExpiresAt: row.ExpiresAt}
}
func alertModel(value domain.Alert) alertRow {
	return alertRow{ID: value.ID, ProductID: value.ProductID, ServiceID: value.ServiceID, Fingerprint: value.Fingerprint, Priority: string(value.Priority), State: string(value.State), Title: value.Title, Message: value.Message, OpenedAt: value.OpenedAt, UpdatedAt: value.UpdatedAt, AcknowledgedAt: value.AcknowledgedAt, ResolvedAt: value.ResolvedAt, ExpiresAt: value.ExpiresAt}
}

type outboxRow struct {
	ID, EventID, Topic string
	Payload            []byte
	Attempts           int
	NextAttemptAt      time.Time
	PublishedAt        *time.Time
	CreatedAt          time.Time
}

func (outboxRow) TableName() string { return "outbox_events" }
func outboxModel(value domain.OutboxEvent) outboxRow {
	return outboxRow{ID: value.ID, EventID: value.EventID, Topic: value.Topic, Payload: value.Payload, Attempts: value.Attempts, NextAttemptAt: value.NextAttemptAt, PublishedAt: value.PublishedAt, CreatedAt: value.CreatedAt}
}
func (s *Store) CreateAlertWithOutbox(ctx context.Context, alert domain.Alert, events []domain.OutboxEvent) error {
	return translate(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(alertModel(alert)).Error; err != nil {
			return err
		}
		for _, event := range events {
			if err := tx.Create(outboxModel(event)).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}
func (s *Store) GetAlert(ctx context.Context, id string) (domain.Alert, error) {
	var row alertRow
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	return alertDomain(row), translate(err)
}
func (s *Store) ListAlerts(ctx context.Context, limit int, cursor string, filter ports.AlertFilter) ([]domain.Alert, string, error) {
	query := s.db.WithContext(ctx)
	if cursor != "" {
		query = query.Where("id > ?", cursor)
	}
	if filter.State != "" {
		query = query.Where("state = ?", filter.State)
	}
	if filter.Priority != "" {
		query = query.Where("priority = ?", filter.Priority)
	}
	if filter.ProductID != "" {
		query = query.Where("product_id = ?", filter.ProductID)
	}
	var rows []alertRow
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", translate(err)
	}
	next := ""
	if len(rows) > limit {
		next = rows[limit-1].ID
		rows = rows[:limit]
	}
	items := make([]domain.Alert, len(rows))
	for index, row := range rows {
		items[index] = alertDomain(row)
	}
	return items, next, nil
}
func (s *Store) TransitionAlertWithOutbox(ctx context.Context, id string, target domain.AlertState, at time.Time, events []domain.OutboxEvent) (domain.Alert, error) {
	var result domain.Alert
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row alertRow
		if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
			return err
		}
		if domain.AlertState(row.State) == target {
			result = alertDomain(row)
			return nil
		}
		if row.State == string(domain.AlertResolved) || target == domain.AlertOpen {
			return domain.ErrConflict
		}
		updates := map[string]any{"state": string(target), "updated_at": at}
		if target == domain.AlertAcknowledged {
			updates["acknowledged_at"] = at
		} else if target == domain.AlertResolved {
			updates["resolved_at"] = at
		}
		if err := tx.Model(&alertRow{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		for _, event := range events {
			if err := tx.Create(outboxModel(event)).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
			return err
		}
		result = alertDomain(row)
		return nil
	})
	return result, translate(err)
}

func (s *Store) PendingOutbox(ctx context.Context, limit int, now time.Time) ([]domain.OutboxEvent, error) {
	var rows []outboxRow
	if err := s.db.WithContext(ctx).Where("published_at IS NULL AND next_attempt_at <= ?", now).Order("created_at ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, translate(err)
	}
	items := make([]domain.OutboxEvent, len(rows))
	for index, row := range rows {
		items[index] = domain.OutboxEvent{ID: row.ID, EventID: row.EventID, Topic: row.Topic, Payload: row.Payload, Attempts: row.Attempts, NextAttemptAt: row.NextAttemptAt, PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt}
	}
	return items, nil
}
func (s *Store) MarkOutboxPublished(ctx context.Context, id string, at time.Time) error {
	return translate(s.db.WithContext(ctx).Model(&outboxRow{}).Where("id = ? AND published_at IS NULL", id).Update("published_at", at).Error)
}
func (s *Store) MarkOutboxFailed(ctx context.Context, id string, next time.Time) error {
	return translate(s.db.WithContext(ctx).Model(&outboxRow{}).Where("id = ? AND published_at IS NULL", id).Updates(map[string]any{"attempts": gorm.Expr("attempts + 1"), "next_attempt_at": next}).Error)
}

var _ ports.ServiceRepository = (*Store)(nil)
var _ ports.AlertRepository = (*Store)(nil)
var _ ports.OutboxRepository = (*Store)(nil)
