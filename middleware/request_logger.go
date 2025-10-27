package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gowool/wo"
)

func RequestLogger[T wo.Resolver](logger *slog.Logger, attrFunc func(e T, status int, err error) []slog.Attr, skippers ...Skipper[T]) func(T) error {
	if logger == nil {
		panic("request logger middleware: logger is nil")
	}

	if attrFunc == nil {
		attrFunc = wo.RequestLoggerAttrs
	}

	skip := ChainSkipper[T](skippers...)

	return func(e T) error {
		if skip(e) {
			return e.Next()
		}

		err := e.Next()

		status := e.Response().Status
		if err != nil {
			var httpErr *wo.HTTPError
			if errors.As(err, &httpErr) {
				status = httpErr.Status
			} else {
				status = http.StatusInternalServerError
			}
		}

		attributes := attrFunc(e, status, err)

		var level slog.Level
		switch {
		case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
			level = slog.LevelWarn
		case status >= http.StatusInternalServerError:
			level = slog.LevelError
		default:
			level = slog.LevelInfo
		}

		logger.LogAttrs(context.Background(), level, "incoming request", attributes...)

		ctx := wo.WithRequestLogged(e.Request().Context(), true)
		e.SetRequest(e.Request().WithContext(ctx))

		return err
	}
}
