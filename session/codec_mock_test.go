package session

import (
	"time"

	"github.com/stretchr/testify/mock"
)

// MockCodec implements the Codec interface for testing
type MockCodec struct {
	mock.Mock
}

func (m *MockCodec) Encode(deadline time.Time, values map[string]any) ([]byte, error) {
	args := m.Called(deadline, values)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCodec) Decode(b []byte) (time.Time, map[string]any, error) {
	args := m.Called(b)
	return args.Get(0).(time.Time), args.Get(1).(map[string]any), args.Error(2)
}
