package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gowool/wo"
)

func RequestLogger[E event](logger *slog.Logger, attrFunc func(e E, status int, err error) []slog.Attr, skippers ...Skipper[E]) func(E) error {
	if attrFunc == nil {
		attrFunc = RequestLoggerAttrs
	}

	skip := ChainSkipper[E](skippers...)

	return func(e E) error {
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

		logger.LogAttrs(e.Request().Context(), level, "incoming request", attributes...)

		return err
	}
}

func RequestLoggerAttrs[E event](e E, status int, err error) []slog.Attr {
	req := e.Request()
	res := e.Response()

	p := req.URL.Path
	if p == "" {
		p = "/"
	}

	id := req.Header.Get(wo.HeaderXRequestID)
	if id == "" {
		id = res.Header().Get(wo.HeaderXRequestID)
	}

	n := 11
	if err != nil {
		n++
	}
	if id != "" {
		n++
	}

	attributes := make([]slog.Attr, 0, n)
	attributes = append(attributes,
		slog.String("protocol", req.Proto),
		slog.String("host", req.Host),
		slog.String("method", req.Method),
		slog.String("pattern", req.Pattern),
		slog.String("uri", req.RequestURI),
		slog.String("path", p),
		slog.String("referer", req.Referer()),
		slog.String("user_agent", req.UserAgent()),
		slog.Int("status", status),
		slog.String("content_length", req.Header.Get(wo.HeaderContentLength)),
		slog.Int64("response_size", res.Size),
	)

	if id != "" {
		attributes = append(attributes, slog.String("request-id", id))
	}

	if err != nil {
		attributes = append(attributes, slog.Any("error", err))
	}

	return attributes
}
