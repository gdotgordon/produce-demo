// Package store defines an interface and implementation for performing
// the produce storage operations: add item, delete item, list all items.
// Note, even though the external API allows for multiple adds in a single
// request, they are processed individually as per the spec, so the store API
// only needsto handle single adds.
package store

import (
	"context"
	"sync"

	"github.com/gdotgordon/produce-demo/types"
)

// ProduceStore is the interface for produce item storage and retrieval.
// The use of an interface allows us to conveniently mock the storage in tests.
type ProduceStore interface {

	// Add adds a single produce item to the store or returns an error
	// if it fails.
	Add(context.Context, types.Produce) error

	// Delete deletes single produce item from the store or returns an error
	// if it fails.
	Delete(context.Context, string) error

	// ListAll fetches all produce items from the store or returns an error
	// if it fails.
	ListAll(context.Context) ([]types.Produce, error)

	// Clear is a convenience API to reset the database, useful for testing.
	Clear(context.Context) error
}

// LockingProduceStore is the production implementaiton of the store.
// Note, because it uses a sync.RWMutex, it should only be assigned
// to a ProduceStore interface as a pointer.  The ProduceStore methods
// all require pointers to the item because the mutex cannot be copied.
type LockingProduceStore struct {

	// The store is implemented as a hash map of Produce Code to Produce.
	// We actaully use a pointer to the produce item, because if we ever
	// wanted to mutate the produce, without a pointer, we'd need to
	// copy in a whole new Produce.
	store map[string]*types.Produce

	// Multiple-reader, single writer seems reasonable given the API and
	// the use of the hash map.
	lock sync.RWMutex
}

// New creates an initialized instance of a concrete produce store.  We hide
// the implementation under an interface, so we can easily swap in a new one.
func New() ProduceStore {
	ps := LockingProduceStore{store: make(map[string]*types.Produce)}
	return &ps
}

// Add adds a single produce item to the store or returns an error
// if it fails.
func (lps *LockingProduceStore) Add(ctx context.Context,
	prod types.Produce) error {
	lps.lock.Lock()
	defer lps.lock.Unlock()

	_, ok := lps.store[prod.Code]
	if ok {
		return AlreadyExistsError{Code: prod.Code}
	}
	lps.store[prod.Code] = &prod
	return nil
}

// Delete deletes single produce item from the store or returns an error
// if it fails.
func (lps *LockingProduceStore) Delete(ctx context.Context,
	code string) error {
	lps.lock.Lock()
	defer lps.lock.Unlock()

	_, ok := lps.store[code]
	if !ok {
		return NotFoundError{Code: code}
	}

	delete(lps.store, code)
	return nil
}

// ListAll fetches all produce items from the store or returns an error
// if it fails.
func (lps *LockingProduceStore) ListAll(ctx context.Context) (
	[]types.Produce, error) {
	lps.lock.RLock()
	defer lps.lock.RUnlock()

	ret := make([]types.Produce, 0, len(lps.store))
	for _, v := range lps.store {
		ret = append(ret, *v)
	}
	return ret, nil
}

// Clear is a convenience API to reset the database, useful for testing.
func (lps *LockingProduceStore) Clear(context.Context) error {
	lps.store = make(map[string]*types.Produce)
	return nil
}
