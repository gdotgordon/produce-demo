// Package api is the endpoint implementation for the produce service.
// The HTTP endpoint implmentations are here.  This package deals with
// unmarshaling and marshaling payloads, dispatching to the service (which is
// simply the store), processing those errors, and implementing proper
// REST semantics.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/gdotgordon/produce-demo/store"
	"github.com/gdotgordon/produce-demo/types"
)

// API is the item that dispatches to the endpoint implmentations
type API struct {
	store store.ProduceStore
}

// Init sets up the endpoint processing.  There is nothing returned, other
// than potntial errors, because the endpoint handling is configured in
// the passed-in muxer.
func Init(ctx context.Context, mux *http.ServeMux, store store.ProduceStore) error {
	ap := API{}
	mux.Handle("/v1/status", wrapContext(ctx, ap.getStatus))
	mux.Handle("/v1/produce", wrapContext(ctx, ap.handleProduce))
	return nil
}

// Liveness check
func (a *API) getStatus(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

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

// Handle all produce endpoints
func (a *API) handleProduce(w http.ResponseWriter, r *http.Request) {

	// This wouldn't be necessary if using a high-quality muxer such
	// as Gorilla mux.
	switch r.Method {
	case http.MethodPost:
	case http.MethodGet:
	case http.MethodDelete:
		a.handleDelete(w, r)
	default:
		defer r.Body.Close()
		http.NotFound(w, r)
		return
	}
}

func (a *API) handleDelete(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// The last part of the request URL should have the ID to delete.
	path := r.URL.EscapedPath()
	path, err := url.PathUnescape(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("URL unescape error"))
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	code := strings.ToUpper(parts[2])
	if !types.CodeExp.Match([]byte(code)) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ch := make(chan error)
	defer close(ch)

	// Run the delete in a goroutine as requested by the spec.
	var wch chan<- error = ch
	go func() {
		delErr := a.store.Delete(r.Context(), code)
		wch <- delErr
	}()

	// And wait for the return, which is just an error.
	err, ok := <-ch
	if !ok {
		// Channel was mysteriously closed!
		w.WriteHeader(http.StatusInternalServerError)
	}
	if err != nil {
		_, ok := err.(store.NotFoundError)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
	// http.StatusOK will be written if we haven't set a status
}

// Weave the context into the incoming request in case there is anything
// of use stored in it.
func wrapContext(ctx context.Context, hf http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := r.WithContext(ctx)
		hf(w, rc)
	})
}
