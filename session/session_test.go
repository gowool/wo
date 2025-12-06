package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gowool/wo"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		store   Store
		wantNil bool
	}{
		{
			name:    "valid config and store",
			config:  Config{},
			store:   &MockStore{},
			wantNil: false,
		},
		{
			name:    "nil store",
			config:  Config{},
			store:   nil,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.config, tt.store)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.store, got.store)
				assert.NotNil(t, got.codec)
				// Check that defaults were applied
				assert.Equal(t, "session", got.config.Cookie.Name)
				assert.Equal(t, "/", got.config.Cookie.Path)
				assert.Equal(t, SameSiteLax, got.config.Cookie.SameSite)
				assert.Equal(t, 24*time.Hour, got.config.Lifetime)
			}
		})
	}
}

func TestNewWithCodec(t *testing.T) {
	config := Config{}
	store := &MockStore{}
	codec := &MockCodec{}

	got := NewWithCodec(config, store, codec)
	assert.NotNil(t, got)
	assert.Equal(t, store, got.store)
	assert.Equal(t, codec, got.codec)
	// Check that defaults were applied
	assert.Equal(t, "session", got.config.Cookie.Name)
	assert.Equal(t, "/", got.config.Cookie.Path)
	assert.Equal(t, SameSiteLax, got.config.Cookie.SameSite)
	assert.Equal(t, 24*time.Hour, got.config.Lifetime)
}

func TestReadSessionCookie(t *testing.T) {
	tests := []struct {
		name          string
		cookieValue   string
		storeResponse []byte
		storeFound    bool
		storeError    error
		wantErr       bool
	}{
		{
			name:          "valid cookie",
			cookieValue:   "test-token",
			storeResponse: []byte("test-data"),
			storeFound:    true,
			storeError:    nil,
			wantErr:       false,
		},
		{
			name:          "no cookie",
			cookieValue:   "",
			storeResponse: nil,
			storeFound:    false,
			storeError:    nil,
			wantErr:       false,
		},
		{
			name:          "store error",
			cookieValue:   "test-token",
			storeResponse: nil,
			storeFound:    false,
			storeError:    assert.AnError,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockStore{}
			mockCodec := &MockCodec{}

			if tt.cookieValue != "" {
				mockStore.On("Find", mock.Anything, mock.Anything).Return(tt.storeResponse, tt.storeFound, tt.storeError)
			}

			if tt.storeFound {
				mockCodec.On("Decode", tt.storeResponse).Return(time.Now().Add(time.Hour), make(map[string]any), nil)
			}

			config := Config{}
			config.SetDefaults()
			session := NewWithCodec(config, mockStore, mockCodec)

			req := httptest.NewRequest("GET", "/", nil)
			if tt.cookieValue != "" {
				req.AddCookie(&http.Cookie{
					Name:  config.Cookie.Name,
					Value: tt.cookieValue,
				})
			}

			got, err := session.ReadSessionCookie(req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Same(t, tt.storeError, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				// Check that context was updated
				sessionData := got.Context().Value(session.contextKey)
				assert.NotNil(t, sessionData)
			}

			mockStore.AssertExpectations(t)
			mockCodec.AssertExpectations(t)
		})
	}
}

func TestWriteSessionCookie(t *testing.T) {
	tests := []struct {
		name         string
		persist      bool
		rememberMe   bool
		expiry       time.Time
		expectedPath string
	}{
		{
			name:         "zero expiry (deleted cookie)",
			persist:      false,
			rememberMe:   false,
			expiry:       time.Time{},
			expectedPath: "/",
		},
		{
			name:         "persist enabled",
			persist:      true,
			rememberMe:   false,
			expiry:       time.Now().Add(time.Hour),
			expectedPath: "/",
		},
		{
			name:         "remember me true",
			persist:      false,
			rememberMe:   true,
			expiry:       time.Now().Add(time.Hour),
			expectedPath: "/",
		},
		{
			name:         "no persist and no remember me",
			persist:      false,
			rememberMe:   false,
			expiry:       time.Now().Add(time.Hour),
			expectedPath: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockStore{}
			config := Config{
				Cookie: Cookie{
					Name:        "test-session",
					Path:        "/",
					Persist:     tt.persist,
					Secure:      false,
					Partitioned: false,
					SameSite:    SameSiteLax,
				},
			}
			session := New(config, mockStore)

			ctx := context.Background()
			w := httptest.NewRecorder()

			// Always set up context to avoid panics
			ctx, err := session.Load(ctx, "")
			require.NoError(t, err)

			// Set remember me if needed
			if tt.rememberMe {
				session.RememberMe(ctx, true)
			}

			token := "test-token"
			session.WriteSessionCookie(ctx, w, token, tt.expiry)

			cookies := w.Result().Cookies()
			require.Len(t, cookies, 1)

			cookie := cookies[0]
			assert.Equal(t, token, cookie.Value)
			assert.Equal(t, config.Cookie.Name, cookie.Name)
			assert.Equal(t, config.Cookie.Path, cookie.Path)
			assert.Equal(t, config.Cookie.Secure, cookie.Secure)
			assert.Equal(t, config.Cookie.SameSite.HTTP(), cookie.SameSite)

			if tt.expiry.IsZero() {
				// Use UTC for time comparison
				expectedExpiry := time.Unix(1, 0).UTC()
				assert.Equal(t, expectedExpiry, cookie.Expires.UTC())
				assert.Equal(t, -1, cookie.MaxAge)
			} else if tt.persist || tt.rememberMe {
				// Cookie should have positive max-age and future expiry
				assert.True(t, cookie.MaxAge > 0)
				assert.True(t, cookie.Expires.After(time.Now()))
			}

			// Check headers
			assert.Equal(t, "Cookie", w.Header().Get(wo.HeaderVary))
			assert.Contains(t, w.Header().Get(wo.HeaderCacheControl), `no-cache="Set-Cookie"`)
		})
	}
}

func TestWriteSessionCookie_CustomConfig(t *testing.T) {
	mockStore := &MockStore{}
	config := Config{
		Cookie: Cookie{
			Name:        "custom-session",
			Domain:      "example.com",
			Path:        "/api",
			Secure:      true,
			Partitioned: true,
			SameSite:    SameSiteStrict,
		},
	}
	session := New(config, mockStore)

	ctx := context.Background()
	// Set up context to avoid panics
	ctx, err := session.Load(ctx, "")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	token := "test-token"
	expiry := time.Now().Add(2 * time.Hour)

	session.WriteSessionCookie(ctx, w, token, expiry)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]
	assert.Equal(t, token, cookie.Value)
	assert.Equal(t, config.Cookie.Name, cookie.Name)
	assert.Equal(t, config.Cookie.Domain, cookie.Domain)
	assert.Equal(t, config.Cookie.Path, cookie.Path)
	assert.Equal(t, config.Cookie.Secure, cookie.Secure)
	assert.Equal(t, config.Cookie.Partitioned, cookie.Partitioned)
	assert.Equal(t, config.Cookie.SameSite.HTTP(), cookie.SameSite)
}

func TestWriteSessionCookie_DefaultConfig(t *testing.T) {
	mockStore := &MockStore{}
	config := Config{} // Empty config should use defaults
	session := New(config, mockStore)

	ctx := context.Background()
	// Set up context to avoid panics
	ctx, err := session.Load(ctx, "")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	token := "test-token"
	expiry := time.Now().Add(time.Hour)

	session.WriteSessionCookie(ctx, w, token, expiry)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]
	assert.Equal(t, "session", cookie.Name)                // Default name
	assert.Equal(t, "/", cookie.Path)                      // Default path
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite) // Default SameSite
	assert.True(t, cookie.HttpOnly)                        // Always HttpOnly
}

func TestReadSessionCookie_ContextAlreadyHasSession(t *testing.T) {
	mockStore := &MockStore{}
	config := Config{}
	config.SetDefaults()
	session := New(config, mockStore)

	// Simulate a context that already has session data by using Load
	req := httptest.NewRequest("GET", "/", nil)
	ctx, err := session.Load(req.Context(), "")
	require.NoError(t, err)

	req = req.WithContext(ctx)

	_, err = session.ReadSessionCookie(req)
	assert.NoError(t, err)

	// Store should not be called again if session already exists in context
	mockStore.AssertNotCalled(t, "Find")
}

func TestSessionContextKeyGeneration(t *testing.T) {
	mockStore := &MockStore{}
	config := Config{}

	session1 := New(config, mockStore)
	session2 := New(config, mockStore)

	assert.NotEqual(t, session1.contextKey, session2.contextKey, "Each session should have unique context key")
}
