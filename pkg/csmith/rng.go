// Upstream: AbsRndNumGenerator.cpp (seedrand/genrand via srand48/lrand48),
// DefaultRndNumGenerator.cpp (rnd_upto, rnd_flipcoin, RandomHexDigits, RandomDigits),
// random.cpp (rnd_upto / rnd_flipcoin wrappers).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934 (csmith 2.4.0 self-report).
package csmith

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// Rng is AbsRndNumGenerator concrete state for Default or DFS mode.
// Default: DefaultRndNumGenerator + Abs genrand.
// DFS: DFSRndNumGenerator (dfs engine) + Abs genrand for hex/digits.
type Rng struct {
	state     uint64
	randDepth uint64

	// kind mirrors AbsRndNumGenerator::kind (Default vs DFS).
	kind RngKind

	// traceString mirrors DefaultRndNumGenerator / DFSRndNumGenerator::trace_string_.
	traceString string

	trace     bool
	traceSite bool
	traceTo   io.Writer

	// dfs holds DFSRndNumGenerator search state; nil for Default kind.
	dfs *dfsEngine
}

// NewRng seeds like AbsRndNumGenerator::seedrand → srand48(seed).
// srand48: X0 = (seed << 16) + 0x330E (48-bit).
func NewRng(seed uint64) *Rng {
	r := &Rng{
		state: ((seed << 16) + 0x330E) & lcgMask,
		kind:  RngKindDefault,
	}
	if os.Getenv("CSMITH_TRACE_RNG") != "" && os.Getenv("CSMITH_TRACE_RNG") != "0" {
		r.trace = true
		if os.Getenv("CSMITH_TRACE_RNG_SITE") != "" && os.Getenv("CSMITH_TRACE_RNG_SITE") != "0" {
			r.traceSite = true
		}
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
	return r.GenrandSess(testAmbientSession)
}

// GenrandSess is Genrand with explicit session residual sticky.
func (r *Rng) GenrandSess(s *Session) uint32 {
	// Rng always live for genrand; sticky incomplete no invent zero stream
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	r.state = (lcgA*r.state + lcgC) & lcgMask
	return uint32(r.state >> 17)
}

// RandDepth is DefaultRndNumGenerator::rand_depth_ (count of rnd_upto/rnd_flipcoin/hex digit steps).
// Nil Rng sticky 0 (no invent depth 0 soft-success past missing stream).
func (r *Rng) RandDepth() uint64 {
	return r.RandDepthSess(testAmbientSession)
}

// RandDepthSess is RandDepth with explicit session residual sticky.
func (r *Rng) RandDepthSess(s *Session) uint64 {
	// Rng always live; sticky incomplete no invent depth 0
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	return r.randDepth
}

// RndUpto returns v in [0, n). n must be > 0.
// DefaultRndNumGenerator::rnd_upto (no Filter* → no reject loop).
func (r *Rng) RndUpto(n uint32) uint32 {
	return r.RndUptoFilterSess(testAmbientSession, n, nil)
}

// RndUptoSess is RndUpto with explicit session residual sticky.
func (r *Rng) RndUptoSess(s *Session, n uint32) uint32 {
	return r.RndUptoFilterSess(s, n, nil)
}

// RndUptoFilter is DefaultRndNumGenerator::rnd_upto with optional Filter.
// On reject: re-genrand, keep rand_depth_ as local_depth+1 (does not double-count tries).
// Nil Rng sticky 0 (no invent fixed zero draw). n==0 is undefined in C++ (raw % n);
// non-sticky return 0 without inventing domain (soft re-pick: empty half/list lengths
// must not sticky-poison factories).
func (r *Rng) RndUptoFilter(n uint32, f Filter) uint32 {
	return r.RndUptoFilterSess(testAmbientSession, n, f)
}

// RndUptoFilterSess is RndUptoFilter with explicit session residual sticky.
func (r *Rng) RndUptoFilterSess(s *Session, n uint32, f Filter) uint32 {
	// Rng always live for rnd_upto; sticky incomplete no invent fixed 0 draw
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	// n==0 non-sticky soft empty domain (no invent %0 / poison factories)
	if n == 0 {
		return 0
	}
	// DFSRndNumGenerator::rnd_upto → random_choice (not genrand %)
	if r.kind == RngKindDFS {
		return r.dfsRndUptoSess(s, n, f)
	}
	raw := r.GenrandSess(s)
	v := raw % n
	localDepth := r.randDepth
	r.randDepth++
	tries := uint32(0)
	if f != nil {
		filterAt := func(x uint32) bool {
			switch ff := f.(type) {
			case *VectorFilter:
				return ff.FilterSess(s, x)
			case *ProbabilityFilter:
				return ff.FilterSess(s, x)
			default:
				return f.Filter(x)
			}
		}
		for filterAt(v) {
			// residual ERROR sticky — no invent soft-retry filter past hard IR residual hole
			// (e.g. IsVolatileStructUnion field-Type residual soft-rejects then hangs forever)
			if sessHasError(s) {
				return 0
			}
			// DefaultRndNumGenerator.cpp: roll back rand_depth_ to local_depth+1
			r.randDepth = localDepth + 1
			raw = r.GenrandSess(s)
			v = raw % n
			tries++
		}
		// residual ERROR sticky — no invent accept candidate past residual filter true path
		if sessHasError(s) {
			return 0
		}
	}
	if r.trace && r.traceTo != nil {
		site := ""
		if r.traceSite {
			if _, file, line, ok := runtime.Caller(2); ok {
				// skip RndUpto → RndUptoFilter → real caller when depth allows
				site = fmt.Sprintf(" @%s:%d", filepath.Base(file), line)
			}
			if _, file, line, ok := runtime.Caller(1); ok && site == "" {
				site = fmt.Sprintf(" @%s:%d", filepath.Base(file), line)
			}
		}
		_, _ = fmt.Fprintf(r.traceTo, "U depth=%d n=%d v=%d tries=%d raw=%d%s\n", localDepth, n, v, tries, raw, site)
	}
	return v
}

// RndFlipcoin returns true with probability p% (p clamped to 100).
// DefaultRndNumGenerator::rnd_flipcoin.
func (r *Rng) RndFlipcoin(p uint32) bool {
	return r.RndFlipcoinFilterSess(testAmbientSession, p, nil)
}

// RndFlipcoinSess is RndFlipcoin with explicit session residual sticky.
func (r *Rng) RndFlipcoinSess(s *Session, p uint32) bool {
	return r.RndFlipcoinFilterSess(s, p, nil)
}

// RndFlipcoinFilter is DefaultRndNumGenerator::rnd_flipcoin with optional Filter.
// If filter rejects 0 → true without genrand; rejects 1 → false without genrand.
// Nil Rng sticky false (no invent fixed tails / soft re-pick without RNG).
func (r *Rng) RndFlipcoinFilter(p uint32, f Filter) bool {
	return r.RndFlipcoinFilterSess(testAmbientSession, p, f)
}

// RndFlipcoinFilterSess is RndFlipcoinFilter with explicit session residual sticky.
func (r *Rng) RndFlipcoinFilterSess(s *Session, p uint32, f Filter) bool {
	// Rng always live for flipcoin; sticky incomplete no invent fixed false
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if p > 100 {
		p = 100
	}
	// DFSRndNumGenerator::rnd_flipcoin → random_choice(2) with invalid list
	if r.kind == RngKindDFS {
		return r.dfsRndFlipcoinSess(s, p, f)
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
	raw := r.GenrandSess(s)
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
	return r.RandomHexDigitsSess(testAmbientSession, num)
}

// RandomHexDigitsSess is RandomHexDigits with explicit session residual sticky.
func (r *Rng) RandomHexDigitsSess(s *Session, num int) string {
	// AbsRndNumGenerator always has live RNG; sticky no invent empty hex without it
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if num <= 0 {
		return ""
	}
	b := make([]byte, 0, num)
	for i := 0; i < num; i++ {
		x := r.GenrandSess(s) % 16
		b = append(b, HexAlphabet[x])
		// DefaultRndNumGenerator increments rand_depth_; Abs/DFS does not.
		if r.kind != RngKindDFS {
			r.randDepth++
		}
	}
	return string(b)
}

// RandomDigits is DefaultRndNumGenerator::RandomDigits when CGOptions::is_random().
// DFS uses AbsRndNumGenerator::RandomDigits (no rand_depth_ bump).
func (r *Rng) RandomDigits(num int) string {
	return r.RandomDigitsSess(testAmbientSession, num)
}

// RandomDigitsSess is RandomDigits with explicit session residual sticky.
func (r *Rng) RandomDigitsSess(s *Session, num int) string {
	// AbsRndNumGenerator always has live RNG; sticky no invent empty digits without it
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if num <= 0 {
		return ""
	}
	b := make([]byte, 0, num)
	for i := 0; i < num; i++ {
		x := r.GenrandSess(s) % 10
		b = append(b, DecAlphabet[x])
		if r.kind != RngKindDFS {
			r.randDepth++
		}
	}
	return string(b)
}

// Kind is AbsRndNumGenerator::kind — Default or DFS.
func (r *Rng) Kind() RngKind {
	return r.KindSess(testAmbientSession)
}

// KindSess is Kind with explicit session residual sticky.
func (r *Rng) KindSess(s *Session) RngKind {
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return RngKindDefault
	}
	return r.kind
}

// GetPrefixedNameDefault is DefaultRndNumGenerator::get_prefixed_name — identity.
// DefaultRndNumGenerator.cpp:105–107.
func GetPrefixedNameDefault(name string) string { return name }

// TraceDepth is DefaultRndNumGenerator::trace_depth (where-string accumulator).
// Random-mode default does not append where labels unless callers use where;
// Go Rng keeps an empty string unless extended.
func (r *Rng) TraceDepth() string {
	return r.TraceDepthSess(testAmbientSession)
}

// TraceDepthSess is TraceDepth with explicit session residual sticky.
func (r *Rng) TraceDepthSess(s *Session) string {
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return r.traceString
}

// GetSequence is DefaultRndNumGenerator::get_sequence / DFSRndNumGenerator::get_sequence.
// Default: sequence bookkeeping no-op → "".
// DFS: LinearSequence map joined by sep (empty sticky "").
func (r *Rng) GetSequence() string {
	return r.GetSequenceSess(testAmbientSession)
}

// GetSequenceSess is GetSequence with explicit session residual sticky.
func (r *Rng) GetSequenceSess(s *Session) string {
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if r.kind == RngKindDFS {
		return r.dfsSequenceStringSess(s)
	}
	return ""
}

// SetRandDepth is DefaultRndNumGenerator::set_rand_depth.
func (r *Rng) SetRandDepth(depth uint64) {
	r.SetRandDepthSess(testAmbientSession, depth)
}

// SetRandDepthSess is SetRandDepth with explicit session residual sticky.
func (r *Rng) SetRandDepthSess(s *Session, depth uint64) {
	if r == nil {
		sessNoteError(s, ErrGeneric)
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

// --- random.cpp session wrappers (RandomNumber::GetInstance → bag Rng) ---

// ProcessRndUptoSess mirrors random.cpp::rnd_upto on an explicit session bag.
// random.cpp:67–71. Nil bag RNG sticky 0.
func ProcessRndUptoSess(s *Session, n uint32, f Filter) uint32 {
	r := ProcessRngSess(s)
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	return r.RndUptoFilterSess(s, n, f)
}

// ProcessRndFlipcoinSess mirrors random.cpp::rnd_flipcoin on an explicit bag.
// random.cpp:73–77.
func ProcessRndFlipcoinSess(s *Session, p uint32, f Filter) bool {
	r := ProcessRngSess(s)
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return r.RndFlipcoinFilterSess(s, p, f)
}

// ProcessRandomHexDigitsSess mirrors random.cpp::RandomHexDigits on an explicit bag.
// random.cpp:57–60.
func ProcessRandomHexDigitsSess(s *Session, num int) string {
	r := ProcessRngSess(s)
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return r.RandomHexDigitsSess(s, num)
}

// ProcessRandomDigitsSess mirrors random.cpp::RandomDigits on an explicit bag.
// random.cpp:62–65.
func ProcessRandomDigitsSess(s *Session, num int) string {
	r := ProcessRngSess(s)
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return r.RandomDigitsSess(s, num)
}

// ProcessTraceDepthSess mirrors random.cpp::trace_depth on an explicit bag.
// random.cpp:132–135.
func ProcessTraceDepthSess(s *Session) string {
	r := ProcessRngSess(s)
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return r.TraceDepthSess(s)
}

// ProcessGetSequenceSess mirrors random.cpp::get_sequence on an explicit bag.
// random.cpp:137–140.
func ProcessGetSequenceSess(s *Session) string {
	r := ProcessRngSess(s)
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return r.GetSequenceSess(s)
}

// PureRndUpto mirrors pure_rnd_upto.
// random.cpp:104–117 — n==0 → 0; !is_random switches to Default generator temporarily.
func PureRndUpto(n uint32, f Filter) uint32 {
	return PureRndUptoSess(testAmbientSession, n, f)
}

// PureRndUptoSess is PureRndUpto on an explicit session bag.
func PureRndUptoSess(s *Session, n uint32, f Filter) uint32 {
	if n == 0 {
		return 0
	}
	if sessOpts(s).IsRandom() {
		return ProcessRndUptoSess(s, n, f)
	}
	old := SwitchRndNumGeneratorSess(s, RngKindDefault)
	rv := ProcessRndUptoSess(s, n, f)
	_ = SwitchRndNumGeneratorSess(s, old)
	return rv
}

// PureRndFlipcoin mirrors pure_rnd_flipcoin.
// random.cpp:119–130.
func PureRndFlipcoin(p uint32, f Filter) bool {
	return PureRndFlipcoinSess(testAmbientSession, p, f)
}

// PureRndFlipcoinSess is PureRndFlipcoin on an explicit session bag.
func PureRndFlipcoinSess(s *Session, p uint32, f Filter) bool {
	if sessOpts(s).IsRandom() {
		return ProcessRndFlipcoinSess(s, p, f)
	}
	old := SwitchRndNumGeneratorSess(s, RngKindDefault)
	rv := ProcessRndFlipcoinSess(s, p, f)
	_ = SwitchRndNumGeneratorSess(s, old)
	return rv
}

// PureRandomHexDigits mirrors PureRandomHexDigits.
// random.cpp:79–89.
func PureRandomHexDigits(num int) string {
	return PureRandomHexDigitsSess(testAmbientSession, num)
}

// PureRandomHexDigitsSess is PureRandomHexDigits on an explicit session bag.
func PureRandomHexDigitsSess(s *Session, num int) string {
	if sessOpts(s).IsRandom() {
		return ProcessRandomHexDigitsSess(s, num)
	}
	old := SwitchRndNumGeneratorSess(s, RngKindDefault)
	rv := ProcessRandomHexDigitsSess(s, num)
	_ = SwitchRndNumGeneratorSess(s, old)
	return rv
}

// PureRandomDigits mirrors PureRandomDigits.
// random.cpp:91–102.
func PureRandomDigits(num int) string {
	return PureRandomDigitsSess(testAmbientSession, num)
}

// PureRandomDigitsSess is PureRandomDigits on an explicit session bag.
func PureRandomDigitsSess(s *Session, num int) string {
	if sessOpts(s).IsRandom() {
		return ProcessRandomDigitsSess(s, num)
	}
	old := SwitchRndNumGeneratorSess(s, RngKindDefault)
	rv := ProcessRandomDigitsSess(s, num)
	_ = SwitchRndNumGeneratorSess(s, old)
	return rv
}

// legacy names used by older call sites if any remain during transition.
func newRNG(seed uint64) *Rng { return NewRng(seed) }
