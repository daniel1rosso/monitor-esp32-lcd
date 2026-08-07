package persistence

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/config"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*/*.sql
var migrationFiles embed.FS

type Store struct{ db *gorm.DB }

func Open(ctx context.Context, cfg config.Storage) (*Store, error) {
	var dialector gorm.Dialector
	switch cfg.Driver {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(cfg.SQLite.Path), 0o750); err != nil {
			return nil, fmt.Errorf("create sqlite directory: %w", err)
		}
		dialector = sqlite.Open(cfg.SQLite.Path)
	case "postgres":
		dialector = postgres.Open(os.Getenv("DATABASE_URL"))
	default:
		return nil, fmt.Errorf("unsupported storage driver %q", cfg.Driver)
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", cfg.Driver, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access sql database: %w", err)
	}
	if cfg.Driver == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
		busyMillis := cfg.SQLite.BusyTimeout.Milliseconds()
		if err := db.WithContext(ctx).Exec("PRAGMA busy_timeout = " + strconv.FormatInt(busyMillis, 10)).Error; err != nil {
			return nil, fmt.Errorf("configure sqlite busy timeout: %w", err)
		}
		if cfg.SQLite.WAL {
			if err := db.WithContext(ctx).Exec("PRAGMA journal_mode = WAL").Error; err != nil {
				return nil, fmt.Errorf("enable sqlite WAL: %w", err)
			}
		}
		if err := db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}

func (s *Store) Migrate(ctx context.Context, driver string) error {
	directory := "migrations/" + driver
	entries, err := fs.ReadDir(migrationFiles, directory)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if driver == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(7335266274711)").Error; err != nil {
				return fmt.Errorf("lock migrations: %w", err)
			}
		}
		if err := tx.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY, applied_at TIMESTAMP NOT NULL)").Error; err != nil {
			return err
		}
		for _, entry := range entries {
			versionText := strings.SplitN(entry.Name(), "_", 2)[0]
			version, err := strconv.ParseInt(versionText, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid migration filename %q", entry.Name())
			}
			var count int64
			if err := tx.Table("schema_migrations").Where("version = ?", version).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			content, err := migrationFiles.ReadFile(directory + "/" + entry.Name())
			if err != nil {
				return err
			}
			if err := tx.Exec(string(content)).Error; err != nil {
				return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
			}
			if err := tx.Exec("INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)", version, time.Now().UTC()).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type productRow struct {
	ID, Key, Name, Description string
	Enabled                    bool
	CreatedAt, UpdatedAt       time.Time
	ArchivedAt                 *time.Time
}

func (productRow) TableName() string { return "products" }

func productDomain(row productRow) domain.Product {
	return domain.Product{ID: row.ID, Key: row.Key, Name: row.Name, Description: row.Description, Enabled: row.Enabled, Health: domain.ServiceUnknown, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ArchivedAt: row.ArchivedAt}
}

func productModel(value domain.Product) productRow {
	return productRow{ID: value.ID, Key: value.Key, Name: value.Name, Description: value.Description, Enabled: value.Enabled, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, ArchivedAt: value.ArchivedAt}
}

func (s *Store) CreateProduct(ctx context.Context, product domain.Product) error {
	return translate(s.db.WithContext(ctx).Create(productModel(product)).Error)
}

func (s *Store) GetProduct(ctx context.Context, id string) (domain.Product, error) {
	var row productRow
	err := s.db.WithContext(ctx).Where("id = ? AND archived_at IS NULL", id).First(&row).Error
	return productDomain(row), translate(err)
}

func (s *Store) GetProductByKey(ctx context.Context, key string) (domain.Product, error) {
	var row productRow
	err := s.db.WithContext(ctx).Where("key = ? AND archived_at IS NULL", key).First(&row).Error
	return productDomain(row), translate(err)
}

func (s *Store) ListProducts(ctx context.Context, limit int, cursor string) ([]domain.Product, string, error) {
	query := s.db.WithContext(ctx).Where("archived_at IS NULL")
	if cursor != "" {
		query = query.Where("id > ?", cursor)
	}
	var rows []productRow
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", translate(err)
	}
	next := ""
	if len(rows) > limit {
		next = rows[limit-1].ID
		rows = rows[:limit]
	}
	products := make([]domain.Product, len(rows))
	for index, row := range rows {
		products[index] = productDomain(row)
	}
	return products, next, nil
}

func (s *Store) UpdateProduct(ctx context.Context, product domain.Product) error {
	result := s.db.WithContext(ctx).Model(&productRow{}).Where("id = ? AND archived_at IS NULL", product.ID).Updates(map[string]any{"name": product.Name, "description": product.Description, "enabled": product.Enabled, "updated_at": product.UpdatedAt})
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ArchiveProduct(ctx context.Context, id string, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&productRow{}).Where("id = ? AND archived_at IS NULL", id).Updates(map[string]any{"archived_at": at, "updated_at": at, "enabled": false})
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type profileRow struct {
	ID, Key, Name        string
	Revision             int
	CreatedAt, UpdatedAt time.Time
}

func (profileRow) TableName() string { return "screen_profiles" }

type screenRow struct {
	ID, ProfileID        string
	Position             int
	ScreenKey, Type      string
	Priority, DurationMS int
	Enabled              bool
	ConfigJSON           string
}

func (screenRow) TableName() string { return "profile_screens" }

func (s *Store) GetProfile(ctx context.Context, field, value string) (domain.ScreenProfile, error) {
	var row profileRow
	if err := s.db.WithContext(ctx).Where(field+" = ?", value).First(&row).Error; err != nil {
		return domain.ScreenProfile{}, translate(err)
	}
	var screens []screenRow
	if err := s.db.WithContext(ctx).Where("profile_id = ?", row.ID).Order("position ASC").Find(&screens).Error; err != nil {
		return domain.ScreenProfile{}, translate(err)
	}
	result := domain.ScreenProfile{ID: row.ID, Key: row.Key, Name: row.Name, Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	for _, screen := range screens {
		result.Screens = append(result.Screens, domain.Screen{ID: screen.ScreenKey, Type: screen.Type, Priority: screen.Priority, DurationMS: screen.DurationMS, Enabled: screen.Enabled, ConfigJSON: []byte(screen.ConfigJSON)})
	}
	return result, nil
}

func (s *Store) GetProfileByID(ctx context.Context, id string) (domain.ScreenProfile, error) {
	return s.GetProfile(ctx, "id", id)
}
func (s *Store) GetProfileByKey(ctx context.Context, key string) (domain.ScreenProfile, error) {
	return s.GetProfile(ctx, "key", key)
}

func (s *Store) ListProfiles(ctx context.Context, limit int, cursor string) ([]domain.ScreenProfile, string, error) {
	query := s.db.WithContext(ctx)
	if cursor != "" {
		query = query.Where("id > ?", cursor)
	}
	var rows []profileRow
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", translate(err)
	}
	next := ""
	if len(rows) > limit {
		next = rows[limit-1].ID
		rows = rows[:limit]
	}
	items := make([]domain.ScreenProfile, 0, len(rows))
	for _, row := range rows {
		item, err := s.GetProfileByID(ctx, row.ID)
		if err != nil {
			return nil, "", err
		}
		items = append(items, item)
	}
	return items, next, nil
}

func (s *Store) CreateProfile(ctx context.Context, profile domain.ScreenProfile, ids ports.IDGenerator) error {
	return translate(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(profileRow{ID: profile.ID, Key: profile.Key, Name: profile.Name, Revision: profile.Revision, CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt}).Error; err != nil {
			return err
		}
		return createScreens(tx, profile.ID, profile.Screens, ids)
	}))
}

func (s *Store) ReplaceProfile(ctx context.Context, profile domain.ScreenProfile, ids ports.IDGenerator) error {
	return translate(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&profileRow{}).Where("id = ?", profile.ID).Updates(map[string]any{"name": profile.Name, "revision": profile.Revision, "updated_at": profile.UpdatedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		if err := tx.Where("profile_id = ?", profile.ID).Delete(&screenRow{}).Error; err != nil {
			return err
		}
		return createScreens(tx, profile.ID, profile.Screens, ids)
	}))
}

func (s *Store) DeleteProfile(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Delete(&profileRow{}, "id = ?", id)
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func createScreens(tx *gorm.DB, profileID string, screens []domain.Screen, ids ports.IDGenerator) error {
	for position, screen := range screens {
		raw := screen.ConfigJSON
		if len(raw) == 0 {
			raw = []byte("{}")
		}
		if !json.Valid(raw) {
			return domain.ErrInvalid
		}
		if err := tx.Create(screenRow{ID: ids.New(), ProfileID: profileID, Position: position, ScreenKey: screen.ID, Type: screen.Type, Priority: screen.Priority, DurationMS: screen.DurationMS, Enabled: screen.Enabled, ConfigJSON: string(raw)}).Error; err != nil {
			return err
		}
	}
	return nil
}

type deviceRow struct {
	ID, DeviceID, Name, ProfileID, State string
	FirmwareVersion                      *string
	LastSeenAt                           *time.Time
	OverridesJSON                        string
	CreatedAt, UpdatedAt                 time.Time
}

func (deviceRow) TableName() string { return "devices" }

type tokenRow struct {
	ID, DeviceID, SecretHash string
	Generation               int
	ExpiresAt, RevokedAt     *time.Time
	CreatedAt                time.Time
}

func (tokenRow) TableName() string { return "device_tokens" }

type mqttRow struct {
	ID, DeviceID, Username, PasswordHash string
	Generation                           int
	RevokedAt                            *time.Time
	CreatedAt                            time.Time
}

func (mqttRow) TableName() string { return "device_mqtt_credentials" }

func deviceDomain(row deviceRow) domain.Device {
	return domain.Device{ID: row.ID, DeviceID: row.DeviceID, Name: row.Name, ProfileID: row.ProfileID, State: domain.DeviceState(row.State), FirmwareVersion: row.FirmwareVersion, LastSeenAt: row.LastSeenAt, OverridesJSON: []byte(row.OverridesJSON), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (s *Store) CreateWithToken(ctx context.Context, device domain.Device, token domain.DeviceToken, mqttUsername, mqttPasswordHash string) error {
	return translate(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(deviceRow{ID: device.ID, DeviceID: device.DeviceID, Name: device.Name, ProfileID: device.ProfileID, State: string(device.State), OverridesJSON: "{}", CreatedAt: device.CreatedAt, UpdatedAt: device.UpdatedAt}).Error; err != nil {
			return err
		}
		if err := tx.Create(tokenRow{ID: token.ID, DeviceID: token.DeviceID, SecretHash: token.SecretHash, Generation: token.Generation, ExpiresAt: token.ExpiresAt, CreatedAt: token.CreatedAt}).Error; err != nil {
			return err
		}
		return tx.Create(mqttRow{ID: token.ID, DeviceID: device.ID, Username: mqttUsername, PasswordHash: mqttPasswordHash, Generation: token.Generation, CreatedAt: token.CreatedAt}).Error
	}))
}

func (s *Store) GetDevice(ctx context.Context, field, value string) (domain.Device, error) {
	var row deviceRow
	err := s.db.WithContext(ctx).Where(field+" = ?", value).First(&row).Error
	return deviceDomain(row), translate(err)
}
func (s *Store) GetDeviceByID(ctx context.Context, id string) (domain.Device, error) {
	return s.GetDevice(ctx, "id", id)
}
func (s *Store) GetDeviceByDeviceID(ctx context.Context, id string) (domain.Device, error) {
	return s.GetDevice(ctx, "device_id", id)
}

func (s *Store) ListDevices(ctx context.Context, limit int, cursor string) ([]domain.Device, string, error) {
	query := s.db.WithContext(ctx)
	if cursor != "" {
		query = query.Where("id > ?", cursor)
	}
	var rows []deviceRow
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", translate(err)
	}
	next := ""
	if len(rows) > limit {
		next = rows[limit-1].ID
		rows = rows[:limit]
	}
	items := make([]domain.Device, len(rows))
	for index, row := range rows {
		items[index] = deviceDomain(row)
	}
	return items, next, nil
}

func (s *Store) UpdateDevice(ctx context.Context, device domain.Device) error {
	overrides := device.OverridesJSON
	if len(overrides) == 0 {
		overrides = []byte("{}")
	} else if !json.Valid(overrides) {
		return domain.ErrInvalid
	}
	result := s.db.WithContext(ctx).Model(&deviceRow{}).Where("id = ?", device.ID).Updates(map[string]any{"name": device.Name, "profile_id": device.ProfileID, "state": string(device.State), "overrides_json": string(overrides), "updated_at": device.UpdatedAt})
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) LatestToken(ctx context.Context, deviceID string) (domain.DeviceToken, error) {
	var row tokenRow
	err := s.db.WithContext(ctx).Where("device_id = ? AND revoked_at IS NULL", deviceID).Order("generation DESC").First(&row).Error
	return domain.DeviceToken{ID: row.ID, DeviceID: row.DeviceID, SecretHash: row.SecretHash, Generation: row.Generation, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt, CreatedAt: row.CreatedAt}, translate(err)
}

func (s *Store) LatestMQTTUsername(ctx context.Context, deviceID string) (string, error) {
	var row mqttRow
	err := s.db.WithContext(ctx).Where("device_id = ? AND revoked_at IS NULL", deviceID).Order("generation DESC").First(&row).Error
	return row.Username, translate(err)
}

func (s *Store) RotateToken(ctx context.Context, token domain.DeviceToken, mqttUsername, mqttPasswordHash string, at time.Time) error {
	return translate(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&tokenRow{}).Where("device_id = ? AND revoked_at IS NULL", token.DeviceID).Update("revoked_at", at).Error; err != nil {
			return err
		}
		if err := tx.Model(&mqttRow{}).Where("device_id = ? AND revoked_at IS NULL", token.DeviceID).Update("revoked_at", at).Error; err != nil {
			return err
		}
		if err := tx.Create(tokenRow{ID: token.ID, DeviceID: token.DeviceID, SecretHash: token.SecretHash, Generation: token.Generation, CreatedAt: token.CreatedAt}).Error; err != nil {
			return err
		}
		return tx.Create(mqttRow{ID: token.ID, DeviceID: token.DeviceID, Username: mqttUsername, PasswordHash: mqttPasswordHash, Generation: token.Generation, CreatedAt: token.CreatedAt}).Error
	}))
}

func (s *Store) Touch(ctx context.Context, id, firmware string, at time.Time) error {
	return translate(s.db.WithContext(ctx).Model(&deviceRow{}).Where("id = ?", id).Updates(map[string]any{"firmware_version": firmware, "last_seen_at": at, "state": string(domain.DeviceOnline), "updated_at": at}).Error)
}
func (s *Store) Revoke(ctx context.Context, id string, at time.Time) error {
	return translate(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&deviceRow{}).Where("id = ?", id).Updates(map[string]any{"state": string(domain.DeviceRevoked), "updated_at": at}).Error; err != nil {
			return err
		}
		return tx.Model(&tokenRow{}).Where("device_id = ? AND revoked_at IS NULL", id).Update("revoked_at", at).Error
	}))
}

type userRow struct {
	ID, Email, PasswordHash, Role string
	CreatedAt, UpdatedAt          time.Time
}

func (userRow) TableName() string { return "users" }
func (s *Store) GetUser(ctx context.Context, field, value string) (domain.User, error) {
	var row userRow
	err := s.db.WithContext(ctx).Where(field+" = ?", value).First(&row).Error
	return domain.User{ID: row.ID, Email: row.Email, PasswordHash: row.PasswordHash, Role: domain.Role(row.Role), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, translate(err)
}
func (s *Store) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	return s.GetUser(ctx, "email", strings.ToLower(email))
}
func (s *Store) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	return s.GetUser(ctx, "id", id)
}
func (s *Store) CreateUser(ctx context.Context, user domain.User) error {
	return translate(s.db.WithContext(ctx).Create(userRow{ID: user.ID, Email: strings.ToLower(user.Email), PasswordHash: user.PasswordHash, Role: string(user.Role), CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}).Error)
}

type refreshRow struct {
	ID, UserID, FamilyID, TokenHash string
	ExpiresAt                       time.Time
	RevokedAt                       *time.Time
	CreatedAt                       time.Time
}

func (refreshRow) TableName() string { return "refresh_sessions" }

func (s *Store) CreateRefreshSession(ctx context.Context, session domain.RefreshSession) error {
	row := refreshRow{ID: session.ID, UserID: session.UserID, FamilyID: session.FamilyID, TokenHash: session.TokenHash, ExpiresAt: session.ExpiresAt, CreatedAt: session.CreatedAt}
	return translate(s.db.WithContext(ctx).Create(row).Error)
}

func (s *Store) RotateRefreshSession(ctx context.Context, currentHash string, next domain.RefreshSession, at time.Time) (string, error) {
	var userID string
	var reused bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current refreshRow
		if err := tx.Where("token_hash = ?", currentHash).First(&current).Error; err != nil {
			return err
		}
		userID = current.UserID
		if current.RevokedAt != nil {
			if err := tx.Model(&refreshRow{}).Where("family_id = ? AND revoked_at IS NULL", current.FamilyID).Update("revoked_at", at).Error; err != nil {
				return err
			}
			reused = true
			return nil
		}
		if !current.ExpiresAt.After(at) {
			return domain.ErrUnauthorized
		}
		result := tx.Model(&refreshRow{}).Where("id = ? AND revoked_at IS NULL", current.ID).Update("revoked_at", at)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrUnauthorized
		}
		next.UserID = current.UserID
		next.FamilyID = current.FamilyID
		return tx.Create(refreshRow{ID: next.ID, UserID: next.UserID, FamilyID: next.FamilyID, TokenHash: next.TokenHash, ExpiresAt: next.ExpiresAt, CreatedAt: next.CreatedAt}).Error
	})
	if err == nil && reused {
		return "", domain.ErrUnauthorized
	}
	return userID, translate(err)
}

func (s *Store) RevokeRefreshSession(ctx context.Context, hash string, at time.Time) error {
	return translate(s.db.WithContext(ctx).Model(&refreshRow{}).Where("token_hash = ? AND revoked_at IS NULL", hash).Update("revoked_at", at).Error)
}

func (s *Store) Seed(ctx context.Context, cfg config.Config, ids ports.IDGenerator, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range cfg.Products {
			var count int64
			if err := tx.Model(&productRow{}).Where("key = ?", item.Key).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := tx.Create(productRow{ID: ids.New(), Key: item.Key, Name: item.Name, Enabled: item.Enabled, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
					return err
				}
			}
		}
		for _, profile := range cfg.Profiles {
			var count int64
			if err := tx.Model(&profileRow{}).Where("key = ?", profile.Key).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			profileID := ids.New()
			if err := tx.Create(profileRow{ID: profileID, Key: profile.Key, Name: profile.Name, Revision: 1, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				return err
			}
			for position, screen := range profile.Screens {
				raw, err := json.Marshal(screen.Config)
				if err != nil {
					return err
				}
				if string(raw) == "null" {
					raw = []byte("{}")
				}
				if err := tx.Create(screenRow{ID: ids.New(), ProfileID: profileID, Position: position, ScreenKey: screen.ID, Type: screen.Type, Priority: screen.Priority, DurationMS: int(screen.Duration.Milliseconds()), Enabled: screen.Enabled, ConfigJSON: string(raw)}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique constraint") || strings.Contains(strings.ToLower(err.Error()), "foreign key constraint") {
		return domain.ErrConflict
	}
	return err
}

var _ ports.HealthChecker = (*Store)(nil)
var _ ports.ProductRepository = (*Store)(nil)
var _ ports.ProfileRepository = (*Store)(nil)
var _ ports.DeviceRepository = (*Store)(nil)
var _ ports.UserRepository = (*Store)(nil)
var _ ports.SessionRepository = (*Store)(nil)
