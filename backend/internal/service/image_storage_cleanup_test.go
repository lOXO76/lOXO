//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type cleaningStorage struct {
	recordingStorage
	calls   int
	prefix  string
	before  time.Time
	deleted int
}

func (s *cleaningStorage) DeleteObjectsBefore(_ context.Context, prefix string, before time.Time) (int, error) {
	s.calls++
	s.prefix = prefix
	s.before = before
	return s.deleted, nil
}

func newImageStorageCleanupFixture(t *testing.T) (*ImageStorageSettingService, *stubSettingRepo, *cleaningStorage) {
	t.Helper()
	repo := newStubSettingRepo()
	encryptor := reversibleEncryptor{}
	backup := NewBackupService(repo, &config.Config{
		Totp: config.TotpConfig{EncryptionKeyConfigured: true},
	}, encryptor, nil, nil)
	storage := &cleaningStorage{deleted: 3}
	factory := func(context.Context, *config.ImageStorageConfig) (ImageStorage, error) {
		return storage, nil
	}
	return NewImageStorageSettingService(repo, encryptor, backup, factory, config.ImageStorageConfig{}), repo, storage
}

func newTestImageStorageCleanup(settings *ImageStorageSettingService, now time.Time) *ImageStorageCleanupService {
	svc := NewImageStorageCleanupService(settings)
	svc.now = func() time.Time { return now }
	return svc
}

func TestImageStorageCleanupDefaultsTo168Hours(t *testing.T) {
	settings, _, storage := newImageStorageCleanupFixture(t)
	ctx := context.Background()
	_, err := settings.Update(ctx, ImageStorageSettings{
		Enabled: true, Bucket: "img", AccessKeyID: "ak", SecretAccessKey: "sk",
	})
	require.NoError(t, err)

	got, err := settings.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, 168, got.DeleteAfterHours)

	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	deleted := newTestImageStorageCleanup(settings, now).cleanupOnce()

	require.Equal(t, 3, deleted)
	require.Equal(t, 1, storage.calls)
	require.Equal(t, "images/", storage.prefix)
	require.Equal(t, now.Add(-168*time.Hour), storage.before)
}

func TestImageStorageCleanupUsesConfiguredHours(t *testing.T) {
	settings, _, storage := newImageStorageCleanupFixture(t)
	_, err := settings.Update(context.Background(), ImageStorageSettings{
		Enabled: true, Bucket: "img", AccessKeyID: "ak", SecretAccessKey: "sk",
		Prefix: "gen", DeleteAfterHours: 48,
	})
	require.NoError(t, err)

	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	newTestImageStorageCleanup(settings, now).cleanupOnce()

	require.Equal(t, "gen/", storage.prefix)
	require.Equal(t, now.Add(-48*time.Hour), storage.before)
}

func TestImageStorageCleanupSkipsWhenDisabled(t *testing.T) {
	settings, _, storage := newImageStorageCleanupFixture(t)
	_, err := settings.Update(context.Background(), ImageStorageSettings{
		Enabled: false, Bucket: "img", AccessKeyID: "ak", SecretAccessKey: "sk",
	})
	require.NoError(t, err)

	require.Zero(t, newTestImageStorageCleanup(settings, time.Now()).cleanupOnce())
	require.Zero(t, storage.calls)
}

// Images may share the backup bucket; a prefix overlapping backups/ must never be swept.
func TestImageStorageCleanupRefusesPrefixOverlappingBackups(t *testing.T) {
	settings, repo, storage := newImageStorageCleanupFixture(t)
	seedBackupS3(t, repo, BackupS3Config{
		Bucket: "shared", AccessKeyID: "ak", SecretAccessKey: "sk", Prefix: "backups/",
	})
	_, err := settings.Update(context.Background(), ImageStorageSettings{
		Enabled: true, ReuseBackupS3: true, Prefix: "backups/",
	})
	require.NoError(t, err)

	require.Zero(t, newTestImageStorageCleanup(settings, time.Now()).cleanupOnce())
	require.Zero(t, storage.calls)

	_, err = settings.Update(context.Background(), ImageStorageSettings{
		Enabled: true, ReuseBackupS3: true, Prefix: "images/",
	})
	require.NoError(t, err)
	newTestImageStorageCleanup(settings, time.Now()).cleanupOnce()
	require.Equal(t, 1, storage.calls, "a disjoint prefix in the shared bucket is cleaned normally")
}

func TestImageStorageSettingsLegacyRowReportsDefaultDeleteAfterHours(t *testing.T) {
	settings, repo, _ := newImageStorageCleanupFixture(t)
	require.NoError(t, repo.Set(context.Background(), settingKeyImageStorageConfig,
		`{"enabled":true,"bucket":"img","prefix":"images/"}`))

	got, err := settings.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, 168, got.DeleteAfterHours)
}
