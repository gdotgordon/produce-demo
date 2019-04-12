// Package api is the endpoint implementation for the produce service.
// The HTTP endpoint implmentations are here.  This package deals with
// unmarshaling and marshaling payloads, dispatching to the service (which is
// itself contains an instance of the store), processing those errors,
// and implementing proper REST semantics.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/gdotgordon/produce-demo/service"
	"github.com/gdotgordon/produce-demo/store"
	"github.com/gdotgordon/produce-demo/types"
	"go.uber.org/zap"
)

// Definitions for the supported URLs.
const (
	statusURL  = "/v1/status"
	produceURL = "/v1/produce"
	resetURL   = "/v1/reset"
)

// API is the item that dispatches to the endpoint implementations
type apiImpl struct {
	service service.Service
	log     *zap.SugaredLogger
}

// Init sets up the endpoint processing.  There is nothing returned, other
// than potntial errors, because the endpoint handling is configured in
// the passed-in muxer.
func Init(ctx context.Context, mux *http.ServeMux, service service.Service,
	log *zap.SugaredLogger) error {
	ap := apiImpl{service: service, log: log}
	mux.Handle(statusURL, wrapContext(ctx, ap.getStatus))
	mux.Handle(produceURL, wrapContext(ctx, ap.handleProduce))
	mux.Handle(produceURL+"/", wrapContext(ctx, ap.handleProduce))
	mux.Handle(resetURL, wrapContext(ctx, ap.handleReset))
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
	sr := types.StatusResponse{Status: "produce service is up and running"}
	b, err := json.MarshalIndent(sr, "", "  ")
	if err != nil {
		a.notifyInternalServerError(w, "json marshal failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

// Handle all produce endpoints.  with the Go built-in muxer, we need to
// manually work with the dispatch of the "/v1/produce" endpoint.
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
// For a single item that succeeds we return HTTP 201 for success.
//
// For partial successes we return HTTP 200 and returning a json list of
// the individual results, with proper semantics per result.  So an item
// That was sucessfully added will show 201 in it's inidivdual status, even
// though the over all status returned is 200.  Likewise, if any or all
// of the requests fail, each will have the proper HTTP status, but again,
// HTTP 200 will be returned.
//
// An attempt to add an item already present generates HTTP 409 (Conflict).
//
// For individual items added, we do support incoming JSON for a single
// Produce item not enclosed in an array.
//
// Since this API is arguably not purely Restful, it is a topic where ten
// different sources propose ten different ways of doing it, so I picked a
// reasonable one that somewhat stays within REST semantics.
func (a apiImpl) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		writeBadRequestResponse(w, errors.New("No body for POST"))
		return
	}
	defer r.Body.Close()

	a.log.Debugw("handling POST request", "url", r.URL.String())

	// Ensure the URL path is exactly the produce base URL.
	_, ok := a.extractPath(w, r, produceURL)
	if !ok {
		return
	}

	// Unmarshal the request item.  Note adding 0 items is deemed an error.
	var items types.ProduceAddRequest
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		a.notifyInternalServerError(w, "error reading request body", err)
		return
	}

	// Unmarshal the payload either into a produce item slice, or if not,
	// then try as a single item.
	if err = json.Unmarshal(b, &items); err != nil {
		// See if this is in fact a single produce item.
		var prod types.Produce
		serr := json.Unmarshal(b, &prod)
		if serr == nil {
			items = []types.Produce{prod}
		} else {
			writeBadRequestResponse(w, err)
			return
		}
	}

	if len(items) == 0 {
		writeBadRequestResponse(w,
			errors.New("At least one item must be specifed to add"))
		return
	}

	// Invoke the service to do the add
	addRes, err := a.service.Add(r.Context(), items)

	if err != nil {
		a.notifyInternalServerError(w, "server error from Add", err)
		return
	}

	// If there was only one item to add, handle that without the mass response.
	if len(items) == 1 {
		if addRes[0].Err == nil {
			w.WriteHeader(http.StatusCreated)
		} else {
			sc := errorToStatusCode(addRes[0].Err, http.StatusCreated)
			if sc == http.StatusBadRequest {
				writeBadRequestResponse(w, addRes[0].Err)
			} else {
				w.WriteHeader(sc)
			}
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

	// If no failures, return a single created response.
	if failures == 0 {
		w.WriteHeader(http.StatusCreated)
		return
	}

	// At least one failuire, so we're going to return HTTP 200 along with
	// the descritpive JSON.
	b, err = json.Marshal(restResp)
	if err != nil {
		a.notifyInternalServerError(w, "JSON marshal error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

// The Get Rest handler simply lists all the items in the database.
// It is valid and meaningful to return an empty array.  It normally
// returns HTTP 200.
func (a apiImpl) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}

	a.log.Debugw("handling GET request", "url", r.URL.String())

	// The last part of the request URL should have the ID to delete.
	_, ok := a.extractPath(w, r, produceURL)
	if !ok {
		return
	}

	// Invoke the service list items call
	items, err := a.service.ListAll(r.Context())
	switch err.(type) {
	case service.InternalError:
		a.notifyInternalServerError(w, "error listing items", err)
	case nil:
		// List was successful - write HTTP 200
		b, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			a.notifyInternalServerError(w, "JSON marshal error", err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	default:
		a.notifyInternalServerError(w, "an unexpected problem occurred", err)
	}
}

// The delete endpoint contains the proudce code as the last part of the
// URL path.  Query strings ar etypically for modfiers, whereas putting
// it as the last component of the path is more Restful, as it is the
// name of the resource.
//
// A 204 code (No Content) is returned if successful, 404 if not found,
// 400 if syntax is incorrect.
func (a apiImpl) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}

	a.log.Debugw("handling DELETE request", "url", r.URL.String())

	// The last part of the request URL should have the ID to delete.
	path := r.URL.EscapedPath()
	path, err := url.PathUnescape(path)
	if err != nil {
		a.notifyInternalServerError(w, "URL unescape error", err)
		return
	}

	// Extract the code from the request URL and validate it
	if strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	if strings.Count(path, "/") != 3 {
		writeBadRequestResponse(w, fmt.Errorf("invalid URL for delete: %s",
			r.URL.String()))
		return
	}
	code := path[strings.LastIndex(path, "/")+1:]

	// Invoke the service delete call
	err = a.service.Delete(r.Context(), code)
	sc := errorToStatusCode(err, http.StatusNoContent)
	if sc == http.StatusBadRequest {
		writeBadRequestResponse(w, err)
	} else {
		w.WriteHeader(sc)
	}
}

// extractPath extracts and unescapes the path component.  If an
// error occurs, it writes the proper response channel data and
// sets a false boolean result.
func (a apiImpl) extractPath(w http.ResponseWriter, r *http.Request,
	expURL string) (string, bool) {
	// The last part of the request URL should have the ID to delete.
	path := r.URL.EscapedPath()
	path, err := url.PathUnescape(path)
	if err != nil {
		a.notifyInternalServerError(w, "cannot unescape URL", err)
		return "", false
	}

	// Make sure it is the correct URL
	if path != expURL && path != expURL+"/" {
		a.log.Errorw("received unexpected URL", "url", path)
		writeBadRequestResponse(w, fmt.Errorf("invalid URL: %s", path))
		return "", false
	}
	return path, true
}

func (a apiImpl) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}
	code := errorToStatusCode(a.service.Clear(r.Context()), http.StatusOK)
	w.WriteHeader(code)
}

func (a apiImpl) notifyInternalServerError(w http.ResponseWriter, msg string,
	err error) {
	a.log.Errorw(msg, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
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

// For HTTP bad request repsonses, serialize a JSON status message with
// the cause.
func writeBadRequestResponse(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusBadRequest)
	b, _ := json.MarshalIndent(types.StatusResponse{Status: err.Error()}, "", "  ")
	w.Write(b)
}

// Weave the context into the incoming request in case there is anything
// of use stored in it.
func wrapContext(ctx context.Context, hf http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := r.WithContext(ctx)
		hf(w, rc)
	})
}
