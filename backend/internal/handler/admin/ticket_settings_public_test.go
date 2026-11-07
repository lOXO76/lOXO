//go:build unit

package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTicketSystemFlagGenericSettingsPreservesAbsentAndAudits(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeyTicketSystemEnabled: "true"})
	w := doUpdateSettings(t, h, map[string]any{}, nil)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "true", repo.values[service.SettingKeyTicketSystemEnabled])
	w = doUpdateSettings(t, h, map[string]any{service.SettingKeyTicketSystemEnabled: false}, nil)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "false", repo.values[service.SettingKeyTicketSystemEnabled])
	require.Contains(t, diffSettings(&service.SystemSettings{}, &service.SystemSettings{TicketSystemEnabled: true}, nil, nil, UpdateSettingsRequest{}), service.SettingKeyTicketSystemEnabled)
}
