package persistence

import (
	"context"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
	"gorm.io/gorm"
)

func (s *Store) LatestDeployment(ctx context.Context) (*domain.Deployment, error) {
	var value domain.Deployment
	err := s.db.WithContext(ctx).Table("deployments").Order("started_at DESC").First(&value).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, translate(err)
	}
	return &value, nil
}

func (s *Store) LatestMarketQuotes(ctx context.Context, limit int) ([]domain.MarketQuote, error) {
	var values []domain.MarketQuote
	query := `SELECT q.* FROM market_quotes q JOIN (SELECT provider, symbol, MAX(observed_at) observed_at FROM market_quotes GROUP BY provider, symbol) latest ON latest.provider=q.provider AND latest.symbol=q.symbol AND latest.observed_at=q.observed_at ORDER BY q.symbol LIMIT ?`
	if err := s.db.WithContext(ctx).Raw(query, limit).Scan(&values).Error; err != nil {
		return nil, translate(err)
	}
	return values, nil
}

func (s *Store) LatestWeather(ctx context.Context) (*domain.WeatherObservation, error) {
	var value domain.WeatherObservation
	err := s.db.WithContext(ctx).Table("weather_observations").Order("observed_at DESC").First(&value).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, translate(err)
	}
	return &value, nil
}

var _ ports.SnapshotRepository = (*Store)(nil)
