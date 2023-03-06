package hooks

import (
	"errors"
	"fmt"
	"reflect"
)

type TooManyRequestError struct {
	AfterSecond int
}

func (e *TooManyRequestError) Error() string {
	return fmt.Sprintf("Too many request, it will be resync after: %d", e.AfterSecond)
}

func UnwrapTo(e error, targetType any) interface{} {
	for e != nil {
		if reflect.TypeOf(e).String() == reflect.TypeOf(targetType).String() {
			return e
		}
		e = errors.Unwrap(e)
	}
	return nil
}
