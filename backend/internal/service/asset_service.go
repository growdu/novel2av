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
