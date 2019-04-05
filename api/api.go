// Package api is the endpoint implementation for the produce service.
// The HTTP endpoint implmentations are here.  This package deals with
// unmarshaling and marshaling payloads, dispatching to the service (which is
// simply the store), processing those errors, and implementing proper
// REST semantics.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/gdotgordon/produce-demo/service"
	"github.com/gdotgordon/produce-demo/store"
	"github.com/gdotgordon/produce-demo/types"
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
	mux.Handle("/v1/status", wrapContext(ctx, ap.getStatus))
	mux.Handle("/v1/produce", wrapContext(ctx, ap.handleProduce))
	return nil
}

// Liveness check
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

// Handle all produce endpoints
func (a *apiImpl) handleProduce(w http.ResponseWriter, r *http.Request) {

	// This wouldn't be necessary if using a high-quality muxer such
	// as Gorilla mux.
	switch r.Method {
	case http.MethodPost:
		a.handleAdd(w, r)
	case http.MethodGet:
		a.handleGet(w, r)
	case http.MethodDelete:
		a.handleDelete(w, r)
	default:
		defer r.Body.Close()
		http.NotFound(w, r)
		return
	}
}

func (a apiImpl) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}

	// Again, all of this extra URL processing is due to using the
	// standard Go muxer.
	path := r.URL.EscapedPath()
	path, err := url.PathUnescape(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("URL unescape error"))
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Unmarshal the request item.
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

	for i, v := range par.Items {
		str, val := types.ValidateAndConvertProduceCode(v.Code)
		if !val {
			msg := fmt.Sprintf("invalid code: '%s'", v.Code)
			status := types.StatusResponse{Status: msg}
			b, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			w.WriteHeader(http.StatusBadRequest)
			w.Write(b)
			return
		}
		par.Items[i].Code = str

		str, val = types.ValidateAndConvertName(v.Name)
		if !val {
			msg := fmt.Sprintf("invalid name: '%s'", v.Code)
			status := types.StatusResponse{Status: msg}
			b, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			w.WriteHeader(http.StatusBadRequest)
			w.Write(b)
			return
		}
		par.Items[i].Name = str
	}

	// Invoke the service to do the add
	addRes, err := a.service.Add(r.Context(), par.Items)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// If there is more than one add, and at least one failure, we'll return
	// HTTP 207 (multi-status) and return a JSON object with the results of each
	// individual add.  If there is only one object or no failiures, we'll
	// return HTTP 201 with no payload.
	restResp := make([]types.ProduceAddItemResponse, len(addRes))
	failures := 0
	for i, v := range addRes {
		restResp[i].Code = v.Code
		if v.Err != nil {
			failures++
		}
	}
	if failures == 0 {
		w.WriteHeader(http.StatusCreated)
		return
	}

	// At least one failed, so we'll go with the HTTP 207 and show
	for i, v := range addRes {
		restResp[i].Code = v.Code
		if v.Err != nil {
			restResp[i].Error = v.Err.Error()
		}
		switch v.Err.(type) {
		case service.InternalError:
			restResp[i].StatusCode = http.StatusInternalServerError
		case store.AlreadyExistsError:
			restResp[i].StatusCode = http.StatusConflict
		case nil:
			// Add was successfiul
			restResp[i].StatusCode = http.StatusNoContent
		default:
			restResp[i].StatusCode = http.StatusInternalServerError
		}
	}

	// We're going to return HTTP 207 along with the descritpive JSON.
	b, err = json.Marshal(par)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusMultiStatus)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(b)
}

func (a apiImpl) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}

	// The last part of the request URL should have the ID to delete.
	path := r.URL.EscapedPath()
	path, err := url.PathUnescape(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("URL unescape error"))
		return
	}

	// Make sure it is the correct URL
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		w.WriteHeader(http.StatusBadRequest)
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
	w.WriteHeader(http.StatusInternalServerError)
}

func (a apiImpl) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}

	// The last part of the request URL should have the ID to delete.
	path := r.URL.EscapedPath()
	path, err := url.PathUnescape(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("URL unescape error"))
		return
	}

	// Extract the code from the request URL and validate it
	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	code, valid := types.ValidateAndConvertProduceCode(parts[2])
	if !valid {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Invoke the service delete call
	err = a.service.Delete(r.Context(), code)
	switch err.(type) {
	case service.InternalError:
		w.WriteHeader(http.StatusInternalServerError)
	case store.NotFoundError:
		w.WriteHeader(http.StatusNotFound)
	case nil:
		// Delete was successful - write HTTP 200 plus status
		msg := fmt.Sprintf("Produce Code '%s' was successfully deleted", code)
		s := types.StatusResponse{Status: msg}
		b, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(b)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// Weave the context into the incoming request in case there is anything
// of use stored in it.
func wrapContext(ctx context.Context, hf http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := r.WithContext(ctx)
		hf(w, rc)
	})
}
