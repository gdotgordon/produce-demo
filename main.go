// Package main runs the produce microservice.  It spins up an http
// server to handle requests, which are handled by the api package.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdotgordon/produce-demo/api"
	"github.com/gdotgordon/produce-demo/service"
	"github.com/gdotgordon/produce-demo/store"
)

var (
	portNum = flag.Int("port", 8080, "HTTP port number")
)

func main() {

	// We'll propagate the context with cancel thorughout the program,
	// such as http clients, server methods we implement, and other
	// loops using channels.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create the server to handle geocode lookups.  The API module will
	// set up the routes, as we don't need to know the details in the
	// main program.
	muxer := http.NewServeMux()
	service := service.New(store.New())
	if err := api.Init(ctx, muxer, service); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting api: '%s'\n", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Handler:      muxer,
		Addr:         fmt.Sprintf(":%d", *portNum),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start Server
	go func() {
		log.Println("Starting Server")
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	// Block until we shutdown.
	waitForShutdown(ctx, srv)
}

func waitForShutdown(ctx context.Context, srv *http.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive our signal.
	<-interruptChan

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("Shutting down")
}
