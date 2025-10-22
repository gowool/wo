package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/gowool/wo/internal/must"
)

var redirectHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	target := &url.URL{
		Scheme:   "https",
		Host:     net.JoinHostPort(host, "443"),
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	http.Redirect(w, r, target.String(), http.StatusTemporaryRedirect)
})

type Server struct {
	cancel   context.CancelFunc
	logger   *slog.Logger
	http3    *http3.Server
	http2    *http.Server
	redirect *http.Server
	chErr    chan error
	wg       sync.WaitGroup
	mu       sync.Mutex
}

func NewServer(cfg Config, handler http.Handler, logger *slog.Logger) *Server {
	h2s := &http2.Server{MaxConcurrentStreams: uint32(cfg.HTTP2.MaxConcurrentStreams)}
	h2Handler := h2c.NewHandler(handler, h2s)

	ctx, cancel := context.WithCancel(context.Background())

	var redirect *http.Server
	if host, port, _ := net.SplitHostPort(cfg.Address); port == "443" {
		redirect = &http.Server{
			Addr:    net.JoinHostPort(host, "80"),
			Handler: redirectHandler,
			BaseContext: func(net.Listener) context.Context {
				return ctx
			},
		}
	}

	var (
		tlsConfig *tls.Config
		h3        *http3.Server
	)
	if cfg.TLS != nil {
		tlsConfig = must.Must(cfg.TLS.tls())

		if cfg.HTTP3 != nil {
			addr, portStr, _ := net.SplitHostPort(cfg.Address)
			port := int(cfg.HTTP3.AdvertisedPort)
			if port > 0 {
				portStr = strconv.Itoa(port)
			} else {
				port, _ = strconv.Atoi(portStr)
			}

			h3 = &http3.Server{
				TLSConfig:      http3.ConfigureTLSConfig(tlsConfig),
				Addr:           fmt.Sprintf("%s:%s", addr, portStr),
				Port:           port,
				Handler:        handler,
				IdleTimeout:    cfg.Transport.IdleTimeout,
				MaxHeaderBytes: cfg.Transport.MaxHeaderBytes,
				Logger:         logger.WithGroup("http3"),
			}
		}
	} else {
		logger.WarnContext(ctx, "TLS configuration is missing, starting server without TLS")
	}

	return &Server{
		logger:   logger,
		cancel:   cancel,
		chErr:    make(chan error, 6),
		redirect: redirect,
		http3:    h3,
		http2: &http.Server{
			TLSConfig:         tlsConfig,
			Addr:              cfg.Address,
			ReadHeaderTimeout: cfg.Transport.ReadHeaderTimeout,
			ReadTimeout:       cfg.Transport.ReadTimeout,
			WriteTimeout:      cfg.Transport.WriteTimeout,
			IdleTimeout:       cfg.Transport.IdleTimeout,
			MaxHeaderBytes:    cfg.Transport.MaxHeaderBytes,
			ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
			BaseContext: func(net.Listener) context.Context {
				return ctx
			},
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.ProtoMajor < 3 && h3 != nil {
					if err := h3.SetQUICHeaders(w.Header()); err != nil {
						logger.Error("set quic headers", "error", err)
					}
				}
				h2Handler.ServeHTTP(w, r)
			}),
		},
	}
}

func (s *Server) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.redirect != nil {
		s.wg.Go(func() {
			s.logger.Info("start redirect http", slog.String("address", s.redirect.Addr))

			s.chErr <- s.redirect.ListenAndServe()
		})
	}

	s.wg.Go(func() {
		s.logger.Info("start http2", slog.String("address", s.http2.Addr))

		if s.http2.TLSConfig == nil {
			s.chErr <- s.http2.ListenAndServe()
			return
		}

		s.chErr <- s.http2.ListenAndServeTLS("", "")
	})

	if s.http3 != nil {
		s.wg.Go(func() {
			s.logger.Info("start http3", slog.String("address", s.http3.Addr))

			s.chErr <- s.http3.ListenAndServe()
		})
	}
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.redirect != nil {
		s.wg.Go(func() {
			s.logger.Info("stop redirect http", slog.String("address", s.redirect.Addr))

			s.chErr <- s.redirect.Shutdown(ctx)
		})
	}

	s.wg.Go(func() {
		s.logger.Info("stop http2", slog.String("address", s.http2.Addr))

		s.chErr <- s.http2.Shutdown(ctx)
	})

	if s.http3 != nil {
		s.wg.Go(func() {
			s.logger.Info("stop http3", slog.String("address", s.http3.Addr))

			s.chErr <- s.http3.Shutdown(ctx)
		})
	}

	go func() {
		s.wg.Wait()
		close(s.chErr)
	}()

	s.cancel()

	var err error

	for {
		select {
		case <-ctx.Done():
			return nil
		case err1, ok := <-s.chErr:
			if !ok {
				if err != nil {
					s.logger.Error("shutdown", "error", err)
				}
				return err
			}
			if !errors.Is(err1, http.ErrServerClosed) {
				err = errors.Join(err, err1)
			}
		}
	}
}
