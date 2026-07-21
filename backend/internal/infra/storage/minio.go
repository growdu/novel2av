// Package storage wraps the MinIO/S3 client.
package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/novel2av/backend/internal/config"
)

type MinIOClient struct {
	cli    *minio.Client
	bucket string
}

// NewMinIO connects to MinIO and ensures the target bucket exists.
func NewMinIO(ctx context.Context, cfg config.S3Config) (*MinIOClient, error) {
	cli, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: false,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio new: %w", err)
	}
	exists, err := cli.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("bucket exists: %w", err)
	}
	if !exists {
		if err := cli.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("make bucket: %w", err)
		}
	}
	return &MinIOClient{cli: cli, bucket: cfg.Bucket}, nil
}

// PresignGet returns a time-limited download URL.
func (m *MinIOClient) PresignGet(ctx context.Context, key string, ttl time.Duration) (*url.URL, error) {
	reqParams := url.Values{}
	u, err := m.cli.PresignedGetObject(ctx, m.bucket, key, ttl, reqParams)
	if err != nil {
		return nil, err
	}
	return u, nil
}
