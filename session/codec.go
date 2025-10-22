package session

import (
	"bytes"
	"encoding/gob"
	"time"
)

type Codec interface {
	Encode(deadline time.Time, values map[string]any) ([]byte, error)
	Decode([]byte) (deadline time.Time, values map[string]any, err error)
}

type GobCodec struct{}

func NewGobCodec() GobCodec {
	return GobCodec{}
}

func (GobCodec) Encode(deadline time.Time, values map[string]any) ([]byte, error) {
	aux := &struct {
		Deadline time.Time
		Values   map[string]any
	}{
		Deadline: deadline,
		Values:   values,
	}

	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(&aux); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (GobCodec) Decode(b []byte) (time.Time, map[string]any, error) {
	aux := &struct {
		Deadline time.Time
		Values   map[string]any
	}{}

	r := bytes.NewReader(b)
	if err := gob.NewDecoder(r).Decode(&aux); err != nil {
		return time.Time{}, nil, err
	}
	return aux.Deadline, aux.Values, nil
}
