package middleware

import (
	"fmt"

	"github.com/gowool/wo"
)

type SecurityConfig struct {
	// XSSProtection provides protection against cross-site scripting attack (XSS)
	// by setting the `X-XSS-Protection` header.
	// Optional. Default value "1; mode=block".
	XSSProtection string `json:"xssProtection,omitempty" yaml:"xssProtection,omitempty"`

	// ContentTypeNosniff provides protection against overriding Content-Type
	// header by setting the `X-Content-Type-Options` header.
	// Optional. Default value "nosniff".
	ContentTypeNosniff string `json:"contentTypeNosniff,omitempty" yaml:"contentTypeNosniff,omitempty"`

	// XFrameOptions can be used to indicate whether or not a browser should
	// be allowed to render a page in a <frame>, <iframe> or <object> .
	// Sites can use this to avoid clickjacking attacks, by ensuring that their
	// content is not embedded into other sites.provides protection against
	// clickjacking.
	// Optional. Default value "SAMEORIGIN".
	// Possible values:
	// - "SAMEORIGIN" - The page can only be displayed in a frame on the same origin as the page itself.
	// - "DENY" - The page cannot be displayed in a frame, regardless of the site attempting to do so.
	// - "ALLOW-FROM uri" - The page can only be displayed in a frame on the specified origin.
	XFrameOptions string `json:"xFrameOptions,omitempty" yaml:"xFrameOptions,omitempty"`

	// HSTSMaxAge sets the `Strict-Transport-Security` header to indicate how
	// long (in seconds) browsers should remember that this site is only to
	// be accessed using HTTPS. This reduces your exposure to some SSL-stripping
	// man-in-the-middle (MITM) attacks.
	// Optional. Default value 15724800.
	HSTSMaxAge int `json:"hstsMaxAge,omitempty" yaml:"hstsMaxAge,omitempty"`

	// HSTSExcludeSubdomains won't include subdomains tag in the `Strict Transport Security`
	// header, excluding all subdomains from security policy. It has no effect
	// unless HSTSMaxAge is set to a non-zero value.
	// Optional. Default value false.
	HSTSExcludeSubdomains bool `json:"hstsExcludeSubdomains,omitempty" yaml:"hstsExcludeSubdomains,omitempty"`

	// ContentSecurityPolicy sets the `Content-Security-Policy` header providing
	// security against cross-site scripting (XSS), clickjacking and other code
	// injection attacks resulting from execution of malicious content in the
	// trusted web page context.
	// Optional. Default value "".
	ContentSecurityPolicy string `json:"contentSecurityPolicy,omitempty" yaml:"contentSecurityPolicy,omitempty"`

	// CSPReportOnly would use the `Content-Security-Policy-Report-Only` header instead
	// of the `Content-Security-Policy` header. This allows iterative updates of the
	// content security policy by only reporting the violations that would
	// have occurred instead of blocking the resource.
	// Optional. Default value false.
	CSPReportOnly bool `json:"cspReportOnly,omitempty" yaml:"cspReportOnly,omitempty"`

	// HSTSPreloadEnabled will add the preload tag in the `Strict Transport Security`
	// header, which enables the domain to be included in the HSTS preload list
	// maintained by Chrome (and used by Firefox and Safari): https://hstspreload.org/
	// Optional.  Default value false.
	HSTSPreloadEnabled bool `json:"hstsPreloadEnabled,omitempty" yaml:"hstsPreloadEnabled,omitempty"`

	// ReferrerPolicy sets the `Referrer-Policy` header providing security against
	// leaking potentially sensitive request paths to third parties.
	// Optional. Default value "".
	ReferrerPolicy string `json:"referrerPolicy,omitempty" yaml:"referrerPolicy,omitempty"`
}

func (c *SecurityConfig) SetDefaults() {
	if c.XSSProtection == "" {
		c.XSSProtection = "1; mode=block"
	}
	if c.ContentTypeNosniff == "" {
		c.ContentTypeNosniff = "nosniff"
	}
	if c.XFrameOptions == "" {
		c.XFrameOptions = "SAMEORIGIN"
	}
	if c.HSTSMaxAge <= 0 {
		c.HSTSMaxAge = 15724800
	}
}

func Security[E event](cfg SecurityConfig, skippers ...Skipper[E]) func(E) error {
	cfg.SetDefaults()

	skip := ChainSkipper[E](skippers...)

	return func(e E) error {
		if skip(e) {
			return e.Next()
		}

		req := e.Request()
		res := e.Response()

		if cfg.XSSProtection != "" {
			res.Header().Set(wo.HeaderXXSSProtection, cfg.XSSProtection)
		}

		if cfg.ContentTypeNosniff != "" {
			res.Header().Set(wo.HeaderXContentTypeOptions, cfg.ContentTypeNosniff)
		}

		if cfg.XFrameOptions != "" {
			res.Header().Set(wo.HeaderXFrameOptions, cfg.XFrameOptions)
		}

		if (e.Request().TLS != nil || (req.Header.Get(wo.HeaderXForwardedProto) == "https")) && cfg.HSTSMaxAge != 0 {
			subdomains := ""
			if !cfg.HSTSExcludeSubdomains {
				subdomains = "; includeSubdomains"
			}
			if cfg.HSTSPreloadEnabled {
				subdomains = fmt.Sprintf("%s; preload", subdomains)
			}
			res.Header().Set(wo.HeaderStrictTransportSecurity, fmt.Sprintf("max-age=%d%s", cfg.HSTSMaxAge, subdomains))
		}

		if cfg.ContentSecurityPolicy != "" {
			if cfg.CSPReportOnly {
				res.Header().Set(wo.HeaderContentSecurityPolicyReportOnly, cfg.ContentSecurityPolicy)
			} else {
				res.Header().Set(wo.HeaderContentSecurityPolicy, cfg.ContentSecurityPolicy)
			}
		}

		if cfg.ReferrerPolicy != "" {
			res.Header().Set(wo.HeaderReferrerPolicy, cfg.ReferrerPolicy)
		}

		return e.Next()
	}
}
