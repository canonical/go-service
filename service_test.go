// Copyright 2021 Canonical Ltd.

package service

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"
)

func TestSignal(t *testing.T) {
	ctx, svc := NewService(context.Background(), syscall.SIGUSR1)
	svc.Go(func() error {
		p, err := os.FindProcess(os.Getpid())
		if err != nil {
			return err
		}
		if err := p.Signal(syscall.SIGUSR1); err != nil {
			return err
		}
		<-ctx.Done()
		return nil
	})
	err := svc.Wait()
	if err.Error() != "received user defined signal 1" {
		t.Error("unexpected error:", err)
	}
}

func TestServiceError(t *testing.T) {
	_, svc := NewService(context.Background(), syscall.SIGUSR2)
	svc.Go(func() error {
		return errors.New("test error")
	})
	err := svc.Wait()
	if err.Error() != "test error" {
		t.Error("unexpected error:", err)
	}
}

func TestOnShutdown(t *testing.T) {
	_, svc := NewService(context.Background())
	var mu sync.Mutex
	var ops []string
	svc.OnShutdown(func() {
		mu.Lock()
		defer mu.Unlock()
		ops = append(ops, "shutdown-1")
	})
	svc.OnShutdown(func() {
		mu.Lock()
		defer mu.Unlock()
		ops = append(ops, "shutdown-2")
	})
	svc.Go(func() error {
		mu.Lock()
		defer mu.Unlock()
		ops = append(ops, "go-1")
		return errors.New("test error")
	})
	err := svc.Wait()
	if err.Error() != "test error" {
		t.Error("unexpected error:", err)
	}
	svc.OnShutdown(func() {
		mu.Lock()
		defer mu.Unlock()
		ops = append(ops, "shutdown-3")
	})
	mu.Lock()
	defer mu.Unlock()
	if len(ops) != 4 {
		t.Fatal("unexpected operations occured:", ops)
	}
	if ops[0] != "go-1" {
		t.Fatal("shutdown operations happened too early", ops)
	}
}
