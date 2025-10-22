package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"

	"github.com/gowool/wo"
)

type RecoverConfig struct {
	// Size of the stack to be printed.
	// Optional. Default value 2KB.
	StackSize int `json:"stackSize,omitempty" yaml:"stackSize,omitempty"`
}

func (c *RecoverConfig) SetDefaults() {
	if c.StackSize == 0 {
		c.StackSize = 2 << 10 // 2KB
	}
}

func Recover[E event](cfg RecoverConfig) func(E) error {
	cfg.SetDefaults()

	return func(e E) (err error) {
		defer func() {
			if r := recover(); r != nil {
				recoverErr, ok := r.(error)
				if !ok {
					recoverErr = fmt.Errorf("%v", r)
				} else if errors.Is(recoverErr, http.ErrAbortHandler) {
					// don't recover ErrAbortHandler so the response to the client can be aborted
					panic(recoverErr)
				}

				stack := make([]byte, cfg.StackSize)
				length := runtime.Stack(stack, true)
				internal := fmt.Errorf("[PANIC RECOVER] %w %s", recoverErr, stack[:length])
				err = wo.ErrInternalServerError.WithInternal(internal)
			}
		}()

		err = e.Next()

		return err
	}
}
