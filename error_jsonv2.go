//go:build goexperiment.jsonv2

package wo

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

func (he *HTTPError) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, errData{
		Status:   he.Status,
		Title:    he.title(),
		Detail:   he.detail(),
		Internal: he.internal(),
	})
}
