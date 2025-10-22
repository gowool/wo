package middleware

import (
	"log/slog"
	"time"

	"github.com/gowool/wo/session"
)

func Session[E event](s *session.Session, logger *slog.Logger, skippers ...Skipper[E]) func(E) error {
	skip := ChainSkipper[E](skippers...)

	return func(e E) error {
		if skip(e) {
			return e.Next()
		}

		r, err := s.ReadSessionCookie(e.Request())
		if err != nil {
			return err
		}

		e.SetRequest(r)
		e.Response().Before(func() {
			switch s.Status(e.Request().Context()) {
			case session.Modified:
				token, expiry, err := s.Commit(e.Request().Context())
				if err != nil {
					logger.Error("failed to commit session", "error", err)
					return
				}

				s.WriteSessionCookie(e.Request().Context(), e.Response(), token, expiry)
			case session.Destroyed:
				s.WriteSessionCookie(e.Request().Context(), e.Response(), "", time.Time{})
			default:
			}
		})

		return e.Next()
	}
}
