package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	degradationVisualTokenPurpose = "sub2api:degradation-visual-render:v1"
	degradationVisualTokenTTL     = 15 * time.Minute
)

var errDegradationVisualToken = errors.New("invalid degradation visual token")

type DegradationVisualTokenService struct {
	key []byte
	now func() time.Time
}

func NewDegradationVisualTokenService(cfg *config.Config) *DegradationVisualTokenService {
	s := &DegradationVisualTokenService{now: time.Now}
	if cfg != nil && cfg.JWT.Secret != "" {
		mac := hmac.New(sha256.New, []byte(cfg.JWT.Secret))
		_, _ = mac.Write([]byte(degradationVisualTokenPurpose))
		s.key = mac.Sum(nil)
		return s
	}
	key := make([]byte, sha256.Size)
	if _, err := rand.Read(key); err != nil {
		slog.Error("degradation visual tokens disabled: random key generation failed", "error", err)
		return s
	}
	s.key = key
	slog.Warn("JWT secret is empty; degradation visual tokens use a process-local random key")
	return s
}

func (s *DegradationVisualTokenService) Issue(runID int64) (*DegradationVisualViewToken, error) {
	if s == nil || len(s.key) == 0 || runID <= 0 {
		return nil, errDegradationVisualToken
	}
	expiresAt := s.now().Add(degradationVisualTokenTTL).UTC().Truncate(time.Second)
	payload := fmt.Sprintf("v1.%d.%d", runID, expiresAt.Unix())
	token := payload + "." + base64.RawURLEncoding.EncodeToString(s.sign(payload))
	return &DegradationVisualViewToken{Token: token, ExpiresAt: expiresAt}, nil
}

func (s *DegradationVisualTokenService) Verify(token string) (int64, error) {
	if s == nil || len(s.key) == 0 || len(token) > 128 {
		return 0, errDegradationVisualToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return 0, errDegradationVisualToken
	}
	runID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || runID <= 0 || strconv.FormatInt(runID, 10) != parts[1] {
		return 0, errDegradationVisualToken
	}
	expires, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || strconv.FormatInt(expires, 10) != parts[2] {
		return 0, errDegradationVisualToken
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(parts[3])
	if err != nil || !hmac.Equal(signature, s.sign(strings.Join(parts[:3], "."))) {
		return 0, errDegradationVisualToken
	}
	if expires <= s.now().Unix() {
		return 0, errDegradationVisualToken
	}
	return runID, nil
}

func (s *DegradationVisualTokenService) sign(payload string) []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
