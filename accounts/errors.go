package accounts

import "fmt"

var (
	ErrLabelAlreadyExists = fmt.Errorf(
		"account label uniqueness constraint violation",
	)
)
