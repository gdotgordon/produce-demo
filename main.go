// Package main runs the produce microservice.  It spins up an http
// server to handle requests, which are handled by the api package.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gdotgordon/produce-demo/api"
	"github.com/gdotgordon/produce-demo/service"
	"github.com/gdotgordon/produce-demo/store"
	"github.com/gdotgordon/produce-demo/types"
	"go.uber.org/zap"
)

const (
	seedFile = "seed.json"
)

var (
	portNum  int    // listen port
	logLevel string // zap log level
	timeout  int    // server timeout in seconds
)

func init() {
	flag.IntVar(&portNum, "port", 8080, "HTTP port number")
	flag.StringVar(&logLevel, "log", "production",
		"log level: 'production', 'development'")
	flag.IntVar(&timeout, "timeout", 30, "server timeout (seconds)")
}

func main() {
	flag.Parse()

	// We'll propagate the context with cancel thorughout the program,
	// such as http clients, server methods we implement, and other
	// loops using channels.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up logging.
	log, err := initLogging()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v", err)
		os.Exit(1)
	}

	// Create the server to handle the produce service.  The API module will
	// set up the routes, as we don't need to know the details in the
	// main program.
	muxer := http.NewServeMux()
	service := service.New(store.New(), log)
	if err := api.Init(ctx, muxer, service, log); err != nil {
		log.Errorf("Error initializing API layer", "error", err)
		os.Exit(1)
	}

	// Load the seed items as (required by the spec), from the seed.json file.
	if err := loadSeedItems(ctx, service, log); err != nil {
		log.Errorw("Error loading seed items", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Handler:      muxer,
		Addr:         fmt.Sprintf(":%d", portNum),
		ReadTimeout:  time.Duration(timeout) * time.Second,
		WriteTimeout: time.Duration(timeout) * time.Second,
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

func loadSeedItems(ctx context.Context, service service.Service,
	log *zap.SugaredLogger) error {
	seedFilePath, _ := os.Executable()
	seedFilePath = filepath.Dir(seedFilePath) + "/" + seedFile
	seedFile, err := os.Open(seedFilePath)
	if err != nil {
		log.Warnw("Cannot open produce seed file", "file", seedFile, "error", err)
		return nil
	}
	defer seedFile.Close()
	b, err := ioutil.ReadAll(seedFile)
	if err != nil {
		log.Errorw("Error reading produce seed file", "error", err)
		seedFile.Close()
		os.Exit(1)
	}
	var items []types.Produce
	if err = json.Unmarshal(b, &items); err != nil {
		log.Errorw("Error unmarshalling produce seed file", "error", err)
		os.Exit(1)
	}
	addItems, err := service.Add(ctx, items)
	if len(addItems) == 0 {
		log.Warn("No seed items loaded")
	}
	return err
}

// set up the logger, condsidering any env vars.
func initLogging() (*zap.SugaredLogger, error) {
	var lg *zap.Logger
	var err error

	pdl := strings.ToLower(os.Getenv("PRODUCE_LOG_LEVEL"))
	if strings.HasPrefix(pdl, "d") {
		logLevel = "development"
	}

	if logLevel == "development" {
		lg, err = zap.NewDevelopment()
	} else {
		lg, err = zap.NewProduction()
	}
	if err != nil {
		return nil, err
	}
	return lg.Sugar(), nil // ♫ ♩ ♩ ♫ ah honey honey
}

// Setup for clean shutdown with signal handlers/cancel.
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
