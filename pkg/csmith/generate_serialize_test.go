package csmith

import (
	"sync"
	"sync/atomic"
	"testing"
)

// Generate serializes full runs: C++ is one process / one generation; package
// globals (pointerCache, Error, Bookkeeper, stm_labels, finalization) are
// process-wide. Concurrent Generate (go test -fuzz multi-worker) must not race.
func TestGenerateSerializesConcurrentCalls(t *testing.T) {
	var wg sync.WaitGroup
	var panics, ok atomic.Int64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
					t.Errorf("panic seed=%d: %v", seed, r)
				}
			}()
			opts := Defaults()
			opts.Seed = seed
			out, err := Generate(opts)
			if err != nil || out == "" {
				t.Errorf("seed=%d err=%v empty=%v", seed, err, out == "")
				return
			}
			ok.Add(1)
		}(uint64(10 + i))
	}
	wg.Wait()
	if panics.Load() != 0 {
		t.Fatalf("panics=%d", panics.Load())
	}
	if ok.Load() != 8 {
		t.Fatalf("ok=%d want 8", ok.Load())
	}
}
