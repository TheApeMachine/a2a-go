package errors

import (
	"fmt"
	"strings"
)

type Error struct {
	Errs []error
	Msgs []any
}

func NewError(errs ...any) error {
	err := &Error{}

	for _, msg := range errs {
		switch v := msg.(type) {
		case error:
			err.Errs = append(err.Errs, v)
		case string:
			err.Msgs = append(err.Msgs, v)
		}
	}

	return err
}

func (err *Error) Error() string {
	builder := &strings.Builder{}

	for _, err := range err.Errs {
		builder.WriteString(err.Error())
		builder.WriteString("\n")
	}

	for _, msg := range err.Msgs {
		builder.WriteString(fmt.Sprintf("%v\n", msg))
	}

	return builder.String()
}
