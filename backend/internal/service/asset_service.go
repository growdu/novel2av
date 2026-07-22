package service

import (
	"context"
	"net/url"
	"time"

	"github.com/novel2av/backend/internal/infra/storage"
)

type AssetService struct {
	storage *storage.MinIOClient
}

func NewAssetService(st *storage.MinIOClient) *AssetService {
	return &AssetService{storage: st}
}

// SignedURL returns a time-limited URL for an asset the client may download.
func (s *AssetService) SignedURL(ctx context.Context, key string, ttl time.Duration) (*url.URL, error) {
	return s.storage.PresignGet(ctx, key, ttl)
}

// URL is the string-returning convenience wrapper.
func (s *AssetService) URL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.storage.PresignGet(ctx, key, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
