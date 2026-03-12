package profile

import (
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestEnablePprof_DoubleClose_Simulated(t *testing.T) {
	// We can't easily trigger the internal closeOnce from outside,
	// but we can test that our EnablePprof handles the error case correctly.

	// 1. Listen on a port to cause ListenAndServe to fail later
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	// 2. Call EnablePprof with the same address.
	stopChan := EnablePprof(addr)

	// 3. Wait for it to close due to error.
	select {
	case <-stopChan:
		t.Log("Channel closed as expected")
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for pprofStopChan to be closed")
	}

	// 4. Send signal to trigger the other close.
	// Since we are using sync.Once now, this should NOT panic.
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGINT)

	// Wait to see if it panics.
	time.Sleep(500 * time.Millisecond)
}
