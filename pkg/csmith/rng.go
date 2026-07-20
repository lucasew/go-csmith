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

// AbsRndNumGenerator.cpp:50–52 — alphabet tables (get_hex1 / get_dec1).
const (
	// HexAlphabet is AbsRndNumGenerator::hex1 / get_hex1().
	HexAlphabet = "0123456789ABCDEF"
	// DecAlphabet is AbsRndNumGenerator::dec1 / get_dec1().
	DecAlphabet = "0123456789"
)

// RngKind mirrors RNDNUM_GENERATOR (AbsRndNumGenerator.h).
type RngKind int

const (
	// RngKindDefault is RNDNUM_GENERATOR::rDefaultRndNumGenerator.
	RngKindDefault RngKind = 0
	// RngKindDFS is RNDNUM_GENERATOR::rDFSRndNumGenerator.
	RngKindDFS RngKind = 1
)

// RngKindCount is AbsRndNumGenerator::count() → MAX_RNDNUM_GENERATOR.
const RngKindCount = int(RngKindDFS) + 1

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

	// traceString mirrors DefaultRndNumGenerator::trace_string_ (where labels).
	traceString string

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
// Nil Rng sticky 0 (no invent fixed zero stream / soft re-pick without RNG).
func (r *Rng) Genrand() uint32 {
	// Rng always live for genrand; sticky incomplete no invent zero stream
	if r == nil {
		SetError(ErrGeneric)
		return 0
	}
	r.state = (lcgA*r.state + lcgC) & lcgMask
	return uint32(r.state >> 17)
}

// RandDepth is DefaultRndNumGenerator::rand_depth_ (count of rnd_upto/rnd_flipcoin/hex digit steps).
// Nil Rng sticky 0 (no invent depth 0 soft-success past missing stream).
func (r *Rng) RandDepth() uint64 {
	// Rng always live; sticky incomplete no invent depth 0
	if r == nil {
		SetError(ErrGeneric)
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
// Nil Rng sticky 0 (no invent fixed zero draw). n==0 is undefined in C++ (raw % n);
// non-sticky return 0 without inventing domain (soft re-pick: empty half/list lengths
// must not sticky-poison factories).
func (r *Rng) RndUptoFilter(n uint32, f Filter) uint32 {
	// Rng always live for rnd_upto; sticky incomplete no invent fixed 0 draw
	if r == nil {
		SetError(ErrGeneric)
		return 0
	}
	// n==0 non-sticky soft empty domain (no invent %0 / poison factories)
	if n == 0 {
		return 0
	}
	raw := r.Genrand()
	v := raw % n
	localDepth := r.randDepth
	r.randDepth++
	tries := uint32(0)
	if f != nil {
		for f.Filter(v) {
			// residual ERROR sticky — no invent soft-retry filter past hard IR residual hole
			// (e.g. IsVolatileStructUnion field-Type residual soft-rejects then hangs forever)
			if HasError() {
				return 0
			}
			// DefaultRndNumGenerator.cpp: roll back rand_depth_ to local_depth+1
			r.randDepth = localDepth + 1
			raw = r.Genrand()
			v = raw % n
			tries++
		}
		// residual ERROR sticky — no invent accept candidate past residual filter true path
		if HasError() {
			return 0
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
// Nil Rng sticky false (no invent fixed tails / soft re-pick without RNG).
func (r *Rng) RndFlipcoinFilter(p uint32, f Filter) bool {
	// Rng always live for flipcoin; sticky incomplete no invent fixed false
	if r == nil {
		SetError(ErrGeneric)
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
// AbsRndNumGenerator.cpp:50 — hex1 = "0123456789ABCDEF" (uppercase; no invent abcdef).
func (r *Rng) RandomHexDigits(num int) string {
	// AbsRndNumGenerator always has live RNG; sticky no invent empty hex without it
	if r == nil {
		SetError(ErrGeneric)
		return ""
	}
	if num <= 0 {
		return ""
	}
	b := make([]byte, 0, num)
	for i := 0; i < num; i++ {
		x := r.Genrand() % 16
		b = append(b, HexAlphabet[x])
		r.randDepth++
	}
	return string(b)
}

// RandomDigits is DefaultRndNumGenerator::RandomDigits when CGOptions::is_random().
func (r *Rng) RandomDigits(num int) string {
	// AbsRndNumGenerator always has live RNG; sticky no invent empty digits without it
	if r == nil {
		SetError(ErrGeneric)
		return ""
	}
	if num <= 0 {
		return ""
	}
	b := make([]byte, 0, num)
	for i := 0; i < num; i++ {
		x := r.Genrand() % 10
		b = append(b, DecAlphabet[x])
		r.randDepth++
	}
	return string(b)
}

// Kind is DefaultRndNumGenerator::kind → rDefaultRndNumGenerator.
func (r *Rng) Kind() RngKind {
	if r == nil {
		SetError(ErrGeneric)
		return RngKindDefault
	}
	return RngKindDefault
}

// GetPrefixedNameDefault is DefaultRndNumGenerator::get_prefixed_name — identity.
// DefaultRndNumGenerator.cpp:105–107.
func GetPrefixedNameDefault(name string) string { return name }

// TraceDepth is DefaultRndNumGenerator::trace_depth (where-string accumulator).
// Random-mode default does not append where labels unless callers use where;
// Go Rng keeps an empty string unless extended.
func (r *Rng) TraceDepth() string {
	if r == nil {
		SetError(ErrGeneric)
		return ""
	}
	return r.traceString
}

// GetSequence is DefaultRndNumGenerator::get_sequence.
// Sequence bookkeeping is a no-op in default mode (add_number empty); returns "".
func (r *Rng) GetSequence() string {
	if r == nil {
		SetError(ErrGeneric)
		return ""
	}
	return ""
}

// SetRandDepth is DefaultRndNumGenerator::set_rand_depth.
func (r *Rng) SetRandDepth(depth uint64) {
	if r == nil {
		SetError(ErrGeneric)
		return
	}
	r.randDepth = depth
}

// filterFunc adapts a function to Filter.
type filterFunc func(uint32) bool

func (f filterFunc) Filter(v uint32) bool { return f(v) }

// RejectEQ rejects a single value (test/helper; C++ Filter subclasses vary).
func RejectEQ(bad uint32) Filter {
	return filterFunc(func(v uint32) bool { return v == bad })
}

// --- random.cpp process wrappers (RandomNumber::GetInstance → ProcessRng) ---

// ProcessRndUpto mirrors random.cpp::rnd_upto → process DefaultRndNumGenerator.
// random.cpp:67–71. Nil process RNG sticky 0.
func ProcessRndUpto(n uint32, f Filter) uint32 {
	return ProcessRng().RndUptoFilter(n, f)
}

// ProcessRndFlipcoin mirrors random.cpp::rnd_flipcoin.
// random.cpp:73–77.
func ProcessRndFlipcoin(p uint32, f Filter) bool {
	return ProcessRng().RndFlipcoinFilter(p, f)
}

// ProcessRandomHexDigits mirrors random.cpp::RandomHexDigits.
// random.cpp:57–60.
func ProcessRandomHexDigits(num int) string {
	return ProcessRng().RandomHexDigits(num)
}

// ProcessRandomDigits mirrors random.cpp::RandomDigits.
// random.cpp:62–65.
func ProcessRandomDigits(num int) string {
	return ProcessRng().RandomDigits(num)
}

// ProcessTraceDepth mirrors random.cpp::trace_depth.
// random.cpp:132–135.
func ProcessTraceDepth() string {
	return ProcessRng().TraceDepth()
}

// ProcessGetSequence mirrors random.cpp::get_sequence.
// random.cpp:137–140.
func ProcessGetSequence() string {
	return ProcessRng().GetSequence()
}

// PureRndUpto mirrors pure_rnd_upto.
// random.cpp:104–117 — n==0 → 0; random mode == rnd_upto; DFS switches generator (not ported).
func PureRndUpto(n uint32, f Filter) uint32 {
	if n == 0 {
		return 0
	}
	// CGOptions::is_random() — non-random switches to DefaultRndNumGenerator.
	// Go only has default RNG; pure path is identity with ProcessRndUpto.
	return ProcessRndUpto(n, f)
}

// PureRndFlipcoin mirrors pure_rnd_flipcoin.
// random.cpp:119–130.
func PureRndFlipcoin(p uint32, f Filter) bool {
	return ProcessRndFlipcoin(p, f)
}

// PureRandomHexDigits mirrors PureRandomHexDigits.
// random.cpp:79–89.
func PureRandomHexDigits(num int) string {
	return ProcessRandomHexDigits(num)
}

// PureRandomDigits mirrors PureRandomDigits.
// random.cpp:91–102.
func PureRandomDigits(num int) string {
	return ProcessRandomDigits(num)
}

// legacy names used by older call sites if any remain during transition.
func newRNG(seed uint64) *Rng { return NewRng(seed) }
