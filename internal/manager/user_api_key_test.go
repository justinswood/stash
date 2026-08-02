package manager

import (
	"strings"
	"testing"
)

// TestGenerateUserAPIKeyIsUnique guards the property the whole scheme rests on:
// two keys must never collide. key_hash is UNIQUE, so a collision would surface
// as a confusing insert failure rather than as the security problem it is.
func TestGenerateUserAPIKeyIsUnique(t *testing.T) {
	seen := make(map[string]bool)
	const iterations = 500

	for i := 0; i < iterations; i++ {
		plaintext, _, _, err := GenerateUserAPIKey()
		if err != nil {
			t.Fatalf("GenerateUserAPIKey: %v", err)
		}
		if seen[plaintext] {
			t.Fatalf("generated a duplicate key after %d iterations", i)
		}
		seen[plaintext] = true
	}
}

// TestGenerateUserAPIKeyPrefix checks that the stored prefix is a genuine
// substring of the plaintext and is short enough to be useless on its own. The
// prefix is shown in the UI forever, so if it ever grew to most of the key it
// would be leaking the credential it is meant to merely label.
func TestGenerateUserAPIKeyPrefix(t *testing.T) {
	plaintext, _, prefix, err := GenerateUserAPIKey()
	if err != nil {
		t.Fatalf("GenerateUserAPIKey: %v", err)
	}

	if !strings.HasPrefix(plaintext, prefix) {
		t.Errorf("prefix %q is not a prefix of the key", prefix)
	}
	if len(prefix) != apiKeyPrefixLen {
		t.Errorf("prefix length = %d, want %d", len(prefix), apiKeyPrefixLen)
	}
	if len(prefix) >= len(plaintext)/2 {
		t.Errorf("prefix %q is %d of %d chars — too much of the key is displayed",
			prefix, len(prefix), len(plaintext))
	}
	if !strings.HasPrefix(plaintext, apiKeyPrefixLabel) {
		t.Errorf("key %q lacks the %q label that makes a leaked key identifiable",
			plaintext, apiKeyPrefixLabel)
	}
}

// TestHashUserAPIKey checks the hash is deterministic (lookup depends on it) and
// that distinct keys hash distinctly.
func TestHashUserAPIKey(t *testing.T) {
	const a = "stash_aaaaaaaaaaaaaaaa"
	const b = "stash_bbbbbbbbbbbbbbbb"

	h1 := HashUserAPIKey(a)
	h2 := HashUserAPIKey(a)
	if string(h1) != string(h2) {
		t.Error("hash is not deterministic — key lookup would fail intermittently")
	}
	if string(HashUserAPIKey(a)) == string(HashUserAPIKey(b)) {
		t.Error("distinct keys produced the same hash")
	}
	if strings.Contains(string(h1), a) {
		t.Error("hash contains the plaintext")
	}
}

// TestGeneratedKeyHashMatches ties the two together: the hash returned at
// creation must be the one FindByKeyHash will later compute from the plaintext
// the user presents. If these ever diverge, every issued key silently stops
// authenticating.
func TestGeneratedKeyHashMatches(t *testing.T) {
	plaintext, hash, _, err := GenerateUserAPIKey()
	if err != nil {
		t.Fatalf("GenerateUserAPIKey: %v", err)
	}
	if string(hash) != string(HashUserAPIKey(plaintext)) {
		t.Error("hash returned at creation does not match HashUserAPIKey(plaintext); " +
			"issued keys would never resolve")
	}
}
