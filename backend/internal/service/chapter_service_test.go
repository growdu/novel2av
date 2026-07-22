package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/novel2av/backend/internal/domain"
)

func TestPatch_RejectsEmptyTitle(t *testing.T) {
	s := &ChapterService{}
	empty := ""
	_, err := s.Patch(context.Background(), "x", &empty, nil)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestPatch_NilTitleOK(t *testing.T) {
	// No DB connection → just verifies the nil-title branch doesn't trigger validation.
	s := &ChapterService{}
	status := "READY"
	_, err := s.Patch(context.Background(), "x", nil, &status)
	require.Error(t, err) // expected to fail on DB; we just want to confirm no validation error
	require.NotErrorIs(t, err, domain.ErrInvalidInput)
}
