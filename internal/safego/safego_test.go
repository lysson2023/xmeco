package safego

import (
	"context"
	"os"
	"testing"
)

func TestGo_NormalExecution(t *testing.T) {
	done := make(chan bool)
	Go("test-normal", context.Background(), func(ctx context.Context) {
		done <- true
	})
	<-done
}

func TestGo_ContextPassed(t *testing.T) {
	type ctxKey struct{}
	done := make(chan context.Context)
	ctx := context.WithValue(context.Background(), ctxKey{}, "v42")

	Go("test-ctx", ctx, func(innerCtx context.Context) {
		done <- innerCtx
	})
	got := <-done
	if v, ok := got.Value(ctxKey{}).(string); !ok || v != "v42" {
		t.Errorf("context value = %v, want v42", v)
	}
}

func TestGo_PanicRecovered(t *testing.T) {
	// Verify that panic inside the goroutine is recovered and the goroutine
	// exits cleanly without crashing the test process.
	done := make(chan bool)
	Go("test-panic", context.Background(), func(ctx context.Context) {
		defer func() { done <- true }()
		panic("intentional test panic")
	})
	<-done // blocks forever if recover() fails
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
