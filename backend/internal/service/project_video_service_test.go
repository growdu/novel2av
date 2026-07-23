package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/novel2av/backend/internal/domain"
)

func TestProjectVideo_MissingProjectMaps(t *testing.T) {
	s := &ProjectVideoService{}
	_, err := s.TriggerCompose(context.Background(), "missing")
	require.Error(t, err)
}
