package repository

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// S3ImageStorage 用 S3 兼容对象存储实现 service.ImageStorage。
type S3ImageStorage struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
	presignExpiry time.Duration
}

var (
	_ service.ImageStorage        = (*S3ImageStorage)(nil)
	_ service.ImageStorageCleaner = (*S3ImageStorage)(nil)
)

// NewS3ImageStorage 依据配置构造 S3 图片存储（调用方应先确认 cfg.Active()）。
func NewS3ImageStorage(ctx context.Context, cfg *config.ImageStorageConfig) (*S3ImageStorage, error) {
	client, err := newS3Client(ctx, s3ClientParams{
		Endpoint:        cfg.Endpoint,
		Region:          cfg.Region,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		ForcePathStyle:  cfg.ForcePathStyle,
	})
	if err != nil {
		return nil, err
	}

	expiry := time.Duration(cfg.PresignExpiry) * time.Hour
	if expiry <= 0 {
		expiry = 24 * time.Hour
	}

	return &S3ImageStorage{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
		presignExpiry: expiry,
	}, nil
}

// Save 上传图片字节，返回可访问 URL：配了 public_base_url 则返回公开直链，否则返回 presigned 临时链接。
func (s *S3ImageStorage) Save(ctx context.Context, key, contentType string, data []byte) (string, error) {
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        bytes.NewReader(data),
		ContentType: &contentType,
	})
	finish()
	if err != nil {
		return "", fmt.Errorf("S3 PutObject: %w", err)
	}

	if s.publicBaseURL != "" {
		return s.publicBaseURL + "/" + strings.TrimLeft(key, "/"), nil
	}

	presignClient := s3.NewPresignClient(s.client)
	result, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(s.presignExpiry))
	if err != nil {
		return "", fmt.Errorf("presign url: %w", err)
	}
	return result.URL, nil
}

// DeleteObjectsBefore 删除 prefix 下 LastModified 早于 before 的对象，返回成功删除数。
//
// 逐个调用 DeleteObject 而非批量 DeleteObjects：阿里云 OSS 等 S3 兼容实现对批量删除的
// 校验头（Content-MD5 / CRC）支持不一致，单删兼容性最好。单个对象删除失败只记录并继续，
// 最后返回首个错误。
func (s *S3ImageStorage) DeleteObjectsBefore(ctx context.Context, prefix string, before time.Time) (int, error) {
	if strings.TrimSpace(prefix) == "" {
		return 0, fmt.Errorf("refusing to delete objects with an empty prefix")
	}
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &prefix,
	})
	deleted := 0
	var firstErr error
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return deleted, fmt.Errorf("S3 ListObjectsV2: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil || obj.LastModified == nil || !obj.LastModified.Before(before) {
				continue
			}
			if !strings.HasPrefix(*obj.Key, prefix) {
				continue
			}
			if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: &s.bucket,
				Key:    obj.Key,
			}); err != nil {
				if ctx.Err() != nil {
					return deleted, ctx.Err()
				}
				if firstErr == nil {
					firstErr = fmt.Errorf("S3 DeleteObject %s: %w", *obj.Key, err)
				}
				continue
			}
			deleted++
		}
	}
	return deleted, firstErr
}
