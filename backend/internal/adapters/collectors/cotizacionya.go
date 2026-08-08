package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/config"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
)

type CotizacionYa struct {
	config config.CotizacionYa
	client *http.Client
	ids    ports.IDGenerator
}

func NewCotizacionYa(value config.CotizacionYa, ids ports.IDGenerator) *CotizacionYa {
	if !value.Enabled {
		return nil
	}
	return &CotizacionYa{config: value, client: &http.Client{Timeout: value.Timeout.Duration}, ids: ids}
}
func (c *CotizacionYa) Name() string            { return "cotizacion_ya" }
func (c *CotizacionYa) Interval() time.Duration { return c.config.Interval.Duration }

type cryptoResponse struct {
	Assets []struct {
		Symbol, QuoteAsset            string
		LastPrice, BidPrice, AskPrice float64
		PriceChangePercent            float64
		UpdatedAt                     time.Time
	} `json:"assets"`
}
type dollarsResponse struct {
	Dollars []struct {
		Source, DisplayName string
		Buy, Sell           float64
		Variation           *float64
		UpdatedAt           time.Time
	} `json:"dollars"`
}

func (c *CotizacionYa) Collect(ctx context.Context) (Result, error) {
	crypto, err := getJSON[cryptoResponse](ctx, c.client, strings.TrimRight(c.config.BaseURL, "/")+"/crypto/assets")
	if err != nil {
		return Result{}, fmt.Errorf("crypto assets: %w", err)
	}
	dollars, err := getJSON[dollarsResponse](ctx, c.client, strings.TrimRight(c.config.BaseURL, "/")+"/argentina/dollars")
	if err != nil {
		return Result{}, fmt.Errorf("argentina dollars: %w", err)
	}
	wanted := map[string]bool{}
	for _, symbol := range c.config.Symbols {
		wanted[strings.ToUpper(symbol)] = true
	}
	result := Result{}
	for _, asset := range crypto.Assets {
		if !wanted[strings.ToUpper(asset.Symbol)] {
			continue
		}
		bid, ask := asset.BidPrice, asset.AskPrice
		result.Quotes = append(result.Quotes, domain.MarketQuote{ID: c.ids.New(), Provider: "cotizacionya", Symbol: asset.Symbol, QuoteAsset: asset.QuoteAsset, Bid: &bid, Ask: &ask, Last: asset.LastPrice, ChangePercent: asset.PriceChangePercent, ObservedAt: asset.UpdatedAt.UTC()})
	}
	for _, dollar := range dollars.Dollars {
		if strings.EqualFold(dollar.Source, "blue") && (wanted["BLUE"] || wanted["USD_BLUE"]) {
			bid, ask, change := dollar.Buy, dollar.Sell, 0.0
			if dollar.Variation != nil {
				change = *dollar.Variation
			}
			result.Quotes = append(result.Quotes, domain.MarketQuote{ID: c.ids.New(), Provider: "cotizacionya", Symbol: "USD_BLUE", QuoteAsset: "ARS", Bid: &bid, Ask: &ask, Last: ask, ChangePercent: change, ObservedAt: dollar.UpdatedAt.UTC()})
		}
	}
	if len(result.Quotes) == 0 {
		return Result{}, fmt.Errorf("provider returned no configured symbols")
	}
	return result, nil
}

func getJSON[T any](ctx context.Context, client *http.Client, endpoint string) (T, error) {
	var value T
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return value, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return value, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return value, fmt.Errorf("unexpected status %s", response.Status)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&value); err != nil {
		return value, err
	}
	return value, nil
}
