package middleware

import (
	"regexp"
	"strings"

	"github.com/gowool/wo"
	"github.com/gowool/wo/internal/arr"
)

var methodRe = regexp.MustCompile(`^(\S*)\s+(.*)$`)

type Skipper[T wo.Resolver] func(e T) bool

func ChainSkipper[T wo.Resolver](skippers ...Skipper[T]) Skipper[T] {
	return func(e T) bool {
		for _, skipper := range skippers {
			if skipper(e) {
				return true
			}
		}
		return false
	}
}

func PrefixPathSkipper[T wo.Resolver](prefixes ...string) Skipper[T] {
	prefixes = arr.Map(prefixes, strings.ToLower)
	return func(e T) bool {
		p := strings.ToLower(e.Request().URL.Path)
		m := strings.ToLower(e.Request().Method)
		for _, prefix := range prefixes {
			if prefix, ok := CheckMethod(m, prefix); ok && strings.HasPrefix(p, prefix) {
				return true
			}
		}
		return false
	}
}

func SuffixPathSkipper[T wo.Resolver](suffixes ...string) Skipper[T] {
	suffixes = arr.Map(suffixes, strings.ToLower)
	return func(e T) bool {
		p := strings.ToLower(e.Request().URL.Path)
		m := strings.ToLower(e.Request().Method)
		for _, suffix := range suffixes {
			if suffix, ok := CheckMethod(m, suffix); ok && strings.HasSuffix(p, suffix) {
				return true
			}
		}
		return false
	}
}

func EqualPathSkipper[T wo.Resolver](paths ...string) Skipper[T] {
	return func(e T) bool {
		for _, path := range paths {
			if path, ok := CheckMethod(e.Request().Method, path); ok && strings.EqualFold(e.Request().URL.Path, path) {
				return true
			}
		}
		return false
	}
}

func CheckMethod(method, skip string) (string, bool) {
	if matches := methodRe.FindStringSubmatch(skip); len(matches) > 2 {
		if matches[1] == method {
			return matches[2], true
		}
		return "", false
	}
	return skip, true
}
