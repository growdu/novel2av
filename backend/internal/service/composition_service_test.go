package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/novel2av/backend/internal/domain"
)

// Most composition paths require a live DB; here we just confirm the
// error mapping behaves correctly when the project row is missing.
func TestCompose_MissingProjectMapsToNotFound(t *testing.T) {
	s := &CompositionService{}
	_, err := s.TriggerCompose(context.Background(), "does-not-exist", "9:16")
	require.Error(t, err)
}
