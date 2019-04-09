// Package main runs the produce microservice.  It spins up an http
// server to handle requests, which are handled by the api package.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdotgordon/produce-demo/api"
	"github.com/gdotgordon/produce-demo/service"
	"github.com/gdotgordon/produce-demo/store"
	"go.uber.org/zap"
)

var (
	portNum  int
	logLevel string
)

func init() {

	flag.IntVar(&portNum, "port", 8080, "HTTP port number")

	flag.StringVar(&logLevel, "log", "production",
		"log level: 'production', 'development'")
}

func main() {
	flag.Parse()

	var lg *zap.Logger
	var err error
	if logLevel == "development" {
		lg, err = zap.NewDevelopment()
	} else {
		lg, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v", err)
		os.Exit(1)
	}
	log := lg.Sugar() // ♫ ♩ ♩ ♫ ah honey honey

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
	if err := api.Init(ctx, muxer, service, log); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting api: '%s'\n", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Handler:      muxer,
		Addr:         fmt.Sprintf(":%d", portNum),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start Server
	go func() {
		log.Infow("Listening for connections", "port", portNum)
		if err := srv.ListenAndServe(); err != nil {
			log.Infow("Server completed", "err", err)
		}
	}()

	// Block until we shutdown.
	waitForShutdown(ctx, srv, log)
}

func waitForShutdown(ctx context.Context, srv *http.Server,
	log *zap.SugaredLogger) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive our signal.
	sig := <-interruptChan
	log.Debugw("Termination signal received", "signal", sig)

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

	log.Infof("Shutting down")
}
