package wo

import (
	"net/http"
	"strings"
)

func SetHeaderIfMissing(res http.ResponseWriter, key string, value string) {
	if res.Header().Get(key) == "" {
		res.Header().Set(key, value)
	}
}

func ParseAcceptLanguageHeader(languageHeader string) []string {
	if languageHeader == "" {
		return make([]string, 0)
	}

	options := strings.Split(languageHeader, ",")
	l := len(options)
	languages := make([]string, l)

	for i := 0; i < l; i++ {
		locale := strings.SplitN(options[i], ";", 2)
		languages[i] = strings.Trim(locale[0], " ")
	}

	return languages
}

func ParseAcceptHeader(acceptHeader string) []string {
	if acceptHeader == "" {
		return make([]string, 0)
	}

	parts := strings.Split(acceptHeader, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if i := strings.IndexByte(part, ';'); i > 0 {
			part = part[:i]
		}
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func NegotiateFormat(accepted []string, offered ...string) string {
	if len(offered) == 0 {
		panic("negotiateFormat: you must provide at least one offer")
	}

	if len(accepted) == 0 {
		return offered[0]
	}

	for _, a := range accepted {
		for _, offer := range offered {
			// According to RFC 2616 and RFC 2396, non-ASCII characters are not allowed in headers,
			// therefore we can just iterate over the string without casting it into []rune
			i := 0
			for ; i < len(a) && i < len(offer); i++ {
				if a[i] == '*' || offer[i] == '*' {
					return offer
				}
				if a[i] != offer[i] {
					break
				}
			}
			if i == len(a) {
				return offer
			}
		}
	}
	return ""
}
