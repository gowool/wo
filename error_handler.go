package wo

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/gowool/wo/internal/convert"
	"github.com/gowool/wo/internal/encode"
)

const errorTemplate = `<!DOCTYPE html>
<html dir="ltr" lang="en">
<head>
	<meta charset="utf-8" />
	<style type="text/css">
		h1 {
		  font-size: 15vmin;
		  margin-bottom: 0;
		}
		h2 {
		  font-size: 5vmin;
		  margin-top: 0;
		  margin-bottom: 40px;
		}
		
		body {
		  height: 100vh;
		  display: flex;
		  flex-direction: column;
		  background-color: white;
		  align-items: center;
		  justify-content: center;
		  overflow: hidden;
		}
	</style>
	<title>{{.status}} - {{.title}}</title>
</head>
<body>
	<h1>{{.title}}!</h1>
	<h2>Code {{.status}}</h2>
</body>
</html>`

var errorTpl = template.Must(template.New("error_template").Parse(errorTemplate))

type HTTPErrorHandler[T Resolver] func(T, error)

func ErrorHandler[T Resolver](render func(T, *HTTPError), mapper func(error) *HTTPError, logger *slog.Logger) HTTPErrorHandler[T] {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	if mapper == nil {
		mapper = AsHTTPError
	}

	return func(e T, err error) {
		req := e.Request()
		res := e.Response()

		if res.Written {
			logger.Warn("error handler: called after response written", "error", err)
			return
		}

		var redirect *RedirectError
		if errors.As(err, &redirect) {
			logger.Debug("error handler: redirect", "error", err)

			res.Header().Set(HeaderLocation, redirect.URL)
			res.WriteHeader(redirect.Status)
			return
		}

		httpErr := mapper(err)
		if httpErr == nil {
			httpErr = ErrInternalServerError.WithInternal(err)
		}

		defer func() {
			if !RequestLogged(e.Request().Context()) {
				logger.LogAttrs(
					context.Background(),
					slog.LevelError,
					"request failed",
					RequestLoggerAttrs[T](e, httpErr.Status, err)...,
				)
			}
		}()

		if req.Method == http.MethodHead {
			res.WriteHeader(httpErr.Status)
			return
		}

		httpErr.Debug = Debug(req.Context())

		if render != nil {
			render(e, httpErr)

			if e.Response().Written {
				return
			}

			req = e.Request()
			res = e.Response()
		}

		contentType := NegotiateFormat(
			ParseAcceptHeader(req.Header.Get(HeaderAccept)),
			MIMETextPlainCharsetUTF8,
			MIMETextHTMLCharsetUTF8,
			MIMEApplicationJSON,
		)

		res.Header().Set(HeaderContentType, contentType)
		res.WriteHeader(httpErr.Status)

		var err1 error
		switch contentType {
		case MIMEApplicationJSON:
			if err1 = encode.MarshalJSON(res, httpErr, indent(req)); err1 != nil {
				err1 = fmt.Errorf("write json: %w", err1)
			}
		case MIMETextHTMLCharsetUTF8:
			if err1 = errorTpl.Execute(res, httpErr.ToMap()); err1 != nil {
				err1 = fmt.Errorf("write html: %w", err1)
			}
		default:
			if _, err1 = res.Write(convert.StringToBytes(httpErr.title())); err1 != nil {
				err1 = fmt.Errorf("write text: %w", err1)
			}
		}

		if err1 != nil {
			logger.Error("error handler: write response", "error", err1)
		}
	}
}
