package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
)

type RewardTaskSettings struct {
	Enabled bool `json:"enabled"`
}

func (s *SettingService) GetRewardTaskSettings(ctx context.Context) (*RewardTaskSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRewardTasksEnabled)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return nil, err
	}
	return &RewardTaskSettings{Enabled: value == "true"}, nil
}

func (s *SettingService) IsRewardTasksEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	settings, err := s.GetRewardTaskSettings(ctx)
	return err == nil && settings.Enabled
}

func (s *SettingService) SetRewardTasksEnabled(ctx context.Context, enabled bool) error {
	// Write only this key so a concurrent settings update cannot be overwritten.
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyRewardTasksEnabled: strconv.FormatBool(enabled),
	}); err != nil {
		return err
	}
	stored, err := s.GetAllSettings(ctx)
	if err != nil {
		slog.Warn("refresh cached settings after reward task update failed", "error", err)
		// The write succeeded: still invalidate the public settings injection cache.
		if s.onUpdate != nil {
			s.onUpdate()
		}
		return nil
	}
	s.refreshCachedSettings(stored)
	return nil
}
