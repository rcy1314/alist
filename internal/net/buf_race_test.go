package net

import (
	"context"
	"sync"
	"testing"
)

// TestBufCloseWriteRace exercises the race fixed for issue #9537/#9190:
// Buf.Close() must synchronize with concurrent Buf.Write() calls so that
// nilling the underlying bytes.Buffer cannot panic an in-flight writer.
// Run with `go test -race ./internal/net/...` to be meaningful.
func TestBufCloseWriteRace(t *testing.T) {
	const iters = 200
	const writers = 8

	for i := 0; i < iters; i++ {
		buf := NewBuf(context.Background(), 1024)
		var wg sync.WaitGroup
		for w := 0; w < writers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panic in Write: %v", r)
					}
				}()
				for j := 0; j < 50; j++ {
					_, _ = buf.Write([]byte("x"))
				}
			}()
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic in Close: %v", r)
				}
			}()
			buf.Close()
		}()
		wg.Wait()
	}
}
