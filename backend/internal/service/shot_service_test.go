package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/novel2av/backend/internal/domain"
)

func TestShotAssetPatch_NilFields_NoOp(t *testing.T) {
	// Sanity: ensure the helper at least constructs and validation flows
	// don't trip on nil pointers (DB path will fail, which is expected).
	s := &ShotService{}
	_, err := s.IngestShotAssets(context.Background(), "x", ShotAssetPatch{})
	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrInvalidInput)
}
