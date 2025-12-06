package session

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockStore implements the Store interface for testing
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Delete(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockStore) Find(ctx context.Context, token string) ([]byte, bool, error) {
	args := m.Called(ctx, token)
	return args.Get(0).([]byte), args.Bool(1), args.Error(2)
}

func (m *MockStore) Commit(ctx context.Context, token string, data []byte, expiry time.Time) error {
	args := m.Called(ctx, token, data, expiry)
	return args.Error(0)
}
