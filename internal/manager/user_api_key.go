package manager

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

const (
	apiKeyBytes = 32
	// apiKeyPrefixLabel makes a leaked key identifiable at a glance (and to
	// secret scanners) without revealing anything about its owner.
	apiKeyPrefixLabel = "stash_"
	// apiKeyPrefixLen is how much of the plaintext is stored so the UI can tell
	// keys apart in a list.
	apiKeyPrefixLen = 12
	// apiKeyLastUsedThrottle bounds how often a key's last_used_at is written.
	// Without it every API request would issue a write against SQLite's single
	// writer connection purely for bookkeeping.
	apiKeyLastUsedThrottle = time.Minute
)

// Per-user keys are deliberately opaque random tokens stored as hashes, not
// JWTs like the config key in apikey.go. A JWT cannot be revoked without
// maintaining a blocklist, and revocability is the entire point of these.

// HashUserAPIKey returns the SHA-256 of a plaintext API key. Only this is stored.
func HashUserAPIKey(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

// GenerateUserAPIKey returns a new random API key: the plaintext (shown to the
// user exactly once), its hash, and the display prefix.
func GenerateUserAPIKey() (plaintext string, hash []byte, prefix string, err error) {
	buf := make([]byte, apiKeyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, "", fmt.Errorf("generating api key: %w", err)
	}
	plaintext = apiKeyPrefixLabel + base64.RawURLEncoding.EncodeToString(buf)
	prefix = plaintext[:apiKeyPrefixLen]
	return plaintext, HashUserAPIKey(plaintext), prefix, nil
}

// ResolveAPIKey resolves a plaintext API key to the username of its owning
// account. ok is false when the key is unknown or its owner is disabled — which
// is what makes a key revocable by disabling the account, with no session or
// cache to wait out.
func (s *Manager) ResolveAPIKey(plaintext string) (string, bool) {
	if plaintext == "" {
		return "", false
	}
	if s.Database == nil || s.Database.Ready() != nil {
		return "", false
	}

	ctx := context.Background()
	hash := HashUserAPIKey(plaintext)

	var (
		username string
		ok       bool
		keyID    int
		lastUsed *time.Time
	)

	err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		k, err := s.Repository.UserAPIKey.FindByKeyHash(ctx, hash)
		if err != nil || k == nil {
			return err
		}
		u, err := s.Repository.User.Find(ctx, k.UserID)
		if err != nil || u == nil {
			return err
		}
		if u.Disabled {
			return nil
		}
		username, ok, keyID, lastUsed = u.Username, true, k.ID, k.LastUsedAt
		return nil
	})
	if err != nil {
		logger.Errorf("error resolving api key: %v", err)
		return "", false
	}
	if !ok {
		return "", false
	}

	s.touchAPIKey(ctx, keyID, lastUsed)
	return username, true
}

// touchAPIKey records last-used, throttled and best effort — a failure here must
// never fail the request that is otherwise correctly authenticated.
func (s *Manager) touchAPIKey(ctx context.Context, keyID int, lastUsed *time.Time) {
	now := time.Now()
	if lastUsed != nil && now.Sub(*lastUsed) < apiKeyLastUsedThrottle {
		return
	}
	if err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		return s.Repository.UserAPIKey.TouchLastUsed(ctx, keyID, now)
	}); err != nil {
		logger.Debugf("could not record api key last-used: %v", err)
	}
}

// CreateUserAPIKey issues a new key for a user and returns the plaintext, which
// is not recoverable afterwards.
func (s *Manager) CreateUserAPIKey(ctx context.Context, userID int, name string) (*models.UserAPIKey, string, error) {
	plaintext, hash, prefix, err := GenerateUserAPIKey()
	if err != nil {
		return nil, "", err
	}

	k := &models.UserAPIKey{
		UserID:  userID,
		Name:    name,
		KeyHash: hash,
		Prefix:  prefix,
	}
	if err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		return s.Repository.UserAPIKey.Create(ctx, k)
	}); err != nil {
		return nil, "", err
	}
	return k, plaintext, nil
}
