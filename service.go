// Copyright 2021 Canonical Ltd.

// Package service provides helpers for long-running service applications.
package service

import (
	"context"
	"os"
	"os/signal"

	"golang.org/x/sync/errgroup"
)

// A Service is a service provided by a number of goroutines which will
// initiate a graceful shutdown when either one of those goroutines errors,
// or on the receipt of chosen signals.
type Service struct {
	g *errgroup.Group

	doneC     <-chan struct{}
	shutdownC chan<- func()
}

// NewService creates a new service instance using the given context. If
// any signals are specified the service will start a shutdown upon
// receiving that signal.
func NewService(ctx context.Context, sig ...os.Signal) (context.Context, *Service) {
	g, ctx := errgroup.WithContext(ctx)

	if len(sig) > 0 {
		sigC := make(chan os.Signal, 1)
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case sig := <-sigC:
				return &SignalError{
					Signal: sig,
				}
			}
		})
		signal.Notify(sigC, sig...)
	}

	shutdownC := make(chan func())
	g.Go(func() error {
		var funcs []func()
		for {
			select {
			case f := <-shutdownC:
				funcs = append(funcs, f)
			case <-ctx.Done():
				for i := len(funcs) - 1; i >= 0; i-- {
					funcs[i]()
				}
				return ctx.Err()
			}
		}
	})

	return ctx, &Service{
		g:         g,
		doneC:     ctx.Done(),
		shutdownC: shutdownC,
	}
}

// Go calls the given function in a new goroutine.
//
// The first call to return a non-nil error cancels the service; its error
// will be returned by Wait.
func (s *Service) Go(f func() error) {
	s.g.Go(f)
}

// Wait waits for all goroutines started by this service and all functions
// registered with OnShutdown to complete. The error returned will be the
// error that caused the service to be canceled, if any.
func (s *Service) Wait() error {
	return s.g.Wait()
}

// OnShutdown registers a function to be called when the service determines
// it is shutting down. The Wait function will wait for all functions
// provided to OnShutdown to complete before returning.
func (s *Service) OnShutdown(f func()) {
	select {
	case s.shutdownC <- f:
	case <-s.doneC:
		f()
	}
}

// A SignalError is the type of error returned when a Service has shutdown
// due to receiving a signal.
type SignalError struct {
	Signal os.Signal
}

// Error implements the error interface.
func (e *SignalError) Error() string {
	return "received " + e.Signal.String()
}
