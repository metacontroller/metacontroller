package hooks

import (
	"fmt"
)

type TooManyRequestError struct {
	AfterSecond int
}

func (e *TooManyRequestError) Error() string {
	return fmt.Sprintf("Too many request, it will be resync after: %d", e.AfterSecond)
}
