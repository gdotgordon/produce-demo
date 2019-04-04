package store

import "fmt"

// NotFoundError is used when an attempt is made to access a
// non-existent produce code from the store.
type NotFoundError struct {
	code string
}

// Error satisfies the error interface.
func (nfe NotFoundError) Error() string {
	return fmt.Sprintf("produce code '%s' was not found", nfe.code)
}

// AlreadyExistsError is used when an attempt is made to add a
// produce code that already exists in the store.
type AlreadyExistsError struct {
	code string
}

// Error satisfies the error interface.
func (aee AlreadyExistsError) Error() string {
	return fmt.Sprintf("produce code '%s' already exists", aee.code)
}
