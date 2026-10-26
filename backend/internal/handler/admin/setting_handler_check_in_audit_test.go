package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDiffSettingsDetectsCheckInChanges(t *testing.T) {
	base := &service.SystemSettings{
		CheckInEnabled:        false,
		CheckInCaptchaEnabled: false,
		CheckInTiers:          []service.CheckInTier{{Min: 0.1, Max: 0.5, Weight: 70}},
	}

	unchanged := *base
	unchanged.CheckInTiers = []service.CheckInTier{{Min: 0.1, Max: 0.5, Weight: 70}}
	require.NotContains(t, diffSettings(base, &unchanged, nil, nil, UpdateSettingsRequest{}), "check_in_enabled")
	require.NotContains(t, diffSettings(base, &unchanged, nil, nil, UpdateSettingsRequest{}), "check_in_tiers")

	enabled := *base
	enabled.CheckInEnabled = true
	require.Contains(t, diffSettings(base, &enabled, nil, nil, UpdateSettingsRequest{}), "check_in_enabled")

	captcha := *base
	captcha.CheckInCaptchaEnabled = true
	require.Contains(t, diffSettings(base, &captcha, nil, nil, UpdateSettingsRequest{}), "check_in_captcha_enabled")

	reweighted := *base
	reweighted.CheckInTiers = []service.CheckInTier{{Min: 0.1, Max: 0.5, Weight: 30}, {Min: 0.5, Max: 1, Weight: 70}}
	require.Contains(t, diffSettings(base, &reweighted, nil, nil, UpdateSettingsRequest{}), "check_in_tiers")
}

func TestEqualCheckInTiers(t *testing.T) {
	a := []service.CheckInTier{{Min: 0, Max: 1, Weight: 1}, {Min: 1, Max: 2, Weight: 2}}
	require.True(t, equalCheckInTiers(a, []service.CheckInTier{{Min: 0, Max: 1, Weight: 1}, {Min: 1, Max: 2, Weight: 2}}))
	require.False(t, equalCheckInTiers(a, a[:1]))
	require.False(t, equalCheckInTiers(a, []service.CheckInTier{{Min: 0, Max: 1, Weight: 1}, {Min: 1, Max: 2, Weight: 3}}))
	require.True(t, equalCheckInTiers(nil, []service.CheckInTier{}))
}
