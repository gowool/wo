package session

import (
	"context"
	"net/http"
	"time"

	"github.com/gowool/wo"
)

type Session struct {
	config Config
	store  Store
	codec  Codec

	// contextKey is the key used to set and retrieve the session data from a
	// context.Context. It's automatically generated to ensure uniqueness.
	contextKey contextKey
}

func New(cfg Config, store Store) *Session {
	return NewWithCodec(cfg, store, NewGobCodec())
}

func NewWithCodec(cfg Config, store Store, codec Codec) *Session {
	cfg.SetDefaults()

	return &Session{
		config:     cfg,
		store:      store,
		codec:      codec,
		contextKey: generateContextKey(),
	}
}

// ReadSessionCookie reads the session cookie from the HTTP request and
// loads the session data into the request context. If the cookie is
// invalid, it returns an error. The session data is stored in the
// request context under the key defined by the session's contextKey.
func (s *Session) ReadSessionCookie(r *http.Request) (*http.Request, error) {
	var token string
	if cookie, err := r.Cookie(s.config.Cookie.Name); err == nil {
		token = cookie.Value
	}

	ctx, err := s.Load(r.Context(), token)
	if err != nil {
		return r, err
	}

	return r.WithContext(ctx), nil
}

// WriteSessionCookie writes a cookie to the HTTP response with the provided
// token as the cookie value and expiry as the cookie expiry time. The expiry
// time will be included in the cookie only if the session is set to persist
// or has had RememberMe(true) called on it. If expiry is an empty time.Time
// struct (so that it's IsZero() method returns true) the cookie will be
// marked with a historical expiry time and negative max-age (so the browser
// deletes it).
func (s *Session) WriteSessionCookie(ctx context.Context, w http.ResponseWriter, token string, expiry time.Time) {
	cookie := &http.Cookie{
		HttpOnly:    true,
		Value:       token,
		Name:        s.config.Cookie.Name,
		Path:        s.config.Cookie.Path,
		Domain:      s.config.Cookie.Domain,
		Secure:      s.config.Cookie.Secure,
		Partitioned: s.config.Cookie.Partitioned,
		SameSite:    s.config.Cookie.SameSite.HTTP(),
	}

	if expiry.IsZero() {
		cookie.Expires = time.Unix(1, 0)
		cookie.MaxAge = -1
	} else if s.config.Cookie.Persist || s.GetBool(ctx, "__rememberMe") {
		cookie.Expires = time.Unix(expiry.Unix()+1, 0)        // Round up to the nearest second.
		cookie.MaxAge = int(time.Until(expiry).Seconds() + 1) // Round up to the nearest second.
	}

	w.Header().Set(wo.HeaderVary, "Cookie")
	w.Header().Add(wo.HeaderCacheControl, `no-cache="Set-Cookie"`)

	http.SetCookie(w, cookie)
}
