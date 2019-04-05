package store

import "fmt"

// NotFoundError is used when an attempt is made to access a
// non-existent produce code from the store.
type NotFoundError struct {
	Code string
}

// Error satisfies the error interface.
func (nfe NotFoundError) Error() string {
	return fmt.Sprintf("produce code '%s' was not found", nfe.Code)
}

// AlreadyExistsError is used when an attempt is made to add a
// produce code that already exists in the store.
type AlreadyExistsError struct {
	Code string
}

// Error satisfies the error interface.
func (aee AlreadyExistsError) Error() string {
	return fmt.Sprintf("produce code '%s' already exists", aee.Code)
}
