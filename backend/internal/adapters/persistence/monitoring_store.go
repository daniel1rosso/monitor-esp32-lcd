package persistence

import (
	"context"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) SaveMarketQuotes(ctx context.Context, values []domain.MarketQuote) error {
	if len(values) == 0 {
		return nil
	}
	for index := range values {
		if values[index].ChangePercent != 0 {
			continue
		}
		var previous domain.MarketQuote
		err := s.db.WithContext(ctx).Where("provider = ? AND symbol = ?", values[index].Provider, values[index].Symbol).Order("observed_at DESC").First(&previous).Error
		if err == nil && previous.Last != 0 {
			values[index].ChangePercent = (values[index].Last - previous.Last) / previous.Last * 100
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return translate(err)
		}
	}
	return translate(s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&values).Error)
}

func (s *Store) SaveWeather(ctx context.Context, value domain.WeatherObservation) error {
	return translate(s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&value).Error)
}

func (s *Store) SaveDeployment(ctx context.Context, value domain.Deployment) error {
	return translate(s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider"}, {Name: "external_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"product_id", "repository", "branch", "workflow", "status", "commit_sha", "commit_message", "author", "started_at", "finished_at", "html_url"}),
	}).Create(&value).Error)
}

func (s *Store) ListDeployments(ctx context.Context, limit int, cursor string) ([]domain.Deployment, string, error) {
	query := s.db.WithContext(ctx)
	if cursor != "" {
		query = query.Where("id < ?", cursor)
	}
	var values []domain.Deployment
	if err := query.Order("started_at DESC, id DESC").Limit(limit + 1).Find(&values).Error; err != nil {
		return nil, "", translate(err)
	}
	next := ""
	if len(values) > limit {
		next = values[limit-1].ID
		values = values[:limit]
	}
	return values, next, nil
}

func (s *Store) ListMarketQuotes(ctx context.Context, limit int, symbol string) ([]domain.MarketQuote, error) {
	query := s.db.WithContext(ctx)
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	var values []domain.MarketQuote
	if err := query.Order("observed_at DESC").Limit(limit).Find(&values).Error; err != nil {
		return nil, translate(err)
	}
	return values, nil
}

func (s *Store) WeatherByLocation(ctx context.Context, location string) (*domain.WeatherObservation, error) {
	var value domain.WeatherObservation
	err := s.db.WithContext(ctx).Where("location_key = ?", location).Order("observed_at DESC").First(&value).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, translate(err)
	}
	return &value, nil
}

func (s *Store) RecordCollectorRun(ctx context.Context, value domain.CollectorRun) error {
	return translate(s.db.WithContext(ctx).Create(&value).Error)
}

func (s *Store) CreateOutbox(ctx context.Context, value domain.OutboxEvent) error {
	return translate(s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(outboxModel(value)).Error)
}

func (s *Store) RegisterWebhookDelivery(ctx context.Context, value domain.WebhookDelivery) (bool, error) {
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&value)
	return result.RowsAffected == 1, translate(result.Error)
}

func (s *Store) DeleteWebhookDelivery(ctx context.Context, provider, deliveryID string) error {
	return translate(s.db.WithContext(ctx).Where("provider = ? AND delivery_id = ?", provider, deliveryID).Delete(&domain.WebhookDelivery{}).Error)
}

var _ ports.OperationsRepository = (*Store)(nil)
