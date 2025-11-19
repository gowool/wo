//go:build !goexperiment.jsonv2

package wo

import "encoding/json"

func (he *HTTPError) MarshalJSON() ([]byte, error) {
	return json.Marshal(errData{
		Status:   he.Status,
		Title:    he.title(),
		Detail:   he.detail(),
		Internal: he.internal(),
	})
}
