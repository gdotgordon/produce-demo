package store

import (
	"context"
	"testing"

	"github.com/gdotgordon/produce-demo/types"
)

var (
	dfltProduce = types.Produce{
		ProduceCode: "A12T-4GH7-QPL9-3N4M",
		Name:        "Lettuce",
		UnitPrice:   types.USD(346),
	}

	secondProduce = types.Produce{
		ProduceCode: "YRT6-72AS-K736-L4AR",
		Name:        "Green Pepper",
		UnitPrice:   types.USD(79),
	}
)

func TestAdd(t *testing.T) {
	var store ProduceStore = NewLockingProduceStore()
	err := store.Add(context.Background(), dfltProduce)
	if err != nil {
		t.Fatalf("error adding produce: %v", err)
	}
	var lps = store.(*LockingProduceStore)
	if len(lps.store) != 1 {
		t.Fatalf("unexpected store count: %d", len(lps.store))
	}

	prod := lps.store[dfltProduce.ProduceCode]
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
	prod = lps.store[secondProduce.ProduceCode]
	if prod == nil || *prod != secondProduce {
		t.Fatalf("expected produce not found")
	}
}

func TestDelete(t *testing.T) {
	var store ProduceStore = NewLockingProduceStore()

	// First test for error when store is empty
	err := store.Delete(context.Background(), dfltProduce.ProduceCode)
	if err == nil {
		t.Fatalf("did not get expected error")
	}
	_, ok := err.(NotFoundError)
	if !ok {
		t.Fatalf("did not get expected error type, got %T", err)
	}

	var lps = store.(*LockingProduceStore)
	lps.store[dfltProduce.ProduceCode] = &dfltProduce
	err = store.Delete(context.Background(), dfltProduce.ProduceCode)
	if err != nil {
		t.Fatalf("error deleting produce: %v", err)
	}
	if len(lps.store) != 0 {
		t.Fatalf("unexpected store count: %d", len(lps.store))
	}
}

func TestListAll(t *testing.T) {
	var store ProduceStore = NewLockingProduceStore()

	// Test empty list
	res, err := store.ListAll(context.Background())
	if err != nil {
		t.Fatalf("error adding produce: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected empty list")
	}

	var lps = store.(*LockingProduceStore)
	lps.store[dfltProduce.ProduceCode] = &dfltProduce
	lps.store[secondProduce.ProduceCode] = &secondProduce
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
	var store ProduceStore = NewLockingProduceStore()

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
	lps.store[dfltProduce.ProduceCode] = &dfltProduce
	err = store.Clear(context.Background())
	if err != nil {
		t.Fatalf("error clearing store: %v", err)
	}
	if len(lps.store) != 0 {
		t.Fatalf("store is not empty after clear")
	}
}
