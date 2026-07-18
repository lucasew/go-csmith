// Upstream: AbsRndNumGenerator.cpp (seedrand/genrand via srand48/lrand48),
// DefaultRndNumGenerator.cpp (rnd_upto, rnd_flipcoin, RandomHexDigits, RandomDigits),
// random.cpp (rnd_upto / rnd_flipcoin wrappers).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934 (csmith 2.4.0 self-report).
package csmith

import (
	"fmt"
	"io"
	"os"
)

// glibc srand48/lrand48 LCG parameters (AbsRndNumGenerator::genrand → lrand48).
const (
	lcgA    uint64 = 0x5DEECE66D
	lcgC    uint64 = 0xB
	lcgMask uint64 = (1 << 48) - 1
)

// Filter mirrors Filter::filter — true means reject this candidate.
// DefaultRndNumGenerator.cpp rnd_upto / rnd_flipcoin.
type Filter interface {
	Filter(v uint32) bool
}

// Rng is the default random-number generator used by Csmith (random-based mode).
// Mirrors DefaultRndNumGenerator + AbsRndNumGenerator genrand.
type Rng struct {
	state     uint64
	randDepth uint64

	trace   bool
	traceTo io.Writer
}

// NewRng seeds like AbsRndNumGenerator::seedrand → srand48(seed).
// srand48: X0 = (seed << 16) + 0x330E (48-bit).
func NewRng(seed uint64) *Rng {
	r := &Rng{
		state: ((seed << 16) + 0x330E) & lcgMask,
	}
	if os.Getenv("CSMITH_TRACE_RNG") != "" && os.Getenv("CSMITH_TRACE_RNG") != "0" {
		r.trace = true
		path := os.Getenv("CSMITH_TRACE_RNG_FILE")
		if path == "" {
			path = "/tmp/csmith-rng.trace"
		}
		f, err := os.Create(path)
		if err == nil {
			_, _ = fmt.Fprintf(f, "# seed=%d\n", seed)
			r.traceTo = f
		}
	}
	return r
}

// Genrand returns the next 31-bit value.
// AbsRndNumGenerator::genrand → lrand48: (X >> 17) after LCG step.
func (r *Rng) Genrand() uint32 {
	if r == nil {
		return 0
	}
	r.state = (lcgA*r.state + lcgC) & lcgMask
	return uint32(r.state >> 17)
}

// RandDepth is DefaultRndNumGenerator::rand_depth_ (count of rnd_upto/rnd_flipcoin/hex digit steps).
func (r *Rng) RandDepth() uint64 {
	if r == nil {
		return 0
	}
	return r.randDepth
}

// RndUpto returns v in [0, n). n must be > 0.
// DefaultRndNumGenerator::rnd_upto (no Filter* → no reject loop).
func (r *Rng) RndUpto(n uint32) uint32 {
	return r.RndUptoFilter(n, nil)
}

// RndUptoFilter is DefaultRndNumGenerator::rnd_upto with optional Filter.
// On reject: re-genrand, keep rand_depth_ as local_depth+1 (does not double-count tries).
func (r *Rng) RndUptoFilter(n uint32, f Filter) uint32 {
	if r == nil || n == 0 {
		return 0
	}
	raw := r.Genrand()
	v := raw % n
	localDepth := r.randDepth
	r.randDepth++
	tries := uint32(0)
	if f != nil {
		for f.Filter(v) {
			// DefaultRndNumGenerator.cpp: roll back rand_depth_ to local_depth+1
			r.randDepth = localDepth + 1
			raw = r.Genrand()
			v = raw % n
			tries++
		}
	}
	if r.trace && r.traceTo != nil {
		_, _ = fmt.Fprintf(r.traceTo, "U depth=%d n=%d v=%d tries=%d raw=%d\n", localDepth, n, v, tries, raw)
	}
	return v
}

// RndFlipcoin returns true with probability p% (p clamped to 100).
// DefaultRndNumGenerator::rnd_flipcoin.
func (r *Rng) RndFlipcoin(p uint32) bool {
	return r.RndFlipcoinFilter(p, nil)
}

// RndFlipcoinFilter is DefaultRndNumGenerator::rnd_flipcoin with optional Filter.
// If filter rejects 0 → true without genrand; rejects 1 → false without genrand.
func (r *Rng) RndFlipcoinFilter(p uint32, f Filter) bool {
	if r == nil {
		return false
	}
	if p > 100 {
		p = 100
	}
	localDepth := r.randDepth
	r.randDepth++
	if f != nil {
		if f.Filter(0) {
			if r.trace && r.traceTo != nil {
				_, _ = fmt.Fprintf(r.traceTo, "F depth=%d p=%d v=1\n", localDepth, p)
			}
			return true
		}
		if f.Filter(1) {
			if r.trace && r.traceTo != nil {
				_, _ = fmt.Fprintf(r.traceTo, "F depth=%d p=%d v=0\n", localDepth, p)
			}
			return false
		}
	}
	raw := r.Genrand()
	ok := (raw % 100) < p
	if r.trace && r.traceTo != nil {
		b := 0
		if ok {
			b = 1
		}
		_, _ = fmt.Fprintf(r.traceTo, "F depth=%d p=%d v=%d\n", localDepth, p, b)
	}
	return ok
}

// RandomHexDigits is DefaultRndNumGenerator::RandomHexDigits when CGOptions::is_random().
// Each digit: genrand()%16; increments rand_depth_ per digit.
func (r *Rng) RandomHexDigits(num int) string {
	if r == nil || num <= 0 {
		return ""
	}
	const hex1 = "0123456789abcdef"
	b := make([]byte, 0, num)
	for i := 0; i < num; i++ {
		x := r.Genrand() % 16
		b = append(b, hex1[x])
		r.randDepth++
	}
	return string(b)
}

// RandomDigits is DefaultRndNumGenerator::RandomDigits when CGOptions::is_random().
func (r *Rng) RandomDigits(num int) string {
	if r == nil || num <= 0 {
		return ""
	}
	const dec1 = "0123456789"
	b := make([]byte, 0, num)
	for i := 0; i < num; i++ {
		x := r.Genrand() % 10
		b = append(b, dec1[x])
		r.randDepth++
	}
	return string(b)
}

// filterFunc adapts a function to Filter.
type filterFunc func(uint32) bool

func (f filterFunc) Filter(v uint32) bool { return f(v) }

// RejectEQ rejects a single value (test/helper; C++ Filter subclasses vary).
func RejectEQ(bad uint32) Filter {
	return filterFunc(func(v uint32) bool { return v == bad })
}

// legacy names used by older call sites if any remain during transition.
func newRNG(seed uint64) *Rng { return NewRng(seed) }
