package wo

import "log/slog"

func RequestLoggerAttrs[T Resolver](e T, status int, err error) []slog.Attr {
	req := e.Request()
	res := e.Response()

	p := req.URL.Path
	if p == "" {
		p = "/"
	}

	id := req.Header.Get(HeaderXRequestID)
	if id == "" {
		id = res.Header().Get(HeaderXRequestID)
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
		slog.String("remote_addr", req.RemoteAddr),
		slog.String("host", req.Host),
		slog.String("method", req.Method),
		slog.String("pattern", req.Pattern),
		slog.String("uri", req.RequestURI),
		slog.String("path", p),
		slog.String("referer", req.Referer()),
		slog.String("user_agent", req.UserAgent()),
		slog.Int("status", status),
		slog.String("content_length", req.Header.Get(HeaderContentLength)),
		slog.Int64("response_size", MustUnwrapResponse(res).Size),
	)

	if id != "" {
		attributes = append(attributes, slog.String("request_id", id))
	}

	if err != nil {
		attributes = append(attributes, slog.Any("error", err))
	}

	return attributes
}
