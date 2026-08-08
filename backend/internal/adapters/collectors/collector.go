// Package collectors implements scheduled external data sources.
package collectors

import (
	"context"
	"log/slog"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
)

type Result struct {
	Quotes  []domain.MarketQuote
	Weather *domain.WeatherObservation
}

type Collector interface {
	Collect(context.Context) (Result, error)
	Name() string
	Interval() time.Duration
}

func Run(ctx context.Context, repository ports.OperationsRepository, ids ports.IDGenerator, logger *slog.Logger, values ...Collector) {
	for _, value := range values {
		if value == nil {
			continue
		}
		go run(ctx, repository, ids, logger, value)
	}
}

func run(ctx context.Context, repository ports.OperationsRepository, ids ports.IDGenerator, logger *slog.Logger, collector Collector) {
	collect := func() {
		started := time.Now().UTC()
		result, err := collector.Collect(ctx)
		finished := time.Now().UTC()
		run := domain.CollectorRun{ID: ids.New(), Collector: collector.Name(), StartedAt: started, FinishedAt: &finished}
		if err != nil {
			run.Status, run.ErrorCode = "failed", "collect_failed"
			_ = repository.RecordCollectorRun(ctx, run)
			logger.Warn("collector failed", "collector", collector.Name(), "error", err)
			return
		}
		if len(result.Quotes) > 0 {
			err = repository.SaveMarketQuotes(ctx, result.Quotes)
			run.RecordsWritten += len(result.Quotes)
		}
		if err == nil && result.Weather != nil {
			err = repository.SaveWeather(ctx, *result.Weather)
			run.RecordsWritten++
		}
		if err != nil {
			run.Status, run.ErrorCode = "failed", "persist_failed"
			logger.Warn("collector persistence failed", "collector", collector.Name(), "error", err)
		} else {
			run.Status = "success"
			logger.Info("collector completed", "collector", collector.Name(), "records", run.RecordsWritten)
		}
		_ = repository.RecordCollectorRun(ctx, run)
	}
	collect()
	ticker := time.NewTicker(collector.Interval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect()
		}
	}
}
