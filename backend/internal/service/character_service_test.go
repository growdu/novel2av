package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/novel2av/backend/internal/domain"
)

func TestCharacterPatch_RejectsEmptyName(t *testing.T) {
	s := &CharacterService{}
	empty := ""
	_, err := s.Patch(context.Background(), "x", domain.CharacterPatch{Name: &empty})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestCharacterPatch_NilNameOK(t *testing.T) {
	s := &CharacterService{}
	role := "protagonist"
	_, err := s.Patch(context.Background(), "x", domain.CharacterPatch{Role: &role})
	require.Error(t, err) // expected to fail on DB; just confirms no validation error
	require.NotErrorIs(t, err, domain.ErrInvalidInput)
}
