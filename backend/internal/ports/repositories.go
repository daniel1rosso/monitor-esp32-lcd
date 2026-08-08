// Package ports declares the dependencies required by application use cases.
package ports

import (
	"context"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
)

type HealthChecker interface {
	Ping(context.Context) error
}

type ProductRepository interface {
	CreateProduct(context.Context, domain.Product) error
	GetProduct(context.Context, string) (domain.Product, error)
	GetProductByKey(context.Context, string) (domain.Product, error)
	ListProducts(context.Context, int, string) ([]domain.Product, string, error)
	UpdateProduct(context.Context, domain.Product) error
	ArchiveProduct(context.Context, string, time.Time) error
}

type ServiceRepository interface {
	CreateService(context.Context, domain.Service) error
	GetService(context.Context, string) (domain.Service, error)
	GetServiceByProductAndKey(context.Context, string, string) (domain.Service, error)
	ListServices(context.Context, int, string, string) ([]domain.Service, string, error)
	UpdateService(context.Context, domain.Service) error
	ArchiveService(context.Context, string, time.Time) error
	RecordServiceStatus(context.Context, domain.ServiceStatus) error
}

type OperationsRepository interface {
	SaveMarketQuotes(context.Context, []domain.MarketQuote) error
	SaveWeather(context.Context, domain.WeatherObservation) error
	SaveDeployment(context.Context, domain.Deployment) error
	ListDeployments(context.Context, int, string) ([]domain.Deployment, string, error)
	ListMarketQuotes(context.Context, int, string) ([]domain.MarketQuote, error)
	WeatherByLocation(context.Context, string) (*domain.WeatherObservation, error)
	RecordCollectorRun(context.Context, domain.CollectorRun) error
	CreateOutbox(context.Context, domain.OutboxEvent) error
	RegisterWebhookDelivery(context.Context, domain.WebhookDelivery) (bool, error)
	DeleteWebhookDelivery(context.Context, string, string) error
}

type AlertFilter struct{ State, Priority, ProductID string }
type AlertRepository interface {
	CreateAlertWithOutbox(context.Context, domain.Alert, []domain.OutboxEvent) error
	GetAlert(context.Context, string) (domain.Alert, error)
	GetOpenAlertByFingerprint(context.Context, string) (domain.Alert, error)
	ListAlerts(context.Context, int, string, AlertFilter) ([]domain.Alert, string, error)
	TransitionAlertWithOutbox(context.Context, string, domain.AlertState, time.Time, []domain.OutboxEvent) (domain.Alert, error)
}

type OutboxRepository interface {
	PendingOutbox(context.Context, int, time.Time) ([]domain.OutboxEvent, error)
	MarkOutboxPublished(context.Context, string, time.Time) error
	MarkOutboxFailed(context.Context, string, time.Time) error
}

type SnapshotRepository interface {
	LatestDeployment(context.Context) (*domain.Deployment, error)
	LatestMarketQuotes(context.Context, int) ([]domain.MarketQuote, error)
	LatestWeather(context.Context) (*domain.WeatherObservation, error)
}

type EventPublisher interface {
	Publish(context.Context, string, []byte) error
	Close()
}

type MQTTCredentialManager interface {
	ProvisionDevice(context.Context, string, string, string) error
	RevokeDevice(context.Context, string, string, bool) error
}

type ProfileRepository interface {
	GetProfileByID(context.Context, string) (domain.ScreenProfile, error)
	GetProfileByKey(context.Context, string) (domain.ScreenProfile, error)
	ListProfiles(context.Context, int, string) ([]domain.ScreenProfile, string, error)
	CreateProfile(context.Context, domain.ScreenProfile, IDGenerator) error
	ReplaceProfile(context.Context, domain.ScreenProfile, IDGenerator) error
	DeleteProfile(context.Context, string) error
}

type DeviceRepository interface {
	CreateWithToken(context.Context, domain.Device, domain.DeviceToken, string, string) error
	GetDeviceByID(context.Context, string) (domain.Device, error)
	GetDeviceByDeviceID(context.Context, string) (domain.Device, error)
	ListDevices(context.Context, int, string) ([]domain.Device, string, error)
	UpdateDevice(context.Context, domain.Device) error
	LatestToken(context.Context, string) (domain.DeviceToken, error)
	LatestMQTTUsername(context.Context, string) (string, error)
	RotateToken(context.Context, domain.DeviceToken, string, string, time.Time) error
	Touch(context.Context, string, string, time.Time) error
	Revoke(context.Context, string, time.Time) error
}

type UserRepository interface {
	GetUserByEmail(context.Context, string) (domain.User, error)
	GetUserByID(context.Context, string) (domain.User, error)
	CreateUser(context.Context, domain.User) error
}

type SessionRepository interface {
	CreateRefreshSession(context.Context, domain.RefreshSession) error
	RotateRefreshSession(context.Context, string, domain.RefreshSession, time.Time) (string, error)
	RevokeRefreshSession(context.Context, string, time.Time) error
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Verify(string, string) bool
}

type AccessClaims struct {
	Subject   string
	Audience  string
	Role      string
	TokenID   string
	ExpiresAt time.Time
}

type TokenIssuer interface {
	Issue(subject, audience, role string, ttl time.Duration) (string, time.Time, error)
	Verify(string, string) (AccessClaims, error)
}

type IDGenerator interface{ New() string }
