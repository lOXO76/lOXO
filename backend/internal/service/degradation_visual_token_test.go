//go:build unit

package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestDegradationVisualToken_IssueAndSharedSecretVerify(t *testing.T) {
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "visual-token-test-secret"}}
	now := time.Date(2026, time.October, 30, 9, 0, 0, 0, time.UTC)
	svc := NewDegradationVisualTokenService(cfg)
	svc.now = func() time.Time { return now }
	token, err := svc.Issue(123)
	require.NoError(t, err)
	require.Equal(t, now.Add(15*time.Minute), token.ExpiresAt)
	payload := "v1.123." + strconv.FormatInt(token.ExpiresAt.Unix(), 10)
	keyMAC := hmac.New(sha256.New, []byte(cfg.JWT.Secret))
	_, _ = keyMAC.Write([]byte("sub2api:degradation-visual-render:v1"))
	mac := hmac.New(sha256.New, keyMAC.Sum(nil))
	_, _ = mac.Write([]byte(payload))
	require.Equal(t, payload+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), token.Token)
	otherInstance := NewDegradationVisualTokenService(cfg)
	otherInstance.now = svc.now
	id, err := otherInstance.Verify(token.Token)
	require.NoError(t, err)
	require.Equal(t, int64(123), id)
}

func TestDegradationVisualToken_RejectsTamperingAndCrossRunReuse(t *testing.T) {
	svc := NewDegradationVisualTokenService(&config.Config{JWT: config.JWTConfig{Secret: "test-secret"}})
	token, err := svc.Issue(123)
	require.NoError(t, err)
	parts := strings.Split(token.Token, ".")
	for _, badToken := range []string{
		"", "v1.1", token.Token + ".extra", strings.Repeat("x", 129),
		"v2." + strings.Join(parts[1:], "."),
		"v1.456." + strings.Join(parts[2:], "."),
		"v1.0." + strings.Join(parts[2:], "."),
		"v1.-1." + strings.Join(parts[2:], "."),
		"v1.+123." + strings.Join(parts[2:], "."),
		"v1.0123." + strings.Join(parts[2:], "."),
		"v1.9223372036854775808." + strings.Join(parts[2:], "."),
		"v1.123.bad." + parts[3],
		"v1.123." + strconv.FormatInt(token.ExpiresAt.Unix()+3600, 10) + "." + parts[3],
		strings.Join(parts[:3], ".") + ".invalid!",
		strings.Join(parts[:3], ".") + "." + base64.RawURLEncoding.EncodeToString(make([]byte, sha256.Size)),
	} {
		id, verifyErr := svc.Verify(badToken)
		require.ErrorIs(t, verifyErr, errDegradationVisualToken, "token=%q", badToken)
		require.Zero(t, id)
	}
	otherSecret := NewDegradationVisualTokenService(&config.Config{JWT: config.JWTConfig{Secret: "other-secret"}})
	_, err = otherSecret.Verify(token.Token)
	require.Error(t, err)
}

func TestDegradationVisualToken_ExpiryBoundary(t *testing.T) {
	svc := NewDegradationVisualTokenService(&config.Config{JWT: config.JWTConfig{Secret: "test-secret"}})
	now := time.Now().UTC().Truncate(time.Second)
	svc.now = func() time.Time { return now }
	token, err := svc.Issue(7)
	require.NoError(t, err)
	svc.now = func() time.Time { return token.ExpiresAt.Add(-time.Second) }
	_, err = svc.Verify(token.Token)
	require.NoError(t, err)
	for _, expiredAt := range []time.Time{token.ExpiresAt, token.ExpiresAt.Add(time.Second)} {
		svc.now = func() time.Time { return expiredAt }
		id, verifyErr := svc.Verify(token.Token)
		require.ErrorIs(t, verifyErr, errDegradationVisualToken)
		require.Zero(t, id)
	}
}

func TestDegradationVisualToken_RandomFallbackAndUnavailable(t *testing.T) {
	svc := NewDegradationVisualTokenService(nil)
	token, err := svc.Issue(5)
	require.NoError(t, err)
	id, err := svc.Verify(token.Token)
	require.NoError(t, err)
	require.Equal(t, int64(5), id)
	other := NewDegradationVisualTokenService(&config.Config{})
	_, err = other.Verify(token.Token)
	require.Error(t, err)
	for _, invalid := range []int64{-1, 0} {
		_, err = svc.Issue(invalid)
		require.Error(t, err)
	}
	var unavailable *DegradationVisualTokenService
	_, err = unavailable.Issue(1)
	require.Error(t, err)
	_, err = unavailable.Verify(token.Token)
	require.Error(t, err)
}
