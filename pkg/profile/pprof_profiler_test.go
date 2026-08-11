package profile

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestEnablePprof_GracefulShutdown(t *testing.T) {
	// 1. Listen on a port to get a free address
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close() // Close it so EnablePprof can use it

	ctx, cancel := context.WithCancel(context.Background())
	// 2. Call EnablePprof
	stopChan := EnablePprof(ctx, addr)

	// Wait a bit for it to start
	time.Sleep(100 * time.Millisecond)

	// 3. Cancel context to trigger shutdown
	cancel()

	// 4. Wait for it to close.
	select {
	case <-stopChan:
		t.Log("Channel closed as expected")
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for pprofStopChan to be closed")
	}
}
