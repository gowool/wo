package middleware

import (
	"time"

	"github.com/gowool/wo"
	"github.com/gowool/wo/session"
)

type ErrorLogger interface {
	Error(msg string, keysAndValues ...any)
}

func Session[T wo.Resolver](s *session.Session, logger ErrorLogger, skippers ...Skipper[T]) func(T) error {
	if s == nil {
		panic("session middleware: session is nil")
	}

	skip := ChainSkipper[T](skippers...)

	return func(e T) error {
		if skip(e) {
			return e.Next()
		}

		r, err := s.ReadSessionCookie(e.Request())
		if err != nil {
			return err
		}

		e.SetRequest(r)
		e.Response().Before(func() {
			ctx := e.Request().Context()

			switch s.Status(ctx) {
			case session.Modified:
				token, expiry, err := s.Commit(ctx)
				if err != nil {
					if logger != nil {
						logger.Error("failed to commit session", "error", err)
					}
					return
				}

				s.WriteSessionCookie(ctx, e.Response(), token, expiry)
			case session.Destroyed:
				s.WriteSessionCookie(ctx, e.Response(), "", time.Time{})
			default:
			}
		})

		return e.Next()
	}
}
