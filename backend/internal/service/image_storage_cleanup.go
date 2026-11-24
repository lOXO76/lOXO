package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	// defaultImageDeleteAfterHours 是生图结果原文件在对象存储中的默认保留时长（7 天）。
	defaultImageDeleteAfterHours = 168

	defaultImageStorageCleanupInterval     = time.Hour
	defaultImageStorageCleanupInitialDelay = 2 * time.Minute
	defaultImageStorageCleanupTimeout      = 30 * time.Minute
)

// ImageStorageCleaner 是 ImageStorage 的可选能力：删除 prefix 下最后修改时间早于 before 的对象。
// 返回成功删除的对象数。未实现该接口的存储不会被自动清理。
type ImageStorageCleaner interface {
	DeleteObjectsBefore(ctx context.Context, prefix string, before time.Time) (deleted int, err error)
}

// imageStorageCleanupTarget 描述一次清理要处理的存储、前缀与截止时间。
type imageStorageCleanupTarget struct {
	cleaner   ImageStorageCleaner
	bucket    string
	prefix    string
	retention time.Duration
}

// cleanupTarget 返回当前生效的清理目标；功能未启用、存储不支持清理或前缀不安全时返回 nil。
func (s *ImageStorageSettingService) cleanupTarget(ctx context.Context) (*imageStorageCleanupTarget, error) {
	if s == nil {
		return nil, nil //nolint:nilnil // nothing to clean
	}
	if _, enabled := s.resolve(); !enabled {
		return nil, nil //nolint:nilnil // feature disabled
	}
	s.mu.Lock()
	storage, cfg := s.storage, s.cfg
	s.mu.Unlock()
	if storage == nil || cfg == nil {
		return nil, nil //nolint:nilnil // invalidated concurrently
	}
	cleaner, ok := storage.(ImageStorageCleaner)
	if !ok {
		return nil, nil //nolint:nilnil // storage does not support cleanup
	}

	prefix := strings.TrimLeft(strings.TrimSpace(cfg.Prefix), "/")
	if prefix == "" {
		// 空前缀意味着整个桶，绝不做批量删除。
		return nil, errors.New("image storage prefix is empty; refusing to clean up the whole bucket")
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if err := s.checkPrefixDisjointFromBackup(ctx, cfg.Bucket, prefix); err != nil {
		return nil, err
	}

	hours := cfg.DeleteAfterHours
	if hours <= 0 {
		hours = defaultImageDeleteAfterHours
	}
	return &imageStorageCleanupTarget{
		cleaner:   cleaner,
		bucket:    cfg.Bucket,
		prefix:    prefix,
		retention: time.Duration(hours) * time.Hour,
	}, nil
}

// checkPrefixDisjointFromBackup 防止图片与数据库备份共用同一个桶且前缀相互包含时误删备份。
func (s *ImageStorageSettingService) checkPrefixDisjointFromBackup(ctx context.Context, bucket, prefix string) error {
	if s.backup == nil {
		return nil
	}
	backupCfg, err := s.backup.loadS3Config(ctx)
	if err != nil || backupCfg == nil || backupCfg.Bucket == "" || backupCfg.Bucket != bucket {
		return nil
	}
	backupPrefix := strings.Trim(strings.TrimSpace(backupCfg.Prefix), "/")
	if backupPrefix == "" {
		backupPrefix = "backups"
	}
	backupPrefix += "/"
	if strings.HasPrefix(backupPrefix, prefix) || strings.HasPrefix(prefix, backupPrefix) {
		return errors.New("image storage prefix overlaps the backup prefix in the same bucket; refusing to clean up")
	}
	return nil
}

// ImageStorageCleanupService 定期删除对象存储中超过保留时长的生图结果原文件。
//
// presigned 链接过期只是链接失效，对象本身仍留在桶里；不清理会无限增长。
// 保留时长取后台设置的 delete_after_hours（默认 168 小时），后台修改后下一轮即生效。
// 多实例部署时各实例都会执行，删除操作幂等，不会互相影响。
type ImageStorageCleanupService struct {
	settings     *ImageStorageSettingService
	interval     time.Duration
	initialDelay time.Duration
	now          func() time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
}

func NewImageStorageCleanupService(settings *ImageStorageSettingService) *ImageStorageCleanupService {
	return &ImageStorageCleanupService{
		settings:     settings,
		interval:     defaultImageStorageCleanupInterval,
		initialDelay: defaultImageStorageCleanupInitialDelay,
		now:          time.Now,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

func (s *ImageStorageCleanupService) Start() {
	if s == nil || s.settings == nil {
		return
	}
	s.startOnce.Do(func() {
		logger.L().Info("image_storage.cleanup_started", zap.Duration("interval", s.interval))
		go s.runLoop()
	})
}

func (s *ImageStorageCleanupService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *ImageStorageCleanupService) runLoop() {
	defer close(s.doneCh)

	// 启动后稍等再执行第一轮，避开启动高峰。
	select {
	case <-time.After(s.initialDelay):
	case <-s.stopCh:
		return
	}
	s.cleanupOnce()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanupOnce()
		case <-s.stopCh:
			return
		}
	}
}

// cleanupOnce 执行一轮清理，返回删除的对象数。
func (s *ImageStorageCleanupService) cleanupOnce() int {
	ctx, cancel := context.WithTimeout(context.Background(), defaultImageStorageCleanupTimeout)
	defer cancel()
	go func() {
		select {
		case <-s.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	target, err := s.settings.cleanupTarget(ctx)
	if err != nil {
		logger.L().Warn("image_storage.cleanup_skipped", zap.Error(err))
		return 0
	}
	if target == nil {
		return 0
	}
	before := s.now().Add(-target.retention)
	deleted, err := target.cleaner.DeleteObjectsBefore(ctx, target.prefix, before)
	if err != nil {
		logger.L().Warn("image_storage.cleanup_failed",
			zap.String("bucket", target.bucket),
			zap.String("prefix", target.prefix),
			zap.Int("deleted", deleted),
			zap.Error(err))
		return deleted
	}
	if deleted > 0 {
		logger.L().Info("image_storage.cleanup_done",
			zap.String("bucket", target.bucket),
			zap.String("prefix", target.prefix),
			zap.Time("before", before),
			zap.Int("deleted", deleted))
	}
	return deleted
}
