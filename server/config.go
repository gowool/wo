package server

import (
	"crypto/tls"
	"errors"
	"os"
	"time"

	"github.com/invopop/validation"
)

type Config struct {
	// Host and port to handle as https server.
	Address string `json:"address,omitempty" yaml:"address,omitempty"`

	// HTTP2 defines http/2 server options.
	HTTP2 *HTTP2Config `json:"http2,omitempty" yaml:"http2,omitempty"`

	// HTTP3 enables HTTP/3 protocol on the entryPoint. HTTP/3 requires a TCP entryPoint,
	// as HTTP/3 always starts as a TCP connection that then gets upgraded to UDP.
	// In most scenarios, this entryPoint is the same as the one used for TLS traffic.
	HTTP3 *HTTP3Config `json:"http3,omitempty" yaml:"http3,omitempty"`

	Transport TransportConfig `json:"transport,omitempty" yaml:"transport,omitempty"`

	TLS *TLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
}

func (c *Config) SetDefaults() {
	switch c.Address {
	case ":http":
		c.Address = ":80"
	case ":https":
		c.Address = ":443"
	case "":
		if c.TLS == nil {
			c.Address = ":8080"
		} else {
			c.Address = ":8443"
		}
	}

	if c.HTTP2 == nil {
		c.HTTP2 = &HTTP2Config{}
	}

	c.HTTP2.SetDefaults()
}

func (c *Config) Validate() error {
	return validation.ValidateStruct(c)
}

type HTTP2Config struct {
	// MaxConcurrentStreams specifies the number of concurrent
	// streams per connection that each client is allowed to initiate.
	// The MaxConcurrentStreams value must be greater than zero, defaults to 250.
	MaxConcurrentStreams uint `json:"maxConcurrentStreams,omitempty" yaml:"maxConcurrentStreams,omitempty"`
}

func (c *HTTP2Config) SetDefaults() {
	if c.MaxConcurrentStreams == 0 {
		c.MaxConcurrentStreams = 250
	}
}

type HTTP3Config struct {
	// AdvertisedPort defines which UDP port to advertise as the HTTP/3 authority.
	// It defaults to the entryPoint's address port. It can be used to override
	// the authority in the alt-svc header.
	AdvertisedPort uint `json:"advertisedPort,omitempty" yaml:"advertisedPort,omitempty"`
}

type TransportConfig struct {
	// ReadTimeout is the maximum duration for reading the entire
	// request, including the body. A zero or negative value means
	// there will be no timeout.
	//
	// Because ReadTimeout does not let Handlers make per-request
	// decisions on each request body's acceptable deadline or
	// upload rate, most users will prefer to use
	// ReadHeaderTimeout. It is valid to use them both.
	ReadTimeout time.Duration `json:"readTimeout,omitempty,format:units" yaml:"readTimeout,omitempty"`

	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers. The connection's read deadline is reset
	// after reading the headers and the Handler can decide what
	// is considered too slow for the body. If zero, the value of
	// ReadTimeout is used. If negative, or if zero and ReadTimeout
	// is zero or negative, there is no timeout.
	ReadHeaderTimeout time.Duration `json:"readHeaderTimeout,omitempty,format:units" yaml:"readHeaderTimeout,omitempty"`

	// WriteTimeout is the maximum duration before timing out
	// writes of the response. It is reset whenever a new
	// request's header is read. Like ReadTimeout, it does not
	// let Handlers make decisions on a per-request basis.
	// A zero or negative value means there will be no timeout.
	WriteTimeout time.Duration `json:"writeTimeout,omitempty,format:units" yaml:"writeTimeout,omitempty"`

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled. If zero, the value
	// of ReadTimeout is used. If negative, or if zero and ReadTimeout
	// is zero or negative, there is no timeout.
	IdleTimeout time.Duration `json:"idleTimeout,omitempty,format:units" yaml:"idleTimeout,omitempty"`

	// MaxHeaderBytes controls the maximum number of bytes the
	// server will read parsing the request header's keys and
	// values, including the request line. It does not limit the
	// size of the request body.
	// If zero, http.DefaultMaxHeaderBytes is used.
	MaxHeaderBytes int `json:"maxHeaderBytes,omitempty" yaml:"maxHeaderBytes,omitempty"`
}

type TLSConfig struct {
	InsecureSkipVerify bool                `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	Certificates       []CertificateConfig `json:"certificates,omitempty" yaml:"certificates,omitempty"`
}

func (c TLSConfig) Validate() error {
	return validation.ValidateStruct(&c, validation.Field(&c.Certificates, validation.Required))
}

func (c TLSConfig) tls() (*tls.Config, error) {
	var err error
	certificates := make([]tls.Certificate, len(c.Certificates))
	for i, cert := range c.Certificates {
		if certificates[i], err = cert.Certificate(); err != nil {
			return nil, err
		}
	}

	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.InsecureSkipVerify,
		Certificates:       certificates,
	}, nil
}

type CertificateConfig struct {
	CertFile string `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile  string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
}

func (c CertificateConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.CertFile, validation.Required),
		validation.Field(&c.KeyFile, validation.Required),
	)
}

func (c CertificateConfig) Certificate() (tls.Certificate, error) {
	if c.CertFile == "" {
		return tls.Certificate{}, errors.New("CertFile is empty")
	}
	if c.KeyFile == "" {
		return tls.Certificate{}, errors.New("KeyFile is empty")
	}
	if info, err := os.Stat(c.CertFile); err == nil {
		if info.IsDir() {
			return tls.Certificate{}, errors.New("CertFile is dir")
		}

		if info, err = os.Stat(c.KeyFile); err == nil {
			if info.IsDir() {
				return tls.Certificate{}, errors.New("KeyFile is dir")
			}
		}

		return tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	}
	return tls.X509KeyPair([]byte(c.CertFile), []byte(c.KeyFile))
}
