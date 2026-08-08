// Package application coordinates use cases through domain ports.
package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
)

const (
	UserAudience   = "desk-monitor-user"
	DeviceAudience = "desk-monitor-device"
	userTokenTTL   = 15 * time.Minute
)

type Service struct {
	products       ports.ProductRepository
	services       ports.ServiceRepository
	profiles       ports.ProfileRepository
	devices        ports.DeviceRepository
	users          ports.UserRepository
	sessions       ports.SessionRepository
	alerts         ports.AlertRepository
	outbox         ports.OutboxRepository
	snapshots      ports.SnapshotRepository
	operations     ports.OperationsRepository
	hasher         ports.PasswordHasher
	tokens         ports.TokenIssuer
	ids            ports.IDGenerator
	mqtt           ports.MQTTCredentialManager
	deviceTokenTTL time.Duration
	alertDuration  time.Duration
	styles         map[domain.Priority]NotificationStyle
	content        DeviceContentConfig
}

type NotificationStyle struct {
	Color  string `json:"color"`
	Icon   string `json:"icon"`
	LEDRGB string `json:"led_rgb"`
	Sound  string `json:"sound"`
}
type Dependencies struct {
	Products       ports.ProductRepository
	Services       ports.ServiceRepository
	Profiles       ports.ProfileRepository
	Devices        ports.DeviceRepository
	Users          ports.UserRepository
	Sessions       ports.SessionRepository
	Alerts         ports.AlertRepository
	Outbox         ports.OutboxRepository
	Snapshots      ports.SnapshotRepository
	Operations     ports.OperationsRepository
	Hasher         ports.PasswordHasher
	Tokens         ports.TokenIssuer
	IDs            ports.IDGenerator
	MQTT           ports.MQTTCredentialManager
	DeviceTokenTTL time.Duration
	AlertDuration  time.Duration
	Styles         map[domain.Priority]NotificationStyle
	Content        DeviceContentConfig
}

func New(dependencies Dependencies) *Service {
	return &Service{products: dependencies.Products, services: dependencies.Services, profiles: dependencies.Profiles, devices: dependencies.Devices, users: dependencies.Users, sessions: dependencies.Sessions, alerts: dependencies.Alerts, outbox: dependencies.Outbox, snapshots: dependencies.Snapshots, operations: dependencies.Operations, hasher: dependencies.Hasher, tokens: dependencies.Tokens, ids: dependencies.IDs, mqtt: dependencies.MQTT, deviceTokenTTL: dependencies.DeviceTokenTTL, alertDuration: dependencies.AlertDuration, styles: dependencies.Styles, content: dependencies.Content}
}

type ProductInput struct {
	Key, Name, Description string
	Enabled                bool
}

func (service *Service) CreateProduct(ctx context.Context, input ProductInput) (domain.Product, error) {
	product, err := domain.NewProduct(service.ids.New(), input.Key, input.Name, input.Description, input.Enabled, time.Now())
	if err != nil {
		return domain.Product{}, err
	}
	if err := service.products.CreateProduct(ctx, product); err != nil {
		return domain.Product{}, err
	}
	return product, nil
}

func (service *Service) GetProduct(ctx context.Context, id string) (domain.Product, error) {
	return service.products.GetProduct(ctx, id)
}
func (service *Service) ListProducts(ctx context.Context, limit int, cursor string) ([]domain.Product, string, error) {
	items, next, err := service.products.ListProducts(ctx, normalizeLimit(limit), cursor)
	if err != nil {
		return nil, "", err
	}
	for index := range items {
		services, _, listErr := service.services.ListServices(ctx, 100, "", items[index].ID)
		if listErr != nil {
			return nil, "", listErr
		}
		items[index].Health = aggregateHealth(services)
	}
	return items, next, nil
}

func aggregateHealth(values []domain.Service) domain.ServiceState {
	if len(values) == 0 {
		return domain.ServiceUnknown
	}
	result := domain.ServiceOperational
	for _, value := range values {
		switch value.Status {
		case domain.ServiceOutage:
			return domain.ServiceOutage
		case domain.ServiceDegraded:
			result = domain.ServiceDegraded
		case domain.ServiceUnknown:
			if result == domain.ServiceOperational {
				result = domain.ServiceUnknown
			}
		}
	}
	return result
}

type ProductPatch struct {
	Name, Description *string
	Enabled           *bool
}

func (service *Service) UpdateProduct(ctx context.Context, id string, patch ProductPatch) (domain.Product, error) {
	product, err := service.products.GetProduct(ctx, id)
	if err != nil {
		return domain.Product{}, err
	}
	if patch.Name != nil {
		product.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.Description != nil {
		product.Description = *patch.Description
	}
	if patch.Enabled != nil {
		product.Enabled = *patch.Enabled
	}
	validated, err := domain.NewProduct(product.ID, product.Key, product.Name, product.Description, product.Enabled, product.CreatedAt)
	if err != nil {
		return domain.Product{}, err
	}
	validated.CreatedAt = product.CreatedAt
	validated.UpdatedAt = time.Now().UTC()
	if err := service.products.UpdateProduct(ctx, validated); err != nil {
		return domain.Product{}, err
	}
	return validated, nil
}

func (service *Service) ArchiveProduct(ctx context.Context, id string) error {
	return service.products.ArchiveProduct(ctx, id, time.Now().UTC())
}

type ProfileInput struct {
	Key, Name string
	Screens   []domain.Screen
}

func (service *Service) CreateProfile(ctx context.Context, input ProfileInput) (domain.ScreenProfile, error) {
	profile, err := domain.NewScreenProfile(service.ids.New(), input.Key, input.Name, input.Screens, time.Now())
	if err != nil {
		return domain.ScreenProfile{}, err
	}
	if err := service.profiles.CreateProfile(ctx, profile, service.ids); err != nil {
		return domain.ScreenProfile{}, err
	}
	return profile, nil
}
func (service *Service) GetProfile(ctx context.Context, id string) (domain.ScreenProfile, error) {
	return service.profiles.GetProfileByID(ctx, id)
}
func (service *Service) ListProfiles(ctx context.Context, limit int, cursor string) ([]domain.ScreenProfile, string, error) {
	return service.profiles.ListProfiles(ctx, normalizeLimit(limit), cursor)
}
func (service *Service) ReplaceProfile(ctx context.Context, id string, input ProfileInput) (domain.ScreenProfile, error) {
	current, err := service.profiles.GetProfileByID(ctx, id)
	if err != nil {
		return domain.ScreenProfile{}, err
	}
	if input.Key != current.Key {
		return domain.ScreenProfile{}, domain.ErrInvalid
	}
	replacement, err := domain.NewScreenProfile(id, current.Key, input.Name, input.Screens, current.CreatedAt)
	if err != nil {
		return domain.ScreenProfile{}, err
	}
	replacement.CreatedAt = current.CreatedAt
	replacement.UpdatedAt = time.Now().UTC()
	replacement.Revision = current.Revision + 1
	if err := service.profiles.ReplaceProfile(ctx, replacement, service.ids); err != nil {
		return domain.ScreenProfile{}, err
	}
	return replacement, nil
}
func (service *Service) DeleteProfile(ctx context.Context, id string) error {
	return service.profiles.DeleteProfile(ctx, id)
}

type DeviceProvision struct{ DeviceID, Name, ProfileID string }
type DeviceCredentials struct {
	Device                                   domain.Device
	DeviceSecret, MQTTUsername, MQTTPassword string
}

func (service *Service) CreateDevice(ctx context.Context, input DeviceProvision) (DeviceCredentials, error) {
	if _, err := service.profiles.GetProfileByID(ctx, input.ProfileID); err != nil {
		return DeviceCredentials{}, err
	}
	now := time.Now().UTC()
	device, err := domain.NewDevice(service.ids.New(), input.DeviceID, input.Name, input.ProfileID, now)
	if err != nil {
		return DeviceCredentials{}, err
	}
	secret, err := randomSecret()
	if err != nil {
		return DeviceCredentials{}, err
	}
	mqttPassword, err := randomSecret()
	if err != nil {
		return DeviceCredentials{}, err
	}
	secretHash, err := service.hasher.Hash(secret)
	if err != nil {
		return DeviceCredentials{}, err
	}
	mqttHash, err := service.hasher.Hash(mqttPassword)
	if err != nil {
		return DeviceCredentials{}, err
	}
	token := domain.DeviceToken{ID: service.ids.New(), DeviceID: device.ID, SecretHash: secretHash, Generation: 1, CreatedAt: now}
	username := "device-" + input.DeviceID
	if service.mqtt != nil {
		if err := service.mqtt.ProvisionDevice(ctx, device.DeviceID, username, mqttPassword); err != nil {
			return DeviceCredentials{}, err
		}
	}
	if err := service.devices.CreateWithToken(ctx, device, token, username, mqttHash); err != nil {
		if service.mqtt != nil {
			_ = service.mqtt.RevokeDevice(ctx, device.DeviceID, username, true)
		}
		return DeviceCredentials{}, err
	}
	return DeviceCredentials{Device: device, DeviceSecret: secret, MQTTUsername: username, MQTTPassword: mqttPassword}, nil
}

func (service *Service) GetDevice(ctx context.Context, id string) (domain.Device, error) {
	return service.devices.GetDeviceByID(ctx, id)
}
func (service *Service) ListDevices(ctx context.Context, limit int, cursor string) ([]domain.Device, string, error) {
	return service.devices.ListDevices(ctx, normalizeLimit(limit), cursor)
}
func (service *Service) RevokeDevice(ctx context.Context, id string) error {
	device, err := service.devices.GetDeviceByID(ctx, id)
	if err != nil {
		return err
	}
	username, err := service.devices.LatestMQTTUsername(ctx, device.ID)
	if err != nil {
		return err
	}
	if service.mqtt != nil {
		if err := service.mqtt.RevokeDevice(ctx, device.DeviceID, username, true); err != nil {
			return err
		}
	}
	return service.devices.Revoke(ctx, id, time.Now().UTC())
}

type DevicePatch struct {
	Name, ProfileID *string
	State           *domain.DeviceState
	OverridesJSON   []byte
	HasOverrides    bool
}

func (service *Service) UpdateDevice(ctx context.Context, id string, patch DevicePatch) (domain.Device, error) {
	device, err := service.devices.GetDeviceByID(ctx, id)
	if err != nil {
		return domain.Device{}, err
	}
	if patch.Name != nil {
		device.Name = strings.TrimSpace(*patch.Name)
	}
	if device.Name == "" || len(device.Name) > 100 {
		return domain.Device{}, domain.ErrInvalid
	}
	if patch.ProfileID != nil {
		if _, err := service.profiles.GetProfileByID(ctx, *patch.ProfileID); err != nil {
			return domain.Device{}, err
		}
		device.ProfileID = *patch.ProfileID
	}
	if patch.State != nil {
		if *patch.State != domain.DeviceOffline && *patch.State != domain.DeviceDisabled {
			return domain.Device{}, domain.ErrInvalid
		}
		device.State = *patch.State
	}
	if patch.HasOverrides {
		device.OverridesJSON = patch.OverridesJSON
	}
	device.UpdatedAt = time.Now().UTC()
	if err := service.devices.UpdateDevice(ctx, device); err != nil {
		return domain.Device{}, err
	}
	return device, nil
}

func (service *Service) RotateDeviceCredentials(ctx context.Context, id string) (DeviceCredentials, error) {
	device, err := service.devices.GetDeviceByID(ctx, id)
	if err != nil {
		return DeviceCredentials{}, err
	}
	current, err := service.devices.LatestToken(ctx, device.ID)
	if err != nil {
		return DeviceCredentials{}, err
	}
	secret, err := randomSecret()
	if err != nil {
		return DeviceCredentials{}, err
	}
	mqttPassword, err := randomSecret()
	if err != nil {
		return DeviceCredentials{}, err
	}
	secretHash, err := service.hasher.Hash(secret)
	if err != nil {
		return DeviceCredentials{}, err
	}
	mqttHash, err := service.hasher.Hash(mqttPassword)
	if err != nil {
		return DeviceCredentials{}, err
	}
	now := time.Now().UTC()
	token := domain.DeviceToken{ID: service.ids.New(), DeviceID: device.ID, SecretHash: secretHash, Generation: current.Generation + 1, CreatedAt: now}
	username := fmt.Sprintf("device-%s-g%d", device.DeviceID, token.Generation)
	currentUsername, err := service.devices.LatestMQTTUsername(ctx, device.ID)
	if err != nil {
		return DeviceCredentials{}, err
	}
	if service.mqtt != nil {
		if err := service.mqtt.ProvisionDevice(ctx, device.DeviceID, username, mqttPassword); err != nil {
			return DeviceCredentials{}, err
		}
	}
	if err := service.devices.RotateToken(ctx, token, username, mqttHash, now); err != nil {
		if service.mqtt != nil {
			_ = service.mqtt.RevokeDevice(ctx, device.DeviceID, username, false)
		}
		return DeviceCredentials{}, err
	}
	if service.mqtt != nil {
		if err := service.mqtt.RevokeDevice(ctx, device.DeviceID, currentUsername, false); err != nil {
			return DeviceCredentials{}, err
		}
	}
	return DeviceCredentials{Device: device, DeviceSecret: secret, MQTTUsername: username, MQTTPassword: mqttPassword}, nil
}

type AccessToken struct {
	Value        string
	ExpiresAt    time.Time
	User         *domain.User
	Device       *domain.Device
	MQTTUsername string
	RefreshToken string
}

func (service *Service) Login(ctx context.Context, email, password string) (AccessToken, error) {
	user, err := service.users.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || !service.hasher.Verify(password, user.PasswordHash) {
		return AccessToken{}, domain.ErrUnauthorized
	}
	token, expires, err := service.tokens.Issue(user.ID, UserAudience, string(user.Role), userTokenTTL)
	if err != nil {
		return AccessToken{}, err
	}
	refresh, err := randomSecret()
	if err != nil {
		return AccessToken{}, err
	}
	now := time.Now().UTC()
	session := domain.RefreshSession{ID: service.ids.New(), UserID: user.ID, FamilyID: service.ids.New(), TokenHash: hashToken(refresh), ExpiresAt: now.Add(7 * 24 * time.Hour), CreatedAt: now}
	if err := service.sessions.CreateRefreshSession(ctx, session); err != nil {
		return AccessToken{}, err
	}
	user.PasswordHash = ""
	return AccessToken{Value: token, ExpiresAt: expires, User: &user, RefreshToken: refresh}, nil
}

func (service *Service) Refresh(ctx context.Context, current string) (AccessToken, error) {
	if current == "" {
		return AccessToken{}, domain.ErrUnauthorized
	}
	nextRaw, err := randomSecret()
	if err != nil {
		return AccessToken{}, err
	}
	now := time.Now().UTC()
	next := domain.RefreshSession{ID: service.ids.New(), TokenHash: hashToken(nextRaw), ExpiresAt: now.Add(7 * 24 * time.Hour), CreatedAt: now}
	userID, err := service.sessions.RotateRefreshSession(ctx, hashToken(current), next, now)
	if err != nil {
		return AccessToken{}, err
	}
	user, err := service.users.GetUserByID(ctx, userID)
	if err != nil {
		return AccessToken{}, err
	}
	token, expires, err := service.tokens.Issue(user.ID, UserAudience, string(user.Role), userTokenTTL)
	if err != nil {
		return AccessToken{}, err
	}
	user.PasswordHash = ""
	return AccessToken{Value: token, ExpiresAt: expires, User: &user, RefreshToken: nextRaw}, nil
}

func (service *Service) Logout(ctx context.Context, refresh string) error {
	if refresh == "" {
		return nil
	}
	return service.sessions.RevokeRefreshSession(ctx, hashToken(refresh), time.Now().UTC())
}

func (service *Service) ExchangeDeviceSecret(ctx context.Context, deviceID, secret, firmware string) (AccessToken, error) {
	device, err := service.devices.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil || device.State == domain.DeviceRevoked || device.State == domain.DeviceDisabled {
		return AccessToken{}, domain.ErrUnauthorized
	}
	credential, err := service.devices.LatestToken(ctx, device.ID)
	if err != nil || credential.RevokedAt != nil || !service.hasher.Verify(secret, credential.SecretHash) {
		return AccessToken{}, domain.ErrUnauthorized
	}
	if err := service.devices.Touch(ctx, device.ID, firmware, time.Now().UTC()); err != nil {
		return AccessToken{}, err
	}
	mqttUsername, err := service.devices.LatestMQTTUsername(ctx, device.ID)
	if err != nil {
		return AccessToken{}, err
	}
	token, expires, err := service.tokens.Issue(device.ID, DeviceAudience, "device", service.deviceTokenTTL)
	if err != nil {
		return AccessToken{}, err
	}
	return AccessToken{Value: token, ExpiresAt: expires, Device: &device, MQTTUsername: mqttUsername}, nil
}

func (service *Service) Authenticate(raw, audience string) (ports.AccessClaims, error) {
	return service.tokens.Verify(raw, audience)
}
func (service *Service) CurrentUser(ctx context.Context, id string) (domain.User, error) {
	user, err := service.users.GetUserByID(ctx, id)
	user.PasswordHash = ""
	return user, err
}

func (service *Service) BootstrapAdmin(ctx context.Context, email, password string) error {
	if email == "" && password == "" {
		return nil
	}
	if email == "" || len(password) < 8 {
		return fmt.Errorf("bootstrap admin requires email and a password of at least 8 characters")
	}
	_, err := service.users.GetUserByEmail(ctx, email)
	if err == nil {
		return nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	hash, err := service.hasher.Hash(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return service.users.CreateUser(ctx, domain.User{ID: service.ids.New(), Email: strings.ToLower(email), PasswordHash: hash, Role: domain.RoleAdmin, CreatedAt: now, UpdatedAt: now})
}

func randomSecret() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func hashToken(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
func normalizeLimit(value int) int {
	if value < 1 {
		return 25
	}
	if value > 100 {
		return 100
	}
	return value
}
