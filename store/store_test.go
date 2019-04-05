package store

import (
	"context"
	"testing"

	"github.com/gdotgordon/produce-demo/types"
)

var (
	dfltProduce = types.Produce{
		Code:      "A12T-4GH7-QPL9-3N4M",
		Name:      "Lettuce",
		UnitPrice: types.USD(346),
	}

	secondProduce = types.Produce{
		Code:      "YRT6-72AS-K736-L4AR",
		Name:      "Green Pepper",
		UnitPrice: types.USD(79),
	}
)

func TestAdd(t *testing.T) {
	var store = New()
	err := store.Add(context.Background(), dfltProduce)
	if err != nil {
		t.Fatalf("error adding produce: %v", err)
	}
	var lps = store.(*LockingProduceStore)
	if len(lps.store) != 1 {
		t.Fatalf("unexpected store count: %d", len(lps.store))
	}

	prod := lps.store[dfltProduce.Code]
	if prod == nil || *prod != dfltProduce {
		t.Fatalf("expected produce not found")
	}

	// Try to add the same one again.
	err = store.Add(context.Background(), dfltProduce)
	if err == nil {
		t.Fatalf("did not get expected error")
	}
	_, ok := err.(AlreadyExistsError)
	if !ok {
		t.Fatalf("did not get expected error type, got %T", err)
	}

	// Add a second one.
	err = store.Add(context.Background(), secondProduce)
	if err != nil {
		t.Fatalf("error adding produce: %v", err)
	}
	if len(lps.store) != 2 {
		t.Fatalf("unexpected store count: %d", len(lps.store))
	}
	prod = lps.store[secondProduce.Code]
	if prod == nil || *prod != secondProduce {
		t.Fatalf("expected produce not found")
	}
}

func TestDelete(t *testing.T) {
	var store = New()

	// First test for error when store is empty
	err := store.Delete(context.Background(), dfltProduce.Code)
	if err == nil {
		t.Fatalf("did not get expected error")
	}
	_, ok := err.(NotFoundError)
	if !ok {
		t.Fatalf("did not get expected error type, got %T", err)
	}

	var lps = store.(*LockingProduceStore)
	lps.store[dfltProduce.Code] = &dfltProduce
	err = store.Delete(context.Background(), dfltProduce.Code)
	if err != nil {
		t.Fatalf("error deleting produce: %v", err)
	}
	if len(lps.store) != 0 {
		t.Fatalf("unexpected store count: %d", len(lps.store))
	}
}

func TestListAll(t *testing.T) {
	var store = New()

	// Test empty list
	res, err := store.ListAll(context.Background())
	if err != nil {
		t.Fatalf("error adding produce: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected empty list")
	}

	var lps = store.(*LockingProduceStore)
	lps.store[dfltProduce.Code] = &dfltProduce
	lps.store[secondProduce.Code] = &secondProduce
	res, err = store.ListAll(context.Background())
	if err != nil {
		t.Fatalf("error adding produce: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 items, got %d", len(res))
	}
	if !((res[0] == dfltProduce && res[1] == secondProduce) ||
		(res[1] == dfltProduce && res[0] == secondProduce)) {
		t.Fatalf("did not receive expected item list")
	}
}

func TestClear(t *testing.T) {
	var store = New()

	// Test clear of empty store
	err := store.Clear(context.Background())
	if err != nil {
		t.Fatalf("error clearing store: %v", err)
	}

	var lps = store.(*LockingProduceStore)
	if len(lps.store) != 0 {
		t.Fatalf("store is not empty after clear")
	}

	// Test clear of non-empty store
	lps.store[dfltProduce.Code] = &dfltProduce
	err = store.Clear(context.Background())
	if err != nil {
		t.Fatalf("error clearing store: %v", err)
	}
	if len(lps.store) != 0 {
		t.Fatalf("store is not empty after clear")
	}
}
