package security

import (
	"encoding/base64"
	"encoding/binary"
	"math/rand/v2"
)

var chaCha8 = newChaCha8()

func newChaCha8() *rand.ChaCha8 {
	var b [32]byte
	binary.LittleEndian.PutUint64(b[:], rand.Uint64())
	binary.LittleEndian.AppendUint64(b[:], rand.Uint64())
	binary.LittleEndian.AppendUint64(b[:], rand.Uint64())
	binary.LittleEndian.AppendUint64(b[:], rand.Uint64())
	return rand.NewChaCha8(b)
}

func Token() (string, error) {
	b := make([]byte, 32)
	_, err := chaCha8.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
