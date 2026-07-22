package service

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/novel2av/backend/internal/domain"
)

// fakeStorage is an in-memory storage stand-in for tests.
type fakeStorage struct {
	put     map[string][]byte
	removed []string
}

func newFakeStorage() *fakeStorage { return &fakeStorage{put: map[string][]byte{}} }

func (f *fakeStorage) Put(_ context.Context, key string, r interface{}, size int64, _ string) error {
	// The real client uses io.Reader; for tests we just record the key.
	f.put[key] = []byte("ok")
	return nil
}
func (f *fakeStorage) RemovePrefix(_ context.Context, prefix string) error {
	f.removed = append(f.removed, prefix)
	return nil
}

func TestCreateProject_Validation(t *testing.T) {
	s := &ProjectService{}
	cases := []struct {
		name string
		in   CreateProjectInput
	}{
		{"missing title", CreateProjectInput{UserID: "u", Filename: "x.txt", Size: 1, Title: "  "}},
		{"missing user", CreateProjectInput{Title: "t", Filename: "x.txt", Size: 1}},
		{"bad ext", CreateProjectInput{UserID: "u", Title: "t", Filename: "x.epub", Size: 1}},
		{"bad size", CreateProjectInput{UserID: "u", Title: "t", Filename: "x.txt", Size: maxNovelBytes + 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.Create(context.Background(), c.in)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}
}

func TestCreateProject_RequiresRealStorage(t *testing.T) {
	// End-to-end DB path requires Postgres; here we just verify Create rejects
	// when storage isn't wired (would nil-deref without fake). Sanity check:
	s := &ProjectService{}
	in := CreateProjectInput{
		UserID: "u", Title: "t", Filename: "a.txt",
		Content: bytes.NewReader([]byte("hello")), Size: 5,
	}
	_, err := s.Create(context.Background(), in)
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "nil pointer"))
}
