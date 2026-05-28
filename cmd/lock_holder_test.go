package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
)

type synchronizedBuffer struct {
	mu  sync.RWMutex
	buf bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.buf.String()
}

func startStateLockHolder(t *testing.T, lockPath string) func() {
	t.Helper()
	helper := exec.Command(os.Args[0], "-test.run=TestStateLockHolderProcess", "--", lockPath)
	helper.Env = append(os.Environ(), "SPECLANE_TEST_HOLD_LOCK=1")

	var stdout synchronizedBuffer
	var stderr synchronizedBuffer
	helper.Stdout = &stdout
	helper.Stderr = &stderr
	stdin, err := helper.StdinPipe()
	if err != nil {
		t.Fatalf("create helper stdin pipe: %v", err)
	}
	if err := helper.Start(); err != nil {
		t.Fatalf("start lock holder helper: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(stdout.String(), "lock-holder-ready") {
		if time.Now().After(deadline) {
			_ = stdin.Close()
			_ = helper.Process.Kill()
			_ = helper.Wait()
			t.Fatalf("lock holder helper did not become ready: stderr=%s", stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	return func() {
		_ = stdin.Close()
		waitCh := make(chan error, 1)
		go func() {
			waitCh <- helper.Wait()
		}()
		select {
		case <-time.After(3 * time.Second):
			_ = helper.Process.Kill()
			<-waitCh
		case <-waitCh:
		}
	}
}

func TestStateLockHolderProcess(t *testing.T) {
	if os.Getenv("SPECLANE_TEST_HOLD_LOCK") != "1" {
		t.Skip("helper subprocess only")
	}

	lockPath, ok := helperArgAfterMarker(os.Args, "--")
	if !ok {
		fmt.Fprintln(os.Stderr, "missing lock path helper arg")
		os.Exit(2)
	}
	lock := fsutil.NewStateLock(lockPath)
	held, err := lock.Acquire(context.Background(), 0, "test-lock-holder")
	if err != nil {
		fmt.Fprintf(os.Stderr, "acquire lock: %v\n", err)
		os.Exit(3)
	}
	_, _ = fmt.Fprintln(os.Stdout, "lock-holder-ready")
	_ = os.Stdout.Sync()
	_, _ = io.Copy(io.Discard, os.Stdin)
	_ = held.Release()
	os.Exit(0)
}

func helperArgAfterMarker(args []string, marker string) (string, bool) {
	for i, arg := range args {
		if arg != marker {
			continue
		}
		if i+1 >= len(args) {
			return "", false
		}
		return strings.TrimSpace(args[i+1]), true
	}
	return "", false
}
