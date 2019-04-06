// Package service implments the functionality of the Produce Service.  It
// is the intermediary between the api package, which handles HTTP specifcs
// JSON marshaling, and the store pacakge, whicn is the data store.  In fact,
// because the store layer implments the storage and retrieval of produce
// items, the current layer is mostly concerned with the mechanics of
// interacting with the store, such as creating the goroutines and managing
// batched requests for add.
package service

import (
	"context"
	"fmt"

	"github.com/gdotgordon/produce-demo/store"
	"github.com/gdotgordon/produce-demo/types"
)

// InternalError is used when something unexpectedly failed in the code
// while invloking the service
type InternalError struct {
	Message string
}

// Error satisfies the error interface.
func (ie InternalError) Error() string {
	return fmt.Sprintf("an unexpected error occurred: %s", ie.Message)
}

// FormatError is used when an item doesn't conform to the expcted format,
// particularly the syntax for a field value.
// while invloking the service
type FormatError struct {
	Message string
}

// Error satisfies the error interface.
func (fe FormatError) Error() string {
	return fmt.Sprintf("invalid item format: %s", fe.Message)
}

// AddResult is used to communicate back the results of each of the
// adds  to the api layer.
type AddResult struct {
	Code string
	Desc string
	Err  error
}

// Service is the interface for produce item management.  The use
// of an interface allows us to conveniently mock the service in tests.
type Service interface {
	// Add adds multiple produce items to the store or returns the status
	// of each add, or a general error if a system error prevented even
	// attempting the add.
	Add(context.Context, []types.Produce) ([]AddResult, error)

	// Delete deletes single produce item from the store or returns an error
	// if it fails.
	Delete(context.Context, string) error

	// ListAll fetches all produce items from the store or returns an error
	// if it fails.
	ListAll(context.Context) ([]types.Produce, error)

	// Clear is a convenience API to reset the database, useful for testing.
	Clear(context.Context) error
}

// ProduceService is the concrete instance of the service described above.
type ProduceService struct {
	store store.ProduceStore
}

// New creates and returns a Produce Service instance
func New(store store.ProduceStore) ProduceService {
	return ProduceService{store: store}
}

// Add adds multiple produce items to the store or returns the status
// of each add, or a general error if a system error prevented even
// attempting the add.
func (ps ProduceService) Add(ctx context.Context,
	items []types.Produce) ([]AddResult, error) {
	// Each goroutine will pass it's index into the array
	// and a possible error back through the channel.
	type addResp struct {
		ndx int
		err error
	}

	ch := make(chan addResp)
	defer close(ch)

	// Run the delete in a goroutine as requested by the spec.
	var wch chan<- addResp = ch
	res := make([]AddResult, len(items))

	for i := 0; i < len(items); i++ {
		// Need the proper loop index bound to the goroutine
		i := i
		go func() {
			// Enforce the semntics and convert the produce items before
			// sending them to storage
			msg := types.ValidateAndConvertProduce(&items[i])
			if msg != "" {
				wch <- addResp{ndx: i, err: FormatError{Message: msg}}
			}
			addErr := ps.store.Add(ctx, items[i])
			wch <- addResp{ndx: i, err: addErr}
		}()
	}

	// Process each retrun from add, and store the error result
	// in the appropriate slot in the return item
	for i := 0; i < len(items); i++ {
		aresp, ok := <-ch
		if !ok {
			// Channel was mysteriously closed!
			return nil, InternalError{Message: "Unexpceted channel close"}
		}
		res[aresp.ndx].Code = items[i].Code
		res[aresp.ndx].Err = aresp.err
	}
	return res, nil
}

// Delete deletes single produce item (specified by the code) from the store,
// or returns an error if it fails.
func (ps ProduceService) Delete(ctx context.Context, code string) error {
	ch := make(chan error)
	defer close(ch)

	// Run the delete in a goroutine as requested by the spec.
	var wch chan<- error = ch
	go func() {
		delErr := ps.store.Delete(ctx, code)
		wch <- delErr
	}()

	// And wait for the return in the channel, which is just an error.
	err, ok := <-ch
	if !ok {
		// Channel was mysteriously closed!
		return InternalError{Message: "Unexpceted channel close"}
	}
	return err
}

// ListAll fetches all produce items from the store or returns an error
// if it fails.
func (ps ProduceService) ListAll(ctx context.Context) ([]types.Produce, error) {
	type listResp struct {
		items []types.Produce
		err   error
	}
	ch := make(chan listResp)
	defer close(ch)

	// Run the delete in a goroutine as requested by the spec.
	var wch chan<- listResp = ch
	go func() {
		items, err := ps.store.ListAll(ctx)
		wch <- listResp{items: items, err: err}
	}()

	// And wait for the return in the channel.
	lr, ok := <-ch
	if !ok {
		// Channel was mysteriously closed!
		return nil, InternalError{Message: "Unexpceted channel close"}
	}
	return lr.items, lr.err
}

// Clear is a convenience API to reset the database, useful for testing.
func (ps ProduceService) Clear(ctx context.Context) error {
	return ps.store.Clear(ctx)
}
