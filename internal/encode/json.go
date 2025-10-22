//go:build !goexperiment.jsonv2

package encode

import (
	"encoding/json"
	"io"
)

func MarshalJSON(out io.Writer, in any, indent string) error {
	enc := json.NewEncoder(out)

	if indent != "" {
		return enc.SetIndent("", indent)
	}

	return enc.Encode(in)
}

func UnmarshalJSON(in io.Reader, out any) error {
	return json.NewDecoder(in).Decode(out)
}
