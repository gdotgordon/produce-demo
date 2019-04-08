package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gdotgordon/produce-demo/service"
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
		Code:      "DRT6-72AS-K736-L4AR",
		Name:      "Green-Pepper",
		UnitPrice: types.USD(79),
	}
)

func TestStatusEndpoint(t *testing.T) {
	api := apiImpl{service: DummyService{}}
	req, err := http.NewRequest(http.MethodGet, statusURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Call the handler for status
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(wrapContext(context.Background(), api.getStatus))
	handler.ServeHTTP(rr, req)

	// Verify the code and expected body
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %d, expected %d",
			rr.Code, http.StatusOK)
	}
	expected := "{\n" + `  "status": "produce service is up and running"` + "\n}"
	body := rr.Body.String()
	if body != expected {
		t.Errorf("unexpected body: %s, expected %s", body, expected)
	}
}

func TestAddEndpoint(t *testing.T) {
	for i, v := range []struct {
		url       string
		req       types.ProduceAddRequest
		servErr   error
		existing  []types.Produce
		expStatus int
		expRes    []types.ProduceAddItemResponse
	}{
		{
			url:       produceURL + "/hello",
			expStatus: http.StatusBadRequest,
		},
		{
			url:       produceURL,
			req:       types.ProduceAddRequest{Items: []types.Produce{}},
			expStatus: http.StatusBadRequest,
		},
		{
			url:       produceURL,
			req:       types.ProduceAddRequest{Items: []types.Produce{dfltProduce}},
			expStatus: http.StatusCreated,
		},
		{
			url:       produceURL,
			existing:  []types.Produce{dfltProduce},
			req:       types.ProduceAddRequest{Items: []types.Produce{dfltProduce}},
			expStatus: http.StatusConflict,
		},
		{
			url:       produceURL,
			req:       types.ProduceAddRequest{Items: []types.Produce{dfltProduce, secondProduce}},
			expStatus: http.StatusCreated,
		},
		{
			url:       produceURL,
			req:       types.ProduceAddRequest{Items: []types.Produce{dfltProduce, dfltProduce}},
			expStatus: http.StatusOK,
			expRes: []types.ProduceAddItemResponse{
				types.ProduceAddItemResponse{Code: "A12T-4GH7-QPL9-3N4M", StatusCode: 201},
				types.ProduceAddItemResponse{Code: "A12T-4GH7-QPL9-3N4M",
					StatusCode: http.StatusConflict,
					Error:      "produce code 'Dup' already exists",
				},
			},
		},
		{
			url:       produceURL,
			req:       types.ProduceAddRequest{Items: []types.Produce{dfltProduce, secondProduceBadName}},
			expStatus: http.StatusOK,
			expRes: []types.ProduceAddItemResponse{
				types.ProduceAddItemResponse{Code: "A12T-4GH7-QPL9-3N4M", StatusCode: 201},
				types.ProduceAddItemResponse{Code: "DRT6-72AS-K736-L4AR",
					StatusCode: http.StatusBadRequest,
					Error:      "invalid item format: invalid name: 'Green-Pepper'",
				},
			},
		},
		{
			url:       produceURL,
			req:       types.ProduceAddRequest{Items: []types.Produce{dfltProduceBadCode}},
			expStatus: http.StatusBadRequest,
		},
		{
			url:       produceURL,
			req:       types.ProduceAddRequest{Items: []types.Produce{dfltProduce}},
			servErr:   errors.New("hiya"),
			expStatus: http.StatusInternalServerError,
		},
	} {
		d := DummyService{}
		if v.servErr != nil {
			d.err = v.servErr
		}
		if v.existing != nil {
			d.existing = v.existing
		}
		api := apiImpl{service: d}
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(api.handleProduce)

		// Setup the incoming payload
		var rdr io.Reader
		if v.req.Items != nil {
			b, err := json.Marshal(v.req)
			if err != nil {
				t.Fatal(err)
			}
			rdr = bytes.NewReader(b)
		}

		req, err := http.NewRequest(http.MethodPost, v.url, rdr)
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)
		if status := rr.Code; status != v.expStatus {
			t.Fatalf("(%d) handler returned wrong status code: got %d, expected %d",
				i, rr.Code, v.expStatus)
		}

		if len(v.expRes) > 0 {
			var par types.ProduceAddResponse
			err = json.Unmarshal(rr.Body.Bytes(), &par)
			if err != nil {
				t.Fatal(err)
			}
			if len(v.expRes) != len(par.Items) {
				t.Fatalf("mismatched add response count: %d, %d", len(v.expRes),
					len(par.Items))
			}
			for i, p := range par.Items {
				if v.expRes[i] != p {
					t.Fatalf("(%d) unexpected return item: %+v", i, p)
				}
			}
		}
	}
}

func TestDeleteEndpoint(t *testing.T) {
	for i, v := range []struct {
		url       string
		servErr   error
		existing  []types.Produce
		expStatus int
		expBody   string
	}{
		{
			url:       produceURL,
			expStatus: http.StatusBadRequest,
		},
		{
			url:       produceURL + "/YRT6-72AS-K736-L4AR",
			servErr:   store.NotFoundError{Code: "YRT6-72AS-K736-L4AR"},
			expStatus: http.StatusNotFound,
		},
		{
			url:       produceURL + "/YRT6-72AS-K736-L4AR",
			existing:  []types.Produce{types.Produce{Code: "YRT6-72AS-K736-L4AR"}},
			expStatus: http.StatusNoContent,
		},
	} {
		d := DummyService{}
		if v.servErr != nil {
			d.err = v.servErr
		}
		api := apiImpl{service: d}
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(api.handleDelete)

		// Bad request: we need the code in the url
		req, err := http.NewRequest(http.MethodDelete, v.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)
		if status := rr.Code; status != v.expStatus {
			t.Fatalf("(%d) handler returned wrong status code: got %d, expected %d",
				i, rr.Code, v.expStatus)
		}
		if v.expBody != "" {
			b, _ := ioutil.ReadAll(rr.Body)
			if v.expBody != string(b) {
				t.Errorf("unexpected body: %s, expected %s", string(b), v.expBody)
			}
		}
	}
}

func TestListEndpoint(t *testing.T) {
	for i, v := range []struct {
		url       string
		servErr   error
		existing  []types.Produce
		expStatus int
		expBody   string
		expRes    []types.Produce
	}{
		{
			url:       produceURL,
			servErr:   service.InternalError{Message: "Unexpceted channel close"},
			expStatus: http.StatusInternalServerError,
		},
		{
			url:       produceURL,
			existing:  []types.Produce{dfltProduce, secondProduce},
			expStatus: http.StatusOK,
			expRes:    []types.Produce{dfltProduce, secondProduce},
		},
		{
			url:       produceURL + "/fred",
			expStatus: http.StatusBadRequest,
		},
	} {
		d := DummyService{}
		if v.servErr != nil {
			d.err = v.servErr
		}
		if len(v.existing) > 0 {
			d.existing = v.existing
		}
		api := apiImpl{service: d}
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(api.handleProduce)

		// Bad request: we need the code in the url
		req, err := http.NewRequest(http.MethodGet, v.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)
		if status := rr.Code; status != v.expStatus {
			t.Fatalf("(%d) handler returned wrong status code: got %d, expected %d",
				i, rr.Code, v.expStatus)
		}
		if v.expBody != "" {
			b, _ := ioutil.ReadAll(rr.Body)
			if v.expBody != string(b) {
				t.Errorf("unexpected body: %s, expected %s", string(b), v.expBody)
			}
		}

		// Check that the list returned is what we expected.  Due to using
		// a hash map, the order is unpredictable, so we need to check that
		// the lengths are the same and all items are accounted for.
		if len(v.expRes) > 0 {
			b, _ := ioutil.ReadAll(rr.Body)
			var ap types.ProduceListResponse
			err := json.Unmarshal(b, &ap)
			if err != nil {
				t.Fatal(err)
			}

			if len(v.expRes) != len(ap.Items) {
				t.Errorf("Did not read expected number of list items")
			}

			cnt := 0
			for _, v := range v.expRes {
				for _, w := range ap.Items {
					if v == w {
						cnt++
						break
					}
				}
			}
			if cnt != len(v.expRes) {
				t.Errorf("did not match expected list results")
			}
		}
	}
}

func TestInvalidMethod(t *testing.T) {
	api := apiImpl{service: DummyService{}}
	req, err := http.NewRequest(http.MethodPut, produceURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Call the handler for status
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(wrapContext(context.Background(), api.handleProduce))
	handler.ServeHTTP(rr, req)

	// Verify the code and expected body
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %d, expected %d",
			rr.Code, http.StatusNotFound)
	}
}

func TestInit(t *testing.T) {
	err := Init(context.Background(), http.NewServeMux(), DummyService{})
	if err != nil {
		t.Fatalf("API init error: %v", err)
	}
}

type DummyService struct {
	err      error
	existing []types.Produce
}

func (d DummyService) Add(ctx context.Context, items []types.Produce) ([]service.AddResult, error) {
	if d.err != nil {
		return nil, d.err
	}
	res := make([]service.AddResult, len(items))
	for i, v := range items {
		res[i].Code = v.Code
		str := types.ValidateAndConvertProduce(&v)
		if str != "" {
			res[i].Err = service.FormatError{Message: str}
			continue
		}
		for _, w := range d.existing {
			if v.Code == w.Code {
				res[i].Err = store.AlreadyExistsError{Code: "Dup"}
				break
			}
		}
		d.existing = append(d.existing, v)
	}
	return res, nil
}

// Delete deletes single produce item from the store or returns an error
// if it fails.
func (d DummyService) Delete(ctx context.Context, code string) error {
	return d.err
}

// ListAll fetches all produce items from the store or returns an error
// if it fails.
func (d DummyService) ListAll(context.Context) ([]types.Produce, error) {
	return d.existing, d.err
}

// Clear is a convenience API to reset the database, useful for testing.
func (d DummyService) Clear(context.Context) error {
	return d.err
}
