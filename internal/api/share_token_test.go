package api

import (
	"crypto/sha256"
	"strings"
	"testing"
)

// Share tokens are the entire authentication for a share link: anyone holding
// one gets the content, with no login. Only the SHA-256 hash is stored, so a
// database read cannot recover a working link — and equally, a bug in the
// hashing or the acceptance rule cannot be noticed by looking at the database.

func TestGenerateShareTokenIsUnpredictable(t *testing.T) {
	const runs = 200

	seen := make(map[string]bool, runs)
	for i := 0; i < runs; i++ {
		plaintext, hash, err := generateShareToken()
		if err != nil {
			t.Fatalf("generateShareToken: %v", err)
		}

		if seen[plaintext] {
			t.Fatalf("generateShareToken produced a duplicate token after %d runs — "+
				"tokens are the only credential a share link has", i+1)
		}
		seen[plaintext] = true

		// 32 random bytes in unpadded base64url is 43 characters. A shorter
		// token would still pass looksLikeShareToken at 32, so the length is
		// asserted here rather than inferred from the acceptance rule.
		if len(plaintext) != 43 {
			t.Errorf("token length = %d, want 43 (32 random bytes, base64url unpadded)", len(plaintext))
		}

		if len(hash) != sha256.Size {
			t.Errorf("hash length = %d, want %d", len(hash), sha256.Size)
		}

		// The stored hash must not contain the token. This is what makes a
		// database leak non-exploitable.
		if strings.Contains(string(hash), plaintext) {
			t.Error("stored hash contains the plaintext token")
		}
	}
}

func TestHashShareTokenMatchesGeneration(t *testing.T) {
	plaintext, hash, err := generateShareToken()
	if err != nil {
		t.Fatalf("generateShareToken: %v", err)
	}

	// ShareCtx looks a link up by hashing the token from the URL. If these two
	// disagreed, every share link would 404 immediately after being created.
	got := HashShareToken(plaintext)
	if string(got) != string(hash) {
		t.Error("HashShareToken disagrees with generateShareToken — " +
			"every issued link would fail lookup")
	}

	// A different token must not collide.
	other := HashShareToken(plaintext + "x")
	if string(other) == string(hash) {
		t.Error("distinct tokens hashed to the same value")
	}
}

// looksLikeShareToken is the cheap gate in front of the database lookup. It
// must accept everything generateShareToken can produce — a false negative here
// rejects a legitimate link outright.
func TestLooksLikeShareTokenAcceptsGeneratedTokens(t *testing.T) {
	for i := 0; i < 200; i++ {
		plaintext, _, err := generateShareToken()
		if err != nil {
			t.Fatalf("generateShareToken: %v", err)
		}
		if !looksLikeShareToken(plaintext) {
			t.Fatalf("looksLikeShareToken rejected a generated token %q — "+
				"legitimate share links would be refused before lookup", plaintext)
		}
	}
}

func TestLooksLikeShareTokenRejectsMalformed(t *testing.T) {
	valid := strings.Repeat("a", 43)

	tests := []struct {
		name  string
		token string
		want  bool
		why   string
	}{
		{"empty", "", false, "an empty token must never reach the database"},
		{"too short", strings.Repeat("a", 31), false, "below the minimum length"},
		{"minimum length", strings.Repeat("a", 32), true, "32 is the documented minimum"},
		{"maximum length", strings.Repeat("a", 128), true, "128 is the documented maximum"},
		{"too long", strings.Repeat("a", 129), false, "above the maximum length"},
		{"valid generated shape", valid, true, "43 base64url chars is what generation emits"},

		// The character rule is what keeps hostile input out of the lookup
		// path entirely.
		{"sql quote", strings.Repeat("a", 42) + "'", false, "quotes must be refused"},
		{"path traversal", strings.Repeat("a", 40) + "/../", false, "slashes must be refused"},
		{"null byte", strings.Repeat("a", 42) + "\x00", false, "control characters must be refused"},
		{"percent encoding", strings.Repeat("a", 40) + "%2e%2e", false, "percent signs must be refused"},
		{"whitespace", strings.Repeat("a", 42) + " ", false, "whitespace must be refused"},
		{"base64 standard padding", strings.Repeat("a", 42) + "=", false, "padding is not part of base64url unpadded"},
		{"base64 standard plus", strings.Repeat("a", 42) + "+", false, "+ is standard base64, not base64url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeShareToken(tt.token); got != tt.want {
				t.Errorf("looksLikeShareToken(%q) = %v, want %v — %s", tt.token, got, tt.want, tt.why)
			}
		})
	}
}
