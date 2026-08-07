package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
	"github.com/golang-jwt/jwt/v5"
)

type JWT struct {
	issuer  string
	keyID   string
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	ids     ports.IDGenerator
}

type claims struct {
	Role string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

func LoadOrCreateJWT(path, issuer string, ids ports.IDGenerator) (*JWT, error) {
	private, err := readPrivateKey(path)
	if os.IsNotExist(err) {
		_, private, err = ed25519.GenerateKey(rand.Reader)
		if err == nil {
			err = writePrivateKey(path, private)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("load signing key: %w", err)
	}
	public := private.Public().(ed25519.PublicKey)
	digest := sha256.Sum256(public)
	return &JWT{issuer: issuer, keyID: hex.EncodeToString(digest[:8]), private: private, public: public, ids: ids}, nil
}

func (service *JWT) Issue(subject, audience, role string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	value := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims{Role: role, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: service.issuer, Subject: subject, Audience: jwt.ClaimStrings{audience}, ID: service.ids.New(),
		IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)), ExpiresAt: jwt.NewNumericDate(expiresAt),
	}})
	value.Header["kid"] = service.keyID
	signed, err := value.SignedString(service.private)
	return signed, expiresAt, err
}

func (service *JWT) Verify(raw, audience string) (ports.AccessClaims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodEdDSA || token.Header["kid"] != service.keyID {
			return nil, domain.ErrUnauthorized
		}
		return service.public, nil
	}, jwt.WithAudience(audience), jwt.WithIssuer(service.issuer), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !parsed.Valid {
		return ports.AccessClaims{}, domain.ErrUnauthorized
	}
	value, ok := parsed.Claims.(*claims)
	if !ok || value.ExpiresAt == nil {
		return ports.AccessClaims{}, domain.ErrUnauthorized
	}
	return ports.AccessClaims{Subject: value.Subject, Audience: audience, Role: value.Role, TokenID: value.ID, ExpiresAt: value.ExpiresAt.Time}, nil
}

func readPrivateKey(path string) (ed25519.PrivateKey, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(content)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	private, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519")
	}
	return private, nil
}

func writePrivateKey(path string, key ed25519.PrivateKey) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
