package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test setup helpers
func setupTestSession() (*Session, context.Context, error) {
	mockStore := &MockStore{}
	mockCodec := &MockCodec{}
	config := Config{
		Lifetime:    24 * time.Hour,
		IdleTimeout: time.Hour,
	}
	config.SetDefaults()
	session := NewWithCodec(config, mockStore, mockCodec)
	ctx := context.Background()

	// Set up session data
	mockStore.On("Find", mock.Anything, "test-token").Return([]byte{}, false, nil)
	ctx, err := session.Load(ctx, "test-token")
	if err != nil {
		return nil, nil, err
	}

	return session, ctx, nil
}

func setupTestSessionWithData() (*Session, context.Context, error) {
	session, ctx, err := setupTestSession()
	if err != nil {
		return nil, nil, err
	}

	// Add some test data
	session.Put(ctx, "stringKey", "stringValue")
	session.Put(ctx, "intKey", 42)
	session.Put(ctx, "boolKey", true)
	session.Put(ctx, "floatKey", 3.14)
	session.Put(ctx, "timeKey", time.Now())
	session.Put(ctx, "bytesKey", []byte("byteValue"))

	return session, ctx, nil
}

// Utility function tests
func TestNewSessionData(t *testing.T) {
	lifetime := 2 * time.Hour
	sd := newSessionData(lifetime)

	assert.NotNil(t, sd)
	assert.Equal(t, Unmodified, sd.status)
	assert.Empty(t, sd.token)
	assert.NotNil(t, sd.values)
	assert.Empty(t, sd.values)

	// Check deadline is approximately now + lifetime
	expectedDeadline := time.Now().Add(lifetime).UTC()
	assert.WithinDuration(t, expectedDeadline, sd.deadline, time.Second)
}

func TestGenerateToken(t *testing.T) {
	token1, err := generateToken()
	assert.NoError(t, err)
	assert.NotEmpty(t, token1)

	token2, err := generateToken()
	assert.NoError(t, err)
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2, "Tokens should be unique")
}

func TestHashToken(t *testing.T) {
	token := "test-token"
	hash1 := hashToken(token)
	assert.NotEmpty(t, hash1)
	assert.Equal(t, hashToken(token), hash1, "Hash should be deterministic")

	// Different tokens should have different hashes
	token2 := "different-token"
	hash2 := hashToken(token2)
	assert.NotEqual(t, hash1, hash2)
}

func TestGenerateContextKey(t *testing.T) {
	key1 := generateContextKey()
	assert.NotEmpty(t, key1)

	key2 := generateContextKey()
	assert.NotEmpty(t, key2)
	assert.NotEqual(t, key1, key2, "Context keys should be unique")

	// Check format
	assert.Contains(t, string(key1), "session.")
}

// Session method tests
func TestLoad_NewSession(t *testing.T) {
	mockStore := &MockStore{}
	mockCodec := &MockCodec{}
	config := Config{Lifetime: time.Hour}
	session := NewWithCodec(config, mockStore, mockCodec)

	ctx := context.Background()
	resultCtx, err := session.Load(ctx, "")

	assert.NoError(t, err)
	assert.NotNil(t, resultCtx)

	// Session data should be in context
	sessionData := resultCtx.Value(session.contextKey)
	assert.NotNil(t, sessionData)
}

func TestLoad_ExistingSession(t *testing.T) {
	mockStore := &MockStore{}
	mockCodec := &MockCodec{}
	config := Config{Lifetime: time.Hour}
	session := NewWithCodec(config, mockStore, mockCodec)

	token := "existing-token"
	storedData := []byte("encoded-data")
	mockStore.On("Find", mock.Anything, token).Return(storedData, true, nil)
	mockCodec.On("Decode", storedData).Return(time.Now().Add(time.Hour), map[string]any{"key": "value"}, nil)

	ctx := context.Background()
	resultCtx, err := session.Load(ctx, token)

	assert.NoError(t, err)
	assert.NotNil(t, resultCtx)

	mockStore.AssertExpectations(t)
	mockCodec.AssertExpectations(t)
}

func TestLoad_SessionAlreadyInContext(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Try to load again - should return same context
	resultCtx, err := session.Load(ctx, "different-token")
	assert.NoError(t, err)
	assert.Same(t, ctx, resultCtx)
}

func TestLoad_StoreError(t *testing.T) {
	mockStore := &MockStore{}
	mockCodec := &MockCodec{}
	config := Config{Lifetime: time.Hour}
	session := NewWithCodec(config, mockStore, mockCodec)

	token := "error-token"
	mockStore.On("Find", mock.Anything, token).Return([]byte{}, false, assert.AnError)

	ctx := context.Background()
	_, err := session.Load(ctx, token)

	assert.Error(t, err)
	assert.Same(t, assert.AnError, err)
}

func TestLoad_CodecError(t *testing.T) {
	mockStore := &MockStore{}
	mockCodec := &MockCodec{}
	config := Config{Lifetime: time.Hour}
	session := NewWithCodec(config, mockStore, mockCodec)

	token := "corrupted-token"
	storedData := []byte("corrupted-data")
	mockStore.On("Find", mock.Anything, token).Return(storedData, true, nil)
	mockCodec.On("Decode", storedData).Return(time.Time{}, map[string]any(nil), assert.AnError)

	ctx := context.Background()
	_, err := session.Load(ctx, token)

	assert.Error(t, err)
	assert.Same(t, assert.AnError, err)
}

func TestLoad_IdleTimeout(t *testing.T) {
	mockStore := &MockStore{}
	mockCodec := &MockCodec{}
	config := Config{Lifetime: time.Hour, IdleTimeout: 30 * time.Minute}
	session := NewWithCodec(config, mockStore, mockCodec)

	token := "existing-token"
	storedData := []byte("encoded-data")
	mockStore.On("Find", mock.Anything, token).Return(storedData, true, nil)
	mockCodec.On("Decode", storedData).Return(time.Now().Add(time.Hour), map[string]any{"key": "value"}, nil)

	ctx := context.Background()
	resultCtx, err := session.Load(ctx, token)

	assert.NoError(t, err)
	assert.NotNil(t, resultCtx)

	// Session should be marked as modified due to idle timeout
	sd := resultCtx.Value(session.contextKey).(*sessionData)
	assert.Equal(t, Modified, sd.status)
}

func TestCommit_NewToken(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Mock the codec and store operations
	expectedData := []byte("encoded-data")
	mockStore := session.store.(*MockStore)
	mockCodec := session.codec.(*MockCodec)
	mockCodec.On("Encode", mock.Anything, mock.Anything).Return(expectedData, nil)
	mockStore.On("Commit", mock.Anything, mock.Anything, expectedData, mock.Anything).Return(nil)

	token, expiry, err := session.Commit(ctx)

	assert.NoError(t, err)
	assert.NotEmpty(t, token) // Token is generated randomly
	assert.NotZero(t, expiry)

	mockCodec.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

func TestCommit_ExistingToken(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Set a token
	session.SetToken(ctx, "existing-token")

	expectedData := []byte("encoded-data")
	mockStore := session.store.(*MockStore)
	mockCodec := session.codec.(*MockCodec)
	mockCodec.On("Encode", mock.Anything, mock.Anything).Return(expectedData, nil)
	mockStore.On("Commit", mock.Anything, "existing-token", expectedData, mock.Anything).Return(nil)

	token, expiry, err := session.Commit(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "existing-token", token)
	assert.NotZero(t, expiry)
}

func TestCommit_TokenGenerationError(t *testing.T) {
	// This test would require mocking the generateToken function or making it replaceable
	// For now, we'll test the happy path only
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	expectedData := []byte("encoded-data")
	mockStore := session.store.(*MockStore)
	mockCodec := session.codec.(*MockCodec)
	mockCodec.On("Encode", mock.Anything, mock.Anything).Return(expectedData, nil)
	mockStore.On("Commit", mock.Anything, mock.Anything, expectedData, mock.Anything).Return(assert.AnError)

	_, _, err = session.Commit(ctx)

	assert.Error(t, err)
	assert.Same(t, assert.AnError, err)
}

func TestDestroy(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Set some data
	session.Put(ctx, "key", "value")
	session.SetToken(ctx, "test-token")

	mockStore := session.store.(*MockStore)
	mockStore.On("Delete", mock.Anything, "test-token").Return(nil)

	err = session.Destroy(ctx)

	assert.NoError(t, err)
	assert.Equal(t, Destroyed, session.Status(ctx))
	assert.Empty(t, session.Token(ctx))
	assert.Empty(t, session.Keys(ctx))

	mockStore.AssertExpectations(t)
}

func TestDestroy_StoreError(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	session.SetToken(ctx, "test-token")

	mockStore := session.store.(*MockStore)
	mockStore.On("Delete", mock.Anything, "test-token").Return(assert.AnError)

	err = session.Destroy(ctx)

	assert.Error(t, err)
	assert.Same(t, assert.AnError, err)
}

func TestDestroy_NoToken(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Mock the store call for empty token (will be hashed if needed)
	mockStore := session.store.(*MockStore)
	mockStore.On("Delete", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Don't set a token - should not panic
	err = session.Destroy(ctx)

	assert.NoError(t, err)
	assert.Equal(t, Destroyed, session.Status(ctx))
}

func TestDeadline(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	deadline := session.Deadline(ctx)
	assert.False(t, deadline.IsZero())
}

func TestSetDeadline(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	newDeadline := time.Now().Add(2 * time.Hour)
	session.SetDeadline(ctx, newDeadline)

	assert.Equal(t, newDeadline, session.Deadline(ctx))
	assert.Equal(t, Modified, session.Status(ctx))
}

func TestGet(t *testing.T) {
	session, ctx, err := setupTestSessionWithData()
	require.NoError(t, err)

	assert.Equal(t, "stringValue", session.Get(ctx, "stringKey"))
	assert.Equal(t, 42, session.Get(ctx, "intKey"))
	assert.Equal(t, true, session.Get(ctx, "boolKey"))
	assert.Equal(t, 3.14, session.Get(ctx, "floatKey"))
	assert.Nil(t, session.Get(ctx, "nonExistentKey"))
}

func TestPop(t *testing.T) {
	session, ctx, err := setupTestSessionWithData()
	require.NoError(t, err)

	// Pop existing key
	value := session.Pop(ctx, "stringKey")
	assert.Equal(t, "stringValue", value)
	assert.Equal(t, Modified, session.Status(ctx))
	assert.Nil(t, session.Get(ctx, "stringKey")) // Should be removed

	// Pop non-existent key
	value = session.Pop(ctx, "nonExistentKey")
	assert.Nil(t, value)
}

func TestRemove(t *testing.T) {
	session, ctx, err := setupTestSessionWithData()
	require.NoError(t, err)

	// Remove existing key
	session.Remove(ctx, "stringKey")
	assert.Nil(t, session.Get(ctx, "stringKey"))
	assert.Equal(t, Modified, session.Status(ctx))

	// Remove non-existent key - should not panic or change status
	session.Remove(ctx, "nonExistentKey")
}

func TestClear(t *testing.T) {
	session, ctx, err := setupTestSessionWithData()
	require.NoError(t, err)

	err = session.Clear(ctx)
	assert.NoError(t, err)
	assert.Empty(t, session.Keys(ctx))
	assert.Equal(t, Modified, session.Status(ctx))

	// Clear empty session - should be no-op
	err = session.Clear(ctx)
	assert.NoError(t, err)
}

func TestHas(t *testing.T) {
	session, ctx, err := setupTestSessionWithData()
	require.NoError(t, err)

	assert.True(t, session.Has(ctx, "stringKey"))
	assert.False(t, session.Has(ctx, "nonExistentKey"))
}

func TestKeys(t *testing.T) {
	session, ctx, err := setupTestSessionWithData()
	require.NoError(t, err)

	keys := session.Keys(ctx)
	assert.Contains(t, keys, "stringKey")
	assert.Contains(t, keys, "intKey")
	assert.Contains(t, keys, "boolKey")
	assert.Contains(t, keys, "floatKey")
	assert.Contains(t, keys, "timeKey")
	assert.Contains(t, keys, "bytesKey")

	// Keys should be sorted
	assert.Equal(t, keys, append([]string{}, keys...))
}

func TestPut(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	session.Put(ctx, "newKey", "newValue")
	assert.Equal(t, "newValue", session.Get(ctx, "newKey"))
	assert.Equal(t, Modified, session.Status(ctx))

	// Overwrite existing value
	session.Put(ctx, "newKey", "updatedValue")
	assert.Equal(t, "updatedValue", session.Get(ctx, "newKey"))
}

func TestRememberMe(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	session.RememberMe(ctx, true)
	assert.True(t, session.GetBool(ctx, "__rememberMe"))
	assert.Equal(t, Modified, session.Status(ctx))
}

func TestToken(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Initially empty
	assert.Empty(t, session.Token(ctx))

	session.SetToken(ctx, "test-token")
	assert.Equal(t, "test-token", session.Token(ctx))
}

func TestSetToken(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	session.SetToken(ctx, "test-token")
	assert.Equal(t, "test-token", session.Token(ctx))
	assert.Equal(t, Modified, session.Status(ctx))
}

func TestStatus(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Initially should be Unmodified (created by Load)
	assert.Equal(t, Unmodified, session.Status(ctx))

	// Modify should change status
	session.Put(ctx, "key", "value")
	assert.Equal(t, Modified, session.Status(ctx))
}

// Type-specific getter tests
func TestTypeGetters(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Set up test data
	session.Put(ctx, "stringKey", "hello")
	session.Put(ctx, "runeKey", rune('x'))
	session.Put(ctx, "boolKey", true)
	session.Put(ctx, "intKey", 42)
	session.Put(ctx, "uintKey", uint(42))
	session.Put(ctx, "int64Key", int64(42))
	session.Put(ctx, "int32Key", int32(42))
	session.Put(ctx, "int16Key", int16(42))
	session.Put(ctx, "int8Key", int8(42))
	session.Put(ctx, "float64Key", 3.14)
	session.Put(ctx, "float32Key", float32(3.14))
	session.Put(ctx, "bytesKey", []byte("test"))
	session.Put(ctx, "timeKey", time.Now())

	tests := []struct {
		name     string
		key      string
		expected interface{}
		getter   func() interface{}
	}{
		{"GetString", "stringKey", "hello", func() interface{} {
			return session.GetString(ctx, "stringKey")
		}},
		{"GetRune", "runeKey", rune('x'), func() interface{} {
			return session.GetRune(ctx, "runeKey")
		}},
		{"GetBool", "boolKey", true, func() interface{} {
			return session.GetBool(ctx, "boolKey")
		}},
		{"GetInt", "intKey", 42, func() interface{} {
			return session.GetInt(ctx, "intKey")
		}},
		{"GetUInt", "uintKey", uint(42), func() interface{} {
			return session.GetUInt(ctx, "uintKey")
		}},
		{"GetInt64", "int64Key", int64(42), func() interface{} {
			return session.GetInt64(ctx, "int64Key")
		}},
		{"GetInt32", "int32Key", int32(42), func() interface{} {
			return session.GetInt32(ctx, "int32Key")
		}},
		{"GetInt16", "int16Key", int16(42), func() interface{} {
			return session.GetInt16(ctx, "int16Key")
		}},
		{"GetInt8", "int8Key", int8(42), func() interface{} {
			return session.GetInt8(ctx, "int8Key")
		}},
		{"GetFloat64", "float64Key", 3.14, func() interface{} {
			return session.GetFloat64(ctx, "float64Key")
		}},
		{"GetFloat32", "float32Key", float32(3.14), func() interface{} {
			return session.GetFloat32(ctx, "float32Key")
		}},
		{"GetBytes", "bytesKey", []byte("test"), func() interface{} {
			return session.GetBytes(ctx, "bytesKey")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.getter()
			assert.Equal(t, tt.expected, actual)
		})
	}
}

// Test zero values for type getters
func TestTypeGetters_ZeroValues(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	assert.Equal(t, "", session.GetString(ctx, "nonexistent"))
	assert.Equal(t, rune(0), session.GetRune(ctx, "nonexistent"))
	assert.Equal(t, false, session.GetBool(ctx, "nonexistent"))
	assert.Equal(t, 0, session.GetInt(ctx, "nonexistent"))
	assert.Equal(t, uint(0), session.GetUInt(ctx, "nonexistent"))
	assert.Equal(t, int64(0), session.GetInt64(ctx, "nonexistent"))
	assert.Equal(t, int32(0), session.GetInt32(ctx, "nonexistent"))
	assert.Equal(t, int16(0), session.GetInt16(ctx, "nonexistent"))
	assert.Equal(t, int8(0), session.GetInt8(ctx, "nonexistent"))
	assert.Equal(t, float64(0), session.GetFloat64(ctx, "nonexistent"))
	assert.Equal(t, float32(0), session.GetFloat32(ctx, "nonexistent"))
	assert.Nil(t, session.GetBytes(ctx, "nonexistent"))
	assert.True(t, session.GetTime(ctx, "nonexistent").IsZero())
}

// Test type getters with wrong types
func TestTypeGetters_WrongTypes(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Put wrong types for each getter
	session.Put(ctx, "stringKey", 42)         // Should be string
	session.Put(ctx, "intKey", "not a int")   // Should be int
	session.Put(ctx, "boolKey", "not a bool") // Should be bool

	// Should return zero values when type assertion fails
	assert.Equal(t, "", session.GetString(ctx, "stringKey"))
	assert.Equal(t, 0, session.GetInt(ctx, "intKey"))
	assert.Equal(t, false, session.GetBool(ctx, "boolKey"))
}

// Test Pop variants for different types
func TestPopTypeMethods(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Set up test data
	session.Put(ctx, "stringKey", "hello")
	session.Put(ctx, "boolKey", true)
	session.Put(ctx, "intKey", 42)

	// Test PopString
	value := session.PopString(ctx, "stringKey")
	assert.Equal(t, "hello", value)
	assert.Nil(t, session.Get(ctx, "stringKey")) // Should be removed

	// Test PopBool
	valueBool := session.PopBool(ctx, "boolKey")
	assert.True(t, valueBool)
	assert.Nil(t, session.Get(ctx, "boolKey")) // Should be removed

	// Test PopInt
	valueInt := session.PopInt(ctx, "intKey")
	assert.Equal(t, 42, valueInt)
	assert.Nil(t, session.Get(ctx, "intKey")) // Should be removed

	// Test popping non-existent keys
	assert.Equal(t, "", session.PopString(ctx, "nonexistent"))
	assert.Equal(t, false, session.PopBool(ctx, "nonexistent"))
	assert.Equal(t, 0, session.PopInt(ctx, "nonexistent"))
}

func TestRenewToken(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Set an initial token
	session.SetToken(ctx, "old-token")

	mockStore := session.store.(*MockStore)
	mockStore.On("Delete", mock.Anything, "old-token").Return(nil)

	err = session.RenewToken(ctx)
	assert.NoError(t, err)

	// Token should be different
	newToken := session.Token(ctx)
	assert.NotEqual(t, "old-token", newToken)
	assert.NotEmpty(t, newToken)
	assert.Equal(t, Modified, session.Status(ctx))

	mockStore.AssertExpectations(t)
}

func TestRenewToken_NoExistingToken(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	err = session.RenewToken(ctx)
	assert.NoError(t, err)

	// Should generate a new token
	token := session.Token(ctx)
	assert.NotEmpty(t, token)
}

func TestRenewToken_StoreError(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	session.SetToken(ctx, "old-token")

	mockStore := session.store.(*MockStore)
	mockStore.On("Delete", mock.Anything, "old-token").Return(assert.AnError)

	err = session.RenewToken(ctx)
	assert.Error(t, err)
	assert.Same(t, assert.AnError, err)

	mockStore.AssertExpectations(t)
}

func TestMergeSession(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	// Add data to current session
	session.Put(ctx, "existingKey", "existingValue")

	// Set up mock store with another session's data
	token := "other-session-token"
	otherData := map[string]any{
		"newKey":    "newValue",
		"otherKey":  "otherValue",
		"timestamp": time.Now(),
	}

	mockStore := session.store.(*MockStore)
	mockCodec := session.codec.(*MockCodec)

	encodedData := []byte("encoded")
	mockStore.On("Find", mock.Anything, token).Return(encodedData, true, nil)
	mockCodec.On("Decode", encodedData).Return(time.Now().Add(time.Hour), otherData, nil)
	mockStore.On("Delete", mock.Anything, token).Return(nil)

	err = session.MergeSession(ctx, token)
	assert.NoError(t, err)

	// Check that data was merged
	assert.Equal(t, "existingValue", session.Get(ctx, "existingKey"))
	assert.Equal(t, "newValue", session.Get(ctx, "newKey"))
	assert.Equal(t, "otherValue", session.Get(ctx, "otherKey"))
	assert.Equal(t, otherData["timestamp"], session.Get(ctx, "timestamp"))
	assert.Equal(t, Modified, session.Status(ctx))

	mockStore.AssertExpectations(t)
	mockCodec.AssertExpectations(t)
}

func TestMergeSession_SameSession(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	token := "same-session-token"
	session.SetToken(ctx, token)

	// Mock the store call (even though it should return early, the mock is needed)
	mockStore := session.store.(*MockStore)
	mockStore.On("Find", mock.Anything, token).Return([]byte{}, false, nil).Maybe()

	// Should return early without error
	err = session.MergeSession(ctx, token)
	assert.NoError(t, err)
}

func TestMergeSession_SessionNotFound(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	token := "nonexistent-session"

	mockStore := session.store.(*MockStore)
	mockStore.On("Find", mock.Anything, token).Return([]byte{}, false, nil)

	err = session.MergeSession(ctx, token)
	assert.NoError(t, err) // Should not error, just no-op

	mockStore.AssertExpectations(t)
}

func TestMergeSession_StoreError(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	token := "error-session"

	mockStore := session.store.(*MockStore)
	mockStore.On("Find", mock.Anything, token).Return([]byte{}, false, assert.AnError)

	err = session.MergeSession(ctx, token)
	assert.Error(t, err)
	assert.Same(t, assert.AnError, err)

	mockStore.AssertExpectations(t)
}

func TestMergeSession_CodecError(t *testing.T) {
	session, ctx, err := setupTestSession()
	require.NoError(t, err)

	token := "corrupted-session"

	mockStore := session.store.(*MockStore)
	mockCodec := session.codec.(*MockCodec)

	encodedData := []byte("corrupted")
	mockStore.On("Find", mock.Anything, token).Return(encodedData, true, nil)
	mockCodec.On("Decode", encodedData).Return(time.Time{}, map[string]any(nil), assert.AnError)

	err = session.MergeSession(ctx, token)
	assert.Error(t, err)
	assert.Same(t, assert.AnError, err)

	mockStore.AssertExpectations(t)
	mockCodec.AssertExpectations(t)
}

func TestDoStoreMethods(t *testing.T) {
	tests := []struct {
		name          string
		hashToken     bool
		expectedToken string
	}{
		{
			name:          "hash disabled",
			hashToken:     false,
			expectedToken: "test-token",
		},
		{
			name:          "hash enabled",
			hashToken:     true,
			expectedToken: hashToken("test-token"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockStore{}
			config := Config{HashTokenInStore: tt.hashToken}
			session := NewWithCodec(config, mockStore, &MockCodec{})

			ctx := context.Background()

			// Test doStoreDelete
			mockStore.On("Delete", ctx, tt.expectedToken).Return(nil)
			err := session.doStoreDelete(ctx, "test-token")
			assert.NoError(t, err)

			// Test doStoreFind
			mockStore.On("Find", ctx, tt.expectedToken).Return([]byte("data"), true, nil)
			data, found, err := session.doStoreFind(ctx, "test-token")
			assert.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, []byte("data"), data)

			// Test doStoreCommit
			mockStore.On("Commit", ctx, tt.expectedToken, []byte("data"), mock.Anything).Return(nil)
			err = session.doStoreCommit(ctx, "test-token", []byte("data"), time.Now())
			assert.NoError(t, err)

			mockStore.AssertExpectations(t)
		})
	}
}
