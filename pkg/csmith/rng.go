package csmith

import (
	"fmt"
	"os"
	"runtime"
)

const (
	// lcgA is the multiplier for the srand48/lrand48 linear congruential generator.
	lcgA uint64 = 0x5DEECE66D
	// lcgC is the addend for the srand48/lrand48 linear congruential generator.
	lcgC uint64 = 0xB
	// lcgMask restricts the state to 48 bits, matching the srand48/lrand48 specification.
	lcgMask uint64 = (1 << 48) - 1
)

// rng is compatible with the libc srand48/lrand48 core recurrence used by Csmith.
// It tracks internal PRNG state and provides tracing capabilities crucial for aligning
// Go generator decisions exactly with the C++ upstream execution path.
type rng struct {
	state     uint64
	trace     bool
	traceSite bool
	traceRaw  bool
	traceFile string
	tracePos  uint64
}

// newRNG initializes a new random number generator.
// It seeds the state using the srand48 semantics to ensure identical bit sequences
// to Csmith's original C++ implementation. It also configures tracing if environment
// variables like CSMITH_TRACE_RNG are set.
func newRNG(seed uint64) *rng {
	// srand48 semantics.
	r := &rng{state: ((seed << 16) + 0x330E) & lcgMask}
	if os.Getenv("CSMITH_TRACE_RNG") != "" {
		r.trace = true
		r.traceSite = os.Getenv("CSMITH_TRACE_RNG_SITE") != ""
		r.traceRaw = os.Getenv("CSMITH_TRACE_RNG_RAW") != ""
		r.traceFile = os.Getenv("CSMITH_TRACE_RNG_FILE")
		if r.traceFile == "" {
			r.traceFile = "/tmp/csmith-go-rng.trace"
		}
		_ = os.WriteFile(r.traceFile, []byte(fmt.Sprintf("# seed=%d\n", seed)), 0644)
	}
	return r
}

// next31 computes the next pseudo-random value.
// It returns a 31-bit unsigned integer matching the C++ stdlib lrand48() output,
// discarding the lower 17 bits of the 48-bit state.
func (r *rng) next31() uint32 {
	r.state = (lcgA*r.state + lcgC) & lcgMask
	return uint32(r.state >> 17)
}

// upto generates a random unsigned integer in the range [0, n).
// It directly modulo-maps the next31() result and logs the outcome to the trace file,
// mimicking upstream's rnd_upto() logic.
func (r *rng) upto(n uint32) uint32 {
	if n == 0 {
		return 0
	}
	raw := r.next31()
	x := raw % n
	r.traceU(n, x, 0, raw)
	return x
}

// uptoWithFilter generates a random integer in [0, n) that satisfies the provided filter.
// It repeatedly draws values until the reject function returns false.
// This matches upstream's StatementFilter retry loops and consumes RNG linearly for each retry.
func (r *rng) uptoWithFilter(n uint32, reject func(uint32) bool) uint32 {
	if n == 0 {
		return 0
	}
	raw := r.next31()
	x := raw % n
	if reject == nil || !reject(x) {
		r.traceU(n, x, 0, raw)
		return x
	}
	var tries uint32
	// Safety guard: avoid pathological infinite loops when all candidates are rejected.
	// Keep this large enough to preserve normal behavior while guaranteeing progress.
	const maxRejectRetries uint32 = 1 << 16
	for reject != nil && reject(x) && tries < maxRejectRetries {
		raw = r.next31()
		x = raw % n
		tries++
	}
	if reject != nil && reject(x) {
		panic(fmt.Sprintf("rng.uptoWithFilter: exceeded retry limit (n=%d tries=%d raw=%d)", n, tries, raw))
	}
	r.traceU(n, x, tries, raw)
	return x
}

// traceU logs an 'upto' decision to the tracing file.
// It records the sequence position, boundary, selected value, raw draw, and optionally
// the call site, which is vital for diffing against C++ Csmith trace output to identify
// divergence points.
func (r *rng) traceU(n uint32, x uint32, tries uint32, raw uint32) {
	if r.trace {
		r.tracePos++
		f, err := os.OpenFile(r.traceFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err == nil {
			if r.traceSite && r.traceRaw {
				_, _ = fmt.Fprintf(f, "%d U %d -> %d tries=%d raw=%d @%s\n", r.tracePos, n, x, tries, raw, traceCaller())
			} else if r.traceSite {
				_, _ = fmt.Fprintf(f, "%d U %d -> %d @%s\n", r.tracePos, n, x, traceCaller())
			} else if r.traceRaw {
				_, _ = fmt.Fprintf(f, "%d U %d -> %d tries=%d raw=%d\n", r.tracePos, n, x, tries, raw)
			} else {
				_, _ = fmt.Fprintf(f, "%d U %d -> %d\n", r.tracePos, n, x)
			}
			_ = f.Close()
		}
	}
}

// flipcoin returns true with probability p/100.
// It mirrors upstream's rnd_flipcoin() and uses modulo 100 on the raw 31-bit draw.
// All decisions are traced to support deterministic alignment.
func (r *rng) flipcoin(p uint32) bool {
	if p > 100 {
		p = 100
	}
	raw := r.next31()
	v := raw % 100
	ok := v < p
	if r.trace {
		r.tracePos++
		f, err := os.OpenFile(r.traceFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err == nil {
			var b uint32
			if ok {
				b = 1
			}
			if r.traceSite && r.traceRaw {
				_, _ = fmt.Fprintf(f, "%d F %d -> %d raw=%d @%s\n", r.tracePos, p, b, raw, traceCaller())
			} else if r.traceSite {
				_, _ = fmt.Fprintf(f, "%d F %d -> %d @%s\n", r.tracePos, p, b, traceCaller())
			} else if r.traceRaw {
				_, _ = fmt.Fprintf(f, "%d F %d -> %d raw=%d\n", r.tracePos, p, b, raw)
			} else {
				_, _ = fmt.Fprintf(f, "%d F %d -> %d\n", r.tracePos, p, b)
			}
			_ = f.Close()
		}
	}
	return ok
}

// traceCaller discovers the name of the function that initiated the RNG call.
// It walks the stack, filtering out internal RNG wrapper functions, to map the
// Go execution path back to its C++ conceptual equivalent.
func traceCaller() string {
	var pcs [12]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		fr, more := frames.Next()
		name := fr.Function
		if name != "" && name != "csmith/pkg/csmith.(*rng).upto" && name != "csmith/pkg/csmith.(*rng).uptoWithFilter" && name != "csmith/pkg/csmith.(*rng).flipcoin" {
			return name
		}
		if !more {
			break
		}
	}
	return "unknown"
}
