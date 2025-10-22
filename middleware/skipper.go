package middleware

import (
	"regexp"
	"strings"

	"github.com/gowool/wo/internal/arr"
)

var methodRe = regexp.MustCompile(`^(\S*)\s+(.*)$`)

type Skipper[E event] func(e E) bool

func ChainSkipper[E event](skippers ...Skipper[E]) Skipper[E] {
	return func(e E) bool {
		for _, skipper := range skippers {
			if skipper(e) {
				return true
			}
		}
		return false
	}
}

func PrefixPathSkipper[E event](prefixes ...string) Skipper[E] {
	prefixes = arr.Map(prefixes, strings.ToLower)
	return func(e E) bool {
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

func SuffixPathSkipper[E event](suffixes ...string) Skipper[E] {
	suffixes = arr.Map(suffixes, strings.ToLower)
	return func(e E) bool {
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

func EqualPathSkipper[E event](paths ...string) Skipper[E] {
	return func(e E) bool {
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
