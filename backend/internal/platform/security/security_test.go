package security

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
)

func TestArgon2ID(t *testing.T) {
	hasher := Argon2ID{}
	hash, err := hasher.Hash("a sufficiently long device secret")
	if err != nil {
		t.Fatal(err)
	}
	if !hasher.Verify("a sufficiently long device secret", hash) {
		t.Fatal("valid secret rejected")
	}
	if hasher.Verify("wrong secret", hash) {
		t.Fatal("invalid secret accepted")
	}
}

func TestJWTAudience(t *testing.T) {
	service, err := LoadOrCreateJWT(filepath.Join(t.TempDir(), "jwt.pem"), "desk-monitor", ULIDGenerator{})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := service.Issue("device-id", "desk-monitor-device", "", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Verify(token, "desk-monitor-device"); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if _, err := service.Verify(token, "desk-monitor-user"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("wrong audience error = %v", err)
	}
}
