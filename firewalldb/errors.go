package firewalldb

import "errors"

var (
	ErrDuplicateRealValue   = errors.New("an entry with the given real value already exists")
	ErrDuplicatePseudoValue = errors.New("an entry with the given pseudo value already exists")
)
