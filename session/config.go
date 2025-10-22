package session

import (
	"net/http"
	"time"
)

type SameSite string

const (
	SameSiteDefault SameSite = "default"
	SameSiteLax     SameSite = "lax"
	SameSiteStrict  SameSite = "strict"
	SameSiteNone    SameSite = "none"
)

func (s SameSite) String() string {
	return string(s)
}

func (s SameSite) HTTP() http.SameSite {
	switch s {
	case SameSiteDefault:
		return http.SameSiteDefaultMode
	case SameSiteLax:
		return http.SameSiteLaxMode
	case SameSiteStrict:
		return http.SameSiteStrictMode
	case SameSiteNone:
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

type Cookie struct {
	Name        string   `json:"name,omitempty" yaml:"name,omitempty"`
	Domain      string   `json:"domain,omitempty" yaml:"domain,omitempty"`
	Path        string   `json:"path,omitempty" yaml:"path,omitempty"`
	Persist     bool     `json:"persist,omitempty" yaml:"persist,omitempty"`
	Secure      bool     `json:"secure,omitempty" yaml:"secure,omitempty"`
	Partitioned bool     `json:"partitioned,omitempty" yaml:"partitioned,omitempty"`
	SameSite    SameSite `json:"sameSite,omitempty" yaml:"sameSite,omitempty"`
}

func (c *Cookie) SetDefaults() {
	if c.Name == "" {
		c.Name = "session"
	}
	if c.Path == "" {
		c.Path = "/"
	}
	if c.SameSite == "" {
		c.SameSite = SameSiteLax
	}
}

type Config struct {
	// IdleTimeout controls the maximum length of time a session can be inactive
	// before it expires. For example, some applications may wish to set this so
	// there is a timeout after 20 minutes of inactivity. By default IdleTimeout
	// is not set and there is no inactivity timeout.
	IdleTimeout time.Duration `json:"idleTimeout,omitempty" yaml:"idleTimeout,omitempty"`

	// Lifetime controls the maximum length of time that a session is valid for
	// before it expires. The lifetime is an 'absolute expiry' which is set when
	// the session is first created and does not change. The default value is 24
	// hours.
	Lifetime time.Duration `json:"lifetime,omitempty" yaml:"lifetime,omitempty"`

	// HashTokenInStore controls to store the session token or a hashed version in the store.
	HashTokenInStore bool `json:"hashTokenInStore,omitempty" yaml:"hashTokenInStore,omitempty"`

	// Cookie contains the configuration settings for session cookies.
	Cookie Cookie `json:"cookie,omitempty" yaml:"cookie,omitempty"`
}

func (c *Config) SetDefaults() {
	c.Cookie.SetDefaults()

	if c.Lifetime == 0 {
		c.Lifetime = 24 * time.Hour
	}
}
