// Package domain contains business state and invariants without infrastructure dependencies.
package domain

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrInvalid      = errors.New("invalid input")
	stableKey       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)
	deviceKey       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,63}$`)
)

type ServiceState string

const (
	ServiceOperational ServiceState = "operational"
	ServiceDegraded    ServiceState = "degraded"
	ServiceOutage      ServiceState = "outage"
	ServiceMaintenance ServiceState = "maintenance"
	ServiceUnknown     ServiceState = "unknown"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type RefreshSession struct {
	ID        string
	UserID    string
	FamilyID  string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

type Product struct {
	ID          string
	Key         string
	Name        string
	Description string
	Enabled     bool
	Health      ServiceState
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ArchivedAt  *time.Time
}

func NewProduct(id, key, name, description string, enabled bool, now time.Time) (Product, error) {
	if !stableKey.MatchString(key) || strings.TrimSpace(name) == "" || len(name) > 100 || len(description) > 500 {
		return Product{}, ErrInvalid
	}
	return Product{ID: id, Key: key, Name: strings.TrimSpace(name), Description: description, Enabled: enabled, Health: ServiceUnknown, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

type Screen struct {
	ID         string
	Type       string
	Priority   int
	DurationMS int
	Enabled    bool
	ConfigJSON []byte
}

type ScreenProfile struct {
	ID        string
	Key       string
	Name      string
	Revision  int
	Screens   []Screen
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewScreenProfile(id, key, name string, screens []Screen, now time.Time) (ScreenProfile, error) {
	if !stableKey.MatchString(key) || strings.TrimSpace(name) == "" || len(name) > 100 || len(screens) > 12 {
		return ScreenProfile{}, ErrInvalid
	}
	seen := make(map[string]struct{}, len(screens))
	for _, screen := range screens {
		if !stableKey.MatchString(screen.ID) || screen.Priority < 0 || screen.Priority > 100 || screen.DurationMS < 3000 || screen.DurationMS > 60000 {
			return ScreenProfile{}, ErrInvalid
		}
		if _, exists := seen[screen.ID]; exists {
			return ScreenProfile{}, ErrInvalid
		}
		seen[screen.ID] = struct{}{}
	}
	return ScreenProfile{ID: id, Key: key, Name: strings.TrimSpace(name), Revision: 1, Screens: screens, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

type DeviceState string

const (
	DeviceOnline   DeviceState = "online"
	DeviceOffline  DeviceState = "offline"
	DeviceDisabled DeviceState = "disabled"
	DeviceRevoked  DeviceState = "revoked"
)

type Device struct {
	ID              string
	DeviceID        string
	Name            string
	ProfileID       string
	State           DeviceState
	FirmwareVersion *string
	LastSeenAt      *time.Time
	OverridesJSON   []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewDevice(id, deviceID, name, profileID string, now time.Time) (Device, error) {
	if !deviceKey.MatchString(deviceID) || strings.TrimSpace(name) == "" || len(name) > 100 || profileID == "" {
		return Device{}, ErrInvalid
	}
	return Device{ID: id, DeviceID: deviceID, Name: strings.TrimSpace(name), ProfileID: profileID, State: DeviceOffline, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

type DeviceToken struct {
	ID         string
	DeviceID   string
	SecretHash string
	Generation int
	RevokedAt  *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
}

type Service struct {
	ID, ProductID, Key, Name, Kind string
	Endpoint                       *string
	Enabled                        bool
	Status                         ServiceState
	LatencyMS                      *int
	CreatedAt, UpdatedAt           time.Time
	ArchivedAt                     *time.Time
}

func NewService(id, productID, key, name, kind string, endpoint *string, enabled bool, now time.Time) (Service, error) {
	if id == "" || productID == "" || !stableKey.MatchString(key) || strings.TrimSpace(name) == "" || len(name) > 100 || len(kind) > 64 {
		return Service{}, ErrInvalid
	}
	if endpoint != nil {
		parsed, err := url.ParseRequestURI(*endpoint)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return Service{}, ErrInvalid
		}
	}
	return Service{ID: id, ProductID: productID, Key: key, Name: strings.TrimSpace(name), Kind: kind, Endpoint: endpoint, Enabled: enabled, Status: ServiceUnknown, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

type ServiceStatus struct {
	ID, ServiceID string
	Status        ServiceState
	LatencyMS     *int
	Reason        string
	ObservedAt    time.Time
}

type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityWarning  Priority = "warning"
	PriorityInfo     Priority = "info"
	PrioritySuccess  Priority = "success"
)

type AlertState string

const (
	AlertOpen         AlertState = "open"
	AlertAcknowledged AlertState = "acknowledged"
	AlertResolved     AlertState = "resolved"
)

type Alert struct {
	ID                                    string
	ProductID, ServiceID                  *string
	Fingerprint                           string
	Priority                              Priority
	State                                 AlertState
	Title, Message                        string
	OpenedAt, UpdatedAt                   time.Time
	AcknowledgedAt, ResolvedAt, ExpiresAt *time.Time
}

func NewAlert(id string, productID, serviceID *string, fingerprint string, priority Priority, title, message string, expiresAt *time.Time, now time.Time) (Alert, error) {
	validPriority := priority == PriorityCritical || priority == PriorityWarning || priority == PriorityInfo || priority == PrioritySuccess
	if id == "" || strings.TrimSpace(fingerprint) == "" || len(fingerprint) > 160 || !validPriority || strings.TrimSpace(title) == "" || len(title) > 48 || strings.TrimSpace(message) == "" || len(message) > 160 {
		return Alert{}, ErrInvalid
	}
	return Alert{ID: id, ProductID: productID, ServiceID: serviceID, Fingerprint: fingerprint, Priority: priority, State: AlertOpen, Title: strings.TrimSpace(title), Message: strings.TrimSpace(message), OpenedAt: now.UTC(), UpdatedAt: now.UTC(), ExpiresAt: expiresAt}, nil
}

type Deployment struct {
	ID                                                                                           string
	ProductID                                                                                    *string
	Provider, ExternalID, Repository, Branch, Workflow, Status, CommitSHA, CommitMessage, Author string
	StartedAt                                                                                    time.Time
	FinishedAt                                                                                   *time.Time
	HTMLURL                                                                                      *string
}

type MarketQuote struct {
	ID, Provider, Symbol, QuoteAsset string
	Bid, Ask                         *float64
	Last, ChangePercent              float64
	ObservedAt                       time.Time
}

type WeatherObservation struct {
	ID, LocationKey, LocationName                           string
	TemperatureC, ApparentTemperatureC                      float64
	HumidityPercent                                         *float64
	WeatherCode                                             int
	WindKMH, DailyMinC, DailyMaxC, PrecipitationProbability *float64
	ObservedAt                                              time.Time
}

type NotificationHistory struct {
	ID, EventID, Destination, Channel string
	Priority                          Priority
	Status                            string
	CreatedAt                         time.Time
}

type Configuration struct {
	ID, Namespace, Key, ValueJSON, ValueType string
	Revision                                 int
	Editable                                 bool
	CreatedAt, UpdatedAt                     time.Time
}

type OutboxEvent struct {
	ID, EventID, Topic string
	Payload            []byte
	Attempts           int
	NextAttemptAt      time.Time
	PublishedAt        *time.Time
	CreatedAt          time.Time
}
