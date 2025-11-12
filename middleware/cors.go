package middleware

import (
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gowool/wo"
)

type CORSConfig struct {
	// AllowOrigins determines the value of the Access-Control-Allow-Origin
	// response header.  This header defines a list of origins that may access the
	// resource.  The wildcard characters '*' and '?' are supported and are
	// converted to regex fragments '.*' and '.' accordingly.
	//
	// Security: use extreme caution when handling the origin, and carefully
	// validate any logic. Remember that attackers may register hostile domain names.
	// See https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html
	//
	// Optional. Default value []string{"*"}.
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Origin
	AllowOrigins []string `env:"ALLOW_ORIGINS" json:"allowOrigins,omitempty" yaml:"allowOrigins,omitempty"`

	// AllowOriginFunc is a custom function to validate the origin. It takes the
	// origin as an argument and returns true if allowed or false otherwise. If
	// an error is returned, it is returned by the handler. If this option is
	// set, AllowOrigins is ignored.
	//
	// Security: use extreme caution when handling the origin, and carefully
	// validate any logic. Remember that attackers may register hostile domain names.
	// See https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html
	//
	// Optional.
	AllowOriginFunc func(origin string) (bool, error) `json:"-" yaml:"-"`

	// AllowMethods determines the value of the Access-Control-Allow-Methods
	// response header.  This header specified the list of methods allowed when
	// accessing the resource.  This is used in response to a preflight request.
	//
	// Optional. Default value DefaultCORSConfig.AllowMethods.
	// If `allowMethods` is left empty, this middleware will fill for preflight
	// request `Access-Control-Allow-Methods` header value
	// from `Allow` header that echo.Router set into context.
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Methods
	AllowMethods []string `env:"ALLOW_METHODS" json:"allowMethods,omitempty" yaml:"allowMethods,omitempty"`

	// AllowHeaders determines the value of the Access-Control-Allow-Headers
	// response header.  This header is used in response to a preflight request to
	// indicate which HTTP headers can be used when making the actual request.
	//
	// Optional. Default value []string{}.
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Headers
	AllowHeaders []string `env:"ALLOW_HEADERS" json:"allowHeaders,omitempty" yaml:"allowHeaders,omitempty"`

	// AllowCredentials determines the value of the
	// Access-Control-Allow-Credentials response header.  This header indicates
	// whether or not the response to the request can be exposed when the
	// credentials mode (Request.credentials) is true. When used as part of a
	// response to a preflight request, this indicates whether or not the actual
	// request can be made using credentials.  See also
	// [MDN: Access-Control-Allow-Credentials].
	//
	// Optional. Default value false, in which case the header is not set.
	//
	// Security: avoid using `AllowCredentials = true` with `AllowOrigins = *`.
	// See "Exploiting CORS misconfigurations for Bitcoins and bounties",
	// https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Credentials
	AllowCredentials bool `env:"ALLOW_CREDENTIALS" json:"allowCredentials,omitempty" yaml:"allowCredentials,omitempty"`

	// UnsafeWildcardOriginWithAllowCredentials UNSAFE/INSECURE: allows wildcard '*' origin to be used with AllowCredentials
	// flag. In that case we consider any origin allowed and send it back to the client with `Access-Control-Allow-Origin` header.
	//
	// This is INSECURE and potentially leads to [cross-origin](https://portswigger.net/research/exploiting-cors-misconfigurations-for-bitcoins-and-bounties)
	// attacks. See: https://github.com/labstack/echo/issues/2400 for discussion on the subject.
	//
	// Optional. Default value is false.
	UnsafeWildcardOriginWithAllowCredentials bool `env:"UNSAFE_WILDCARD_ORIGIN_WITH_ALLOW_CREDENTIALS" json:"unsafeWildcardOriginWithAllowCredentials,omitempty" yaml:"unsafeWildcardOriginWithAllowCredentials,omitempty"`

	// ExposeHeaders determines the value of Access-Control-Expose-Headers, which
	// defines a list of headers that clients are allowed to access.
	//
	// Optional. Default value []string{}, in which case the header is not set.
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Expose-Header
	ExposeHeaders []string `env:"EXPOSE_HEADERS" json:"exposeHeaders,omitempty" yaml:"exposeHeaders,omitempty"`

	// MaxAge determines the value of the Access-Control-Max-Age response header.
	// This header indicates how long (in seconds) the results of a preflight
	// request can be cached.
	// The header is set only if MaxAge != 0, negative value sends "0" which instructs browsers not to cache that response.
	//
	// Optional. Default value 0 - meaning header is not sent.
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Max-Age
	MaxAge int `env:"MAX_AGE" json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
}

func (c *CORSConfig) SetDefaults() {
	if len(c.AllowOrigins) == 0 {
		c.AllowOrigins = []string{"*"}
	}

	if c.AllowMethods == nil {
		c.AllowMethods = []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPut,
			http.MethodPatch,
			http.MethodPost,
			http.MethodDelete,
		}
	}
}

func CORS[T wo.Resolver](cfg CORSConfig, skippers ...Skipper[T]) func(T) error {
	cfg.SetDefaults()

	skip := ChainSkipper[T](skippers...)

	allowOriginPatterns := make([]*regexp.Regexp, 0, len(cfg.AllowOrigins))
	for _, origin := range cfg.AllowOrigins {
		if origin == "*" {
			continue // "*" is handled differently and does not need regexp
		}
		pattern := regexp.QuoteMeta(origin)
		pattern = strings.ReplaceAll(pattern, "\\*", ".*")
		pattern = strings.ReplaceAll(pattern, "\\?", ".")
		pattern = "^" + pattern + "$"

		re, err := regexp.Compile(pattern)
		if err != nil {
			// this is to preserve previous behaviour - invalid patterns were just ignored.
			// If we would turn this to panic, users with invalid patterns
			// would have applications crashing in production due unrecovered panic.
			log.Println("invalid AllowOrigins pattern", origin)
			continue
		}
		allowOriginPatterns = append(allowOriginPatterns, re)
	}

	allowMethods := strings.Join(cfg.AllowMethods, ",")
	allowHeaders := strings.Join(cfg.AllowHeaders, ",")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ",")

	maxAge := "0"
	if cfg.MaxAge > 0 {
		maxAge = strconv.Itoa(cfg.MaxAge)
	}

	return func(e T) error {
		if skip(e) {
			return e.Next()
		}

		req := e.Request()
		res := e.Response()
		origin := req.Header.Get(wo.HeaderOrigin)
		allowOrigin := ""

		res.Header().Add(wo.HeaderVary, wo.HeaderOrigin)

		// Preflight request is an OPTIONS request, using three HTTP request headers: Access-Control-Request-Method,
		// Access-Control-Request-Headers, and the Origin header. See: https://developer.mozilla.org/en-US/docs/Glossary/Preflight_request
		// For simplicity we just consider method type and later `Origin` header.
		preflight := req.Method == http.MethodOptions

		// No Origin provided. This is (probably) not request from actual browser - proceed executing middleware chain
		if origin == "" {
			if !preflight {
				return e.Next()
			}
			res.WriteHeader(http.StatusNoContent)
			return nil
		}

		if cfg.AllowOriginFunc != nil {
			allowed, err := cfg.AllowOriginFunc(origin)
			if err != nil {
				return err
			}
			if allowed {
				allowOrigin = origin
			}
		} else {
			// Check allowed origins
			for _, o := range cfg.AllowOrigins {
				if o == "*" && cfg.AllowCredentials && cfg.UnsafeWildcardOriginWithAllowCredentials {
					allowOrigin = origin
					break
				}
				if o == "*" || o == origin {
					allowOrigin = o
					break
				}
				if matchSubdomain(origin, o) {
					allowOrigin = origin
					break
				}
			}

			checkPatterns := false
			if allowOrigin == "" {
				// to avoid regex cost by invalid (long) domains (253 is domain name max limit)
				if len(origin) <= (253+3+5) && strings.Contains(origin, "://") {
					checkPatterns = true
				}
			}
			if checkPatterns {
				for _, re := range allowOriginPatterns {
					if match := re.MatchString(origin); match {
						allowOrigin = origin
						break
					}
				}
			}
		}

		// Origin not allowed
		if allowOrigin == "" {
			if !preflight {
				return e.Next()
			}
			res.WriteHeader(http.StatusNoContent)
			return nil
		}

		res.Header().Set(wo.HeaderAccessControlAllowOrigin, allowOrigin)
		if cfg.AllowCredentials {
			res.Header().Set(wo.HeaderAccessControlAllowCredentials, "true")
		}

		// Simple request
		if !preflight {
			if exposeHeaders != "" {
				res.Header().Set(wo.HeaderAccessControlExposeHeaders, exposeHeaders)
			}
			return e.Next()
		}

		// Preflight request
		res.Header().Add(wo.HeaderVary, wo.HeaderAccessControlRequestMethod)
		res.Header().Add(wo.HeaderVary, wo.HeaderAccessControlRequestHeaders)
		res.Header().Set(wo.HeaderAccessControlAllowMethods, allowMethods)

		if allowHeaders != "" {
			res.Header().Set(wo.HeaderAccessControlAllowHeaders, allowHeaders)
		} else {
			h := req.Header.Get(wo.HeaderAccessControlRequestHeaders)
			if h != "" {
				res.Header().Set(wo.HeaderAccessControlAllowHeaders, h)
			}
		}
		if cfg.MaxAge != 0 {
			res.Header().Set(wo.HeaderAccessControlMaxAge, maxAge)
		}

		res.WriteHeader(http.StatusNoContent)
		return nil
	}
}

func matchScheme(domain, pattern string) bool {
	didx := strings.Index(domain, ":")
	pidx := strings.Index(pattern, ":")
	return didx != -1 && pidx != -1 && domain[:didx] == pattern[:pidx]
}

// matchSubdomain compares authority with wildcard
func matchSubdomain(domain, pattern string) bool {
	if !matchScheme(domain, pattern) {
		return false
	}
	didx := strings.Index(domain, "://")
	pidx := strings.Index(pattern, "://")
	if didx == -1 || pidx == -1 {
		return false
	}
	domAuth := domain[didx+3:]
	// to avoid long loop by invalid long domain
	if len(domAuth) > 253 {
		return false
	}
	patAuth := pattern[pidx+3:]

	domComp := strings.Split(domAuth, ".")
	patComp := strings.Split(patAuth, ".")
	for i := len(domComp)/2 - 1; i >= 0; i-- {
		opp := len(domComp) - 1 - i
		domComp[i], domComp[opp] = domComp[opp], domComp[i]
	}
	for i := len(patComp)/2 - 1; i >= 0; i-- {
		opp := len(patComp) - 1 - i
		patComp[i], patComp[opp] = patComp[opp], patComp[i]
	}

	for i, v := range domComp {
		if len(patComp) <= i {
			return false
		}
		p := patComp[i]
		if p == "*" {
			return true
		}
		if p != v {
			return false
		}
	}
	return false
}
