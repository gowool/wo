package middleware

import (
	"net/http"

	"github.com/gowool/wo"
)

type event interface {
	Next() error
	Request() *http.Request
	Response() *wo.Response
}
