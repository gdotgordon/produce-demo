package service

import (
	"context"
	"sort"
	"testing"

	"github.com/gdotgordon/produce-demo/store"
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

	dfltProduceBadCode = types.Produce{
		Code:      "A12T-4GH7-QP",
		Name:      "Lettuce",
		UnitPrice: (346),
	}

	secondProduceBadName = types.Produce{
		Code:      "YRT6-72AS-K736-L4AR",
		Name:      "Green-Pepper",
		UnitPrice: types.USD(79),
	}

	secondProduceLower = types.Produce{
		Code:      "yrt6-72as-k736-l4ar",
		Name:      "green pepper",
		UnitPrice: types.USD(79),
	}
	secondProduceBadNameLower = types.Produce{
		Code:      "yrt6-72as-k736-l4ar",
		Name:      "green-pepper",
		UnitPrice: types.USD(79),
	}
)

func TestAdd(t *testing.T) {
	for i, v := range []struct {
		req    []types.Produce
		expRes []AddResult
		expErr error
	}{
		{
			req:    []types.Produce{},
			expRes: []AddResult{},
		},
		{
			req:    []types.Produce{dfltProduce},
			expRes: []AddResult{AddResult{Code: dfltProduce.Code}},
		},
		{
			req: []types.Produce{dfltProduce, secondProduce},
			expRes: []AddResult{AddResult{Code: secondProduce.Code},
				AddResult{Code: dfltProduce.Code}},
		},
		{
			req: []types.Produce{dfltProduce, dfltProduce},
			expRes: []AddResult{AddResult{Code: dfltProduce.Code},
				AddResult{Code: dfltProduce.Code,
					Err: store.AlreadyExistsError{Code: dfltProduce.Code}}},
		},
		{
			req: []types.Produce{dfltProduce, secondProduceBadName},
			expRes: []AddResult{AddResult{Code: dfltProduce.Code},
				AddResult{Code: secondProduceBadName.Code,
					Err: FormatError{Message: "invalid name: 'Green-Pepper'"}}},
		},
		{
			req: []types.Produce{dfltProduceBadCode},
			expRes: []AddResult{AddResult{
				Code: dfltProduceBadCode.Code,
				Err:  FormatError{Message: "invalid code: 'A12T-4GH7-QP'"},
			}},
		},
		{
			req:    []types.Produce{secondProduceLower},
			expRes: []AddResult{AddResult{Code: secondProduce.Code}},
		},
		{
			req: []types.Produce{secondProduceBadNameLower},
			expRes: []AddResult{AddResult{Code: secondProduce.Code,
				Err: FormatError{Message: "invalid name: 'green-pepper'"}}},
		},
		{
			req: []types.Produce{dfltProduce, secondProduce, secondProduceLower, secondProduceBadName},
			expRes: []AddResult{
				AddResult{Code: dfltProduce.Code},
				AddResult{Code: secondProduce.Code},
				AddResult{Code: secondProduce.Code,
					Err: store.AlreadyExistsError{Code: secondProduce.Code}},
				AddResult{Code: secondProduce.Code,
					Err: FormatError{Message: "invalid name: 'Green-Pepper'"}}},
		},
	} {
		d := DummyStore{store: store.New()}
		service := New(d)
		res, err := service.Add(context.Background(), v.req)

		if v.expErr != err {
			t.Fatalf("expected errors don't agree: %v, %v", v.expErr, err)
		}
		if len(v.expRes) != len(res) {
			t.Fatalf("expcted %d responses, got %d", len(v.expRes), len(res))
		}

		if len(v.expRes) > 0 {
			sort.Sort(resSorter{res: v.expRes})
			sort.Sort(resSorter{res: res})
			for j, w := range v.expRes {
				if res[j] != w {
					t.Fatalf("(%d) sorted results differ at %d, %+v, %+v", i, j, res[j], w)
				}
			}
		}
	}
}

func TestDelete(t *testing.T) {
	for i, v := range []struct {
		code   string
		expErr error
		add    *types.Produce
	}{
		{
			code:   "YRT6-72AS-K736-L4AR",
			expErr: store.NotFoundError{Code: "YRT6-72AS-K736-L4AR"},
		},
		{
			code:   "YRT6-72AS-K736-L4AR",
			expErr: nil,
			add:    &secondProduce,
		},
		{
			code:   "badcode",
			expErr: FormatError{"badcode"},
		},
	} {
		d := DummyStore{store: store.New()}
		service := New(d)
		if v.add != nil {
			d.Add(context.Background(), *v.add)
		}
		err := service.Delete(context.Background(), v.code)
		if v.expErr != err {
			t.Fatalf("(%d) expected error: %v, got %v", i, v.expErr, err)
		}
	}
}

func TestList(t *testing.T) {
	d := DummyStore{store: store.New()}
	service := New(d)
	err := d.Add(context.Background(), dfltProduce)
	if err != nil {
		t.Fatalf("unexpected error adding item: %v", err)
	}
	err = d.Add(context.Background(), secondProduce)
	if err != nil {
		t.Fatalf("unexpected error adding item: %v", err)
	}

	items, err := service.ListAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected list length: %d", len(items))
	}
	if !((items[0] == dfltProduce && items[1] == secondProduce) ||
		(items[1] == dfltProduce && items[0] == secondProduce)) {
		t.Fatalf("unexpected list lcontents: %v", items)
	}
}

type DummyStore struct {
	store store.ProduceStore
}

func (d DummyStore) Add(ctx context.Context, item types.Produce) error {
	return d.store.Add(ctx, item)
}

// Delete deletes single produce item from the store or returns an error
// if it fails.
func (d DummyStore) Delete(ctx context.Context, code string) error {
	return d.store.Delete(ctx, code)
}

// ListAll fetches all produce items from the store or returns an error
// if it fails.
func (d DummyStore) ListAll(ctx context.Context) ([]types.Produce, error) {
	return d.store.ListAll(ctx)
}

// Clear is a convenience API to reset the database, useful for testing.
func (d DummyStore) Clear(ctx context.Context) error {
	return d.store.Clear(ctx)
}
