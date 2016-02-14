package app_test

import (
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	"gopkg.in/crast/app.v0"
)

func Example() {
	// Set up an HTTP server
	listener, err := net.Listen("tcp", ":80")
	if err != nil {
		os.Exit(1)
	}

	// app.Serve is a shortcut for using the listener as a closer and calling Serve()
	app.Serve(listener, &http.Server{
		Handler: http.NotFoundHandler(), //example, use real handler
	})

	// You can run one-time tasks too with app.Go
	app.Go(func() error {
		if !registerMyselfSomewhere() {
			return errors.New("Registration failed. Shutting down...")
		}
		return nil
	})

	setupHeartbeat()

	// main will wait for all goroutines to complete and then run closers.
	app.Main()
}

// ping some URL every 10 seconds to let them know we're up
func setupHeartbeat() {
	app.Go(func() {
		tick := time.NewTicker(10 * time.Second)
		app.AddCloser(tick.Stop)
		for _ = range tick.C {
			http.Get("http://example.com/ping")
		}
	})
}

func registerMyselfSomewhere() bool {
	// do some monitoring thing.
	return false
}
