// Package api is the endpoint implementation for the produce service.
// The HTTP endpoint implmentations are here.  This package deals with
// unmarshaling and marshaling payloads, dispatching to the service (which is
// itself contains an instance of the storee), processing those errors,
// and implementing proper REST semantics.
package api

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gdotgordon/produce-demo/service"
	"github.com/gdotgordon/produce-demo/store"
	"github.com/gdotgordon/produce-demo/types"
)

// The staus URL is a liveness check, and the produce endpoint is the

// Definitions for the supported URLs.
const (
	statusURL  = "/v1/status"
	produceURL = "/v1/produce"
)

// API is the item that dispatches to the endpoint implmentations
type apiImpl struct {
	service service.Service
}

// Init sets up the endpoint processing.  There is nothing returned, other
// than potntial errors, because the endpoint handling is configured in
// the passed-in muxer.
func Init(ctx context.Context, mux *http.ServeMux, service service.Service) error {
	ap := apiImpl{service: service}
	mux.Handle(statusURL, wrapContext(ctx, ap.getStatus))
	mux.Handle(produceURL, wrapContext(ctx, ap.handleProduce))
	mux.Handle(produceURL+"/", wrapContext(ctx, ap.handleProduce))
	return nil
}

// Liveness check endpoint
func (a apiImpl) getStatus(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}

	// Where is Gorilla mux when I need it?
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	sr := types.StatusResponse{Status: "produce service up and running"}
	b, err := json.MarshalIndent(sr, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("JSON encode error"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

// Handle all produce endpoints.  Due to the restriction of not using
// a third-partry muxer, we need to manually work with the dispatch
// of the "v1/produce" endpoint.
func (a *apiImpl) handleProduce(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.handleAdd(w, r)
	case http.MethodGet:
		a.handleGet(w, r)
	case http.MethodDelete:
		a.handleDelete(w, r)
	default:
		if r.Body != nil {
			r.Body.Close()
		}
		http.NotFound(w, r)
		return
	}
}

// Handler for POST/add new produce.  We are asked to add mutliple items
// at once, but not all of them may succeed.  On the other hand, there
// is no requirement or rationale for transactionality, so we may end up
// with partial successes when there are multpile items to add.
//
// We handle the case of partial success by sending an HTTP 200 and returning
// a json list of the individual results.  Since this API is arguably not
// purely Restful, it is a topic where ten different resources propose ten
// different ways of doing it, so I picked a reasonable one that somewhat
// stays within REST semantics.
func (a apiImpl) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	log.Printf("handling POST request")

	// Ensure the URL path is exactly the produce base URL.
	_, ok := extractPath(w, r, produceURL)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Unmarshal the request item.  Note adding 0 items is deemed an error.
	var par types.ProduceAddRequest
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err = json.Unmarshal(b, &par); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(par.Items) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Invoke the service to do the add
	addRes, err := a.service.Add(r.Context(), par.Items)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// If there was only one item to add, handle that without the mass response.
	if len(par.Items) == 1 {
		if addRes[0].Err == nil {
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(errorToStatusCode(addRes[0].Err, http.StatusCreated))
		}
		return
	}
	// If there is more than one add, and at least one failure, we'll return
	// HTTP 200 (multi-status) and return a JSON object with the results of each
	// individual add.  If there are no failures, we'll return HTTP 201 with
	// no payload.
	restResp := make([]types.ProduceAddItemResponse, len(addRes))
	failures := 0
	for i, v := range addRes {
		restResp[i].Code = v.Code
		if v.Err != nil {
			failures++
			restResp[i].Error = v.Err.Error()
		}
		restResp[i].StatusCode = errorToStatusCode(v.Err, http.StatusCreated)
	}
	restWrapper := types.ProduceAddResponse{Items: restResp}

	// If no failures, return a single created response.
	if failures == 0 {
		w.WriteHeader(http.StatusCreated)
		return
	}

	// At least one failuire, so we're going to return HTTP 200 along with
	// the descritpive JSON.
	b, err = json.Marshal(restWrapper)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(b)
}

// The Get Rest handler simply lists all the items in the database.
// It is valid and meaningful to return an empty array.
func (a apiImpl) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}

	log.Printf("handling GET request")

	// The last part of the request URL should have the ID to delete.
	_, ok := extractPath(w, r, produceURL)
	if !ok {
		return
	}

	// Invoke the service list items call
	items, err := a.service.ListAll(r.Context())
	switch err.(type) {
	case service.InternalError:
		w.WriteHeader(http.StatusInternalServerError)
	case nil:
		// List was successful - write HTTP 200
		resp := types.ProduceListResponse{Items: items}
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// The delete endpoint contains the proudce code as the last part of the
// URL path.  Query strings ar etypically for modfiers, whereas putting
// it as the last component of the path is more Restful, as it is the
// name pof the resource.
func (a apiImpl) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}

	log.Printf("handling DELETE request")

	// The last part of the request URL should have the ID to delete.
	path := r.URL.EscapedPath()
	path, err := url.PathUnescape(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("URL unescape error"))
		return
	}

	// Extract the code from the request URL and validate it
	if strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	if strings.Count(path, "/") != 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	code := path[strings.LastIndex(path, "/")+1:]

	// Invoke the service delete call
	err = a.service.Delete(r.Context(), code)
	w.WriteHeader(errorToStatusCode(err, http.StatusNoContent))
}

// Map a Go eror to an HTTP status type
func errorToStatusCode(err error, nilCode int) int {
	switch err.(type) {
	case service.InternalError:
		return http.StatusInternalServerError
	case service.FormatError:
		return http.StatusBadRequest
	case store.AlreadyExistsError:
		return http.StatusConflict
	case store.NotFoundError:
		return http.StatusNotFound
	case nil:
		return nilCode
	default:
		return http.StatusInternalServerError
	}
}

// extractPath extracts and unescapes the path component.  If an
// error occurs, it writes the proper response channel data and
// sets a false boolean result.
func extractPath(w http.ResponseWriter, r *http.Request,
	expURL string) (string, bool) {
	// The last part of the request URL should have the ID to delete.
	path := r.URL.EscapedPath()
	path, err := url.PathUnescape(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("URL unescape error"))
		return "", false
	}

	// Make sure it is the correct URL
	if path != expURL && path != expURL+"/" {
		w.WriteHeader(http.StatusBadRequest)
		return "", false
	}
	return path, true
}

// Weave the context into the incoming request in case there is anything
// of use stored in it.
func wrapContext(ctx context.Context, hf http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := r.WithContext(ctx)
		hf(w, rc)
	})
}
