package devices

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken_Length(t *testing.T) {
	tok, err := generateToken()
	require.NoError(t, err)
	// base64.RawURLEncoding of 20 bytes = ceil(20*8/6) = 27 chars.
	assert.Equal(t, 27, len(tok))
}

func TestGenerateToken_URLSafe(t *testing.T) {
	for i := 0; i < 100; i++ {
		tok, err := generateToken()
		require.NoError(t, err)
		for _, c := range tok {
			assert.True(t,
				(c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_',
				"token %q contains non-URL-safe character %q", tok, c)
		}
	}
}

func TestGenerateToken_TwiceReturnsDifferent(t *testing.T) {
	a, err := generateToken()
	require.NoError(t, err)
	b, err := generateToken()
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

func TestGenerateToken_UniqueOver1000(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		tok, err := generateToken()
		require.NoError(t, err)
		_, dup := seen[tok]
		assert.False(t, dup, "duplicate token generated at iteration %d: %q", i, tok)
		seen[tok] = struct{}{}
	}
}
