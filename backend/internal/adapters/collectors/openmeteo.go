package collectors

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/config"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
)

type OpenMeteo struct {
	config config.OpenMeteo
	client *http.Client
	ids    ports.IDGenerator
}

func NewOpenMeteo(value config.OpenMeteo, ids ports.IDGenerator) *OpenMeteo {
	if !value.Enabled {
		return nil
	}
	return &OpenMeteo{config: value, client: &http.Client{Timeout: value.Timeout.Duration}, ids: ids}
}
func (c *OpenMeteo) Name() string            { return "open_meteo" }
func (c *OpenMeteo) Interval() time.Duration { return c.config.Interval.Duration }

type openMeteoResponse struct {
	Current struct {
		Time                string  `json:"time"`
		Temperature         float64 `json:"temperature_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		Humidity            float64 `json:"relative_humidity_2m"`
		WeatherCode         int     `json:"weather_code"`
		Wind                float64 `json:"wind_speed_10m"`
	} `json:"current"`
	Daily struct {
		Minimum       []float64 `json:"temperature_2m_min"`
		Maximum       []float64 `json:"temperature_2m_max"`
		Precipitation []float64 `json:"precipitation_probability_max"`
	} `json:"daily"`
}

func (c *OpenMeteo) Collect(ctx context.Context) (Result, error) {
	query := url.Values{}
	query.Set("latitude", strconv.FormatFloat(c.config.Location.Latitude, 'f', -1, 64))
	query.Set("longitude", strconv.FormatFloat(c.config.Location.Longitude, 'f', -1, 64))
	query.Set("timezone", c.config.Location.Timezone)
	query.Set("forecast_days", "1")
	query.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m")
	query.Set("daily", "temperature_2m_min,temperature_2m_max,precipitation_probability_max")
	value, err := getJSON[openMeteoResponse](ctx, c.client, strings.TrimRight(c.config.BaseURL, "/")+"/forecast?"+query.Encode())
	if err != nil {
		return Result{}, err
	}
	zone, err := time.LoadLocation(c.config.Location.Timezone)
	if err != nil {
		return Result{}, fmt.Errorf("load configured timezone: %w", err)
	}
	observedAt, err := time.ParseInLocation("2006-01-02T15:04", value.Current.Time, zone)
	if err != nil {
		return Result{}, fmt.Errorf("parse provider time: %w", err)
	}
	humidity, wind := value.Current.Humidity, value.Current.Wind
	weather := domain.WeatherObservation{ID: c.ids.New(), LocationKey: c.config.Location.Key, LocationName: c.config.Location.Name, TemperatureC: value.Current.Temperature, ApparentTemperatureC: value.Current.ApparentTemperature, HumidityPercent: &humidity, WeatherCode: value.Current.WeatherCode, WindKMH: &wind, ObservedAt: observedAt.UTC()}
	if len(value.Daily.Minimum) > 0 {
		weather.DailyMinC = &value.Daily.Minimum[0]
	}
	if len(value.Daily.Maximum) > 0 {
		weather.DailyMaxC = &value.Daily.Maximum[0]
	}
	if len(value.Daily.Precipitation) > 0 {
		weather.PrecipitationProbability = &value.Daily.Precipitation[0]
	}
	return Result{Weather: &weather}, nil
}
