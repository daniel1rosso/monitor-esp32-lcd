package collectors

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/config"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/security"
)

func TestCotizacionYaCollectsConfiguredSymbols(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := ""
		switch request.URL.Path {
		case "/crypto/assets":
			body = `{"assets":[{"symbol":"BTCUSDT","quoteAsset":"USDT","lastPrice":65000,"bidPrice":64999,"askPrice":65001,"priceChangePercent":1.2,"updatedAt":"2026-08-08T03:00:00Z"},{"symbol":"ETHUSDT","quoteAsset":"USDT","lastPrice":1900,"updatedAt":"2026-08-08T03:00:00Z"},{"symbol":"XRPUSDT","quoteAsset":"USDT","lastPrice":1.03,"bidPrice":1.02,"askPrice":1.04,"priceChangePercent":-0.2,"updatedAt":"2026-08-08T03:00:00Z"}]}`
		case "/argentina/dollars":
			body = `{"dollars":[{"source":"blue","displayName":"Blue","buy":1505,"sell":1525,"variation":null,"updatedAt":"2026-08-08T03:00:00Z"}]}`
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return jsonResponse(body), nil
	})}

	collector := NewCotizacionYa(config.CotizacionYa{ScheduledCollector: config.ScheduledCollector{Enabled: true, Interval: config.Duration{Duration: time.Minute}, Timeout: config.Duration{Duration: time.Second}}, BaseURL: "https://provider.test", Symbols: []string{"blue", "BTCUSDT", "XRPUSDT"}}, security.ULIDGenerator{})
	collector.client = client
	result, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Quotes) != 3 || result.Quotes[2].Symbol != "USD_BLUE" || result.Quotes[2].Last != 1525 {
		t.Fatalf("unexpected quotes: %#v", result.Quotes)
	}
}

func TestOpenMeteoCollectsRioCuarto(t *testing.T) {
	collector := NewOpenMeteo(config.OpenMeteo{ScheduledCollector: config.ScheduledCollector{Enabled: true, Interval: config.Duration{Duration: time.Minute}, Timeout: config.Duration{Duration: time.Second}}, BaseURL: "https://provider.test", Location: config.Location{Key: "rio-cuarto", Name: "Río Cuarto", Latitude: -33.1307, Longitude: -64.3499, Timezone: "America/Argentina/Cordoba"}}, security.ULIDGenerator{})
	collector.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"current":{"time":"2026-08-08T00:30","temperature_2m":6.8,"apparent_temperature":4.7,"relative_humidity_2m":74,"weather_code":0,"wind_speed_10m":2.4},"daily":{"temperature_2m_min":[3.4],"temperature_2m_max":[12.1],"precipitation_probability_max":[0]}}`), nil
	})}
	result, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Weather == nil || result.Weather.TemperatureC != 6.8 || result.Weather.ObservedAt.Hour() != 3 {
		t.Fatalf("unexpected weather: %#v", result.Weather)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}
}
