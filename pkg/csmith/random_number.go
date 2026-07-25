// Upstream: RandomNumber.h / RandomNumber.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// RandomNumber mirrors RandomNumber — session singleton bridge to AbsRndNumGenerator.
// RandomNumber.cpp:39–152.
type RandomNumber struct {
	seed       uint64
	generators map[RngKind]*Rng
	curr       *Rng
	currKind   RngKind
}

// MakeRndNumGenerator mirrors AbsRndNumGenerator::make_rndnum_generator.
// AbsRndNumGenerator.cpp:66–84 — seedrand then Default or DFS factory.
func MakeRndNumGenerator(kind RngKind, seed uint64) *Rng {
	return MakeRndNumGeneratorSess(nil, kind, seed)
}

// MakeRndNumGeneratorSess is MakeRndNumGenerator using session Options (DFS).
func MakeRndNumGeneratorSess(s *Session, kind RngKind, seed uint64) *Rng {
	return makeRndNumGeneratorWithOptsSess(s, kind, seed, sessOpts(s))
}

// makeRndNumGeneratorWithOpts is the factory used under session ownership.
func makeRndNumGeneratorWithOpts(kind RngKind, seed uint64, opts Options) *Rng {
	return makeRndNumGeneratorWithOptsSess(nil, kind, seed, opts)
}

// makeRndNumGeneratorWithOptsSess is the factory on an explicit session bag.
func makeRndNumGeneratorWithOptsSess(s *Session, kind RngKind, seed uint64, opts Options) *Rng {
	switch kind {
	case RngKindDefault:
		return NewRng(seed)
	case RngKindDFS:
		return makeDFSRndNumGeneratorOptsSess(s, seed, opts)
	default:
		sessNoteError(s, ErrGeneric)
		return nil
	}
}

// CreateRandomNumberInstance mirrors RandomNumber::CreateInstance.
// RandomNumber.cpp:63–78. Session-specific (no process mutex).
func CreateRandomNumberInstance(kind RngKind, seed uint64) {
	CreateRandomNumberInstanceSess(nil, kind, seed)
}

// CreateRandomNumberInstanceSess is CreateInstance on an explicit session bag.
func CreateRandomNumberInstanceSess(s *Session, kind RngKind, seed uint64) {
	s = sessOrAmbient(s)
	if s.RandomNumber == nil {
		s.RandomNumber = &RandomNumber{
			seed:       seed,
			generators: make(map[RngKind]*Rng),
		}
		g := makeRndNumGeneratorWithOptsSess(s, kind, seed, s.Opts)
		if g == nil {
			return
		}
		s.RandomNumber.generators[kind] = g
		s.RandomNumber.curr = g
		s.RandomNumber.currKind = kind
		s.Rng = g
		return
	}
	rn := s.RandomNumber
	g := rn.generators[kind]
	if g == nil {
		g = makeRndNumGeneratorWithOptsSess(s, kind, rn.seed, s.Opts)
		if g == nil {
			return
		}
		rn.generators[kind] = g
	}
	rn.curr = g
	rn.currKind = kind
	s.Rng = g
}

// GetRandomNumber mirrors RandomNumber::GetInstance.
// RandomNumber.cpp:80–83 — C++ asserts; nil is library fail-closed.
func GetRandomNumber() *RandomNumber {
	return GetRandomNumberSess(nil)
}

// GetRandomNumberSess returns RandomNumber on an explicit session bag.
func GetRandomNumberSess(s *Session) *RandomNumber {
	rn := sessOrAmbient(s).RandomNumber
	if rn == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return rn
}

// GetRndNumGenerator mirrors RandomNumber::GetRndNumGenerator.
// RandomNumber.cpp:85–88.
func GetRndNumGenerator() *Rng {
	return GetRndNumGeneratorSess(nil)
}

// GetRndNumGeneratorSess returns the current AbsRng on an explicit session bag.
func GetRndNumGeneratorSess(s *Session) *Rng {
	rn := sessOrAmbient(s).RandomNumber
	if rn == nil || rn.curr == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return rn.curr
}

// SwitchRndNumGenerator mirrors RandomNumber::SwitchRndNumGenerator.
// RandomNumber.cpp:95–110 — create missing generator from seed; return previous kind.
func SwitchRndNumGenerator(kind RngKind) RngKind {
	return SwitchRndNumGeneratorSess(nil, kind)
}

// SwitchRndNumGeneratorSess switches generators on an explicit session bag.
func SwitchRndNumGeneratorSess(s *Session, kind RngKind) RngKind {
	s = sessOrAmbient(s)
	rn := s.RandomNumber
	if rn == nil || rn.curr == nil {
		sessNoteError(s, ErrGeneric)
		return RngKindDefault
	}
	old := rn.currKind
	g := rn.generators[kind]
	if g == nil {
		g = makeRndNumGeneratorWithOptsSess(s, kind, rn.seed, s.Opts)
		if g == nil {
			return old
		}
		rn.generators[kind] = g
	}
	rn.curr = g
	rn.currKind = kind
	s.Rng = g
	return old
}

// RandomNumberDoFinalization mirrors RandomNumber::doFinalization.
// RandomNumber.cpp:142–152 — drop generators and instance.
// Also clears DFSRndNumGenerator::impl_ + SequenceFactory sequences.
func RandomNumberDoFinalization() {
	RandomNumberDoFinalizationSess(nil)
}

// RandomNumberDoFinalizationSess clears RandomNumber/DFS on an explicit session bag.
func RandomNumberDoFinalizationSess(s *Session) {
	s = sessOrAmbient(s)
	s.RandomNumber = nil
	s.Rng = nil
	clearDFSImplSess(s)
}

// --- instance methods (RandomNumber.cpp:112–140) ---

// GetPrefixedName mirrors RandomNumber::get_prefixed_name → curr_generator_.
func (rn *RandomNumber) GetPrefixedName(name string) string {
	return rn.GetPrefixedNameSess(nil, name)
}

// GetPrefixedNameSess is GetPrefixedName with explicit session residual sticky.
func (rn *RandomNumber) GetPrefixedNameSess(s *Session, name string) string {
	if rn == nil || rn.curr == nil {
		sessNoteError(s, ErrGeneric)
		return name
	}
	if rn.curr.kind == RngKindDFS {
		return rn.curr.GetPrefixedNameDFSSess(s, name)
	}
	return GetPrefixedNameDefault(name)
}

// TraceDepth mirrors RandomNumber::trace_depth.
func (rn *RandomNumber) TraceDepth() string {
	return rn.TraceDepthSess(nil)
}

// TraceDepthSess is TraceDepth with explicit session residual sticky.
func (rn *RandomNumber) TraceDepthSess(s *Session) string {
	if rn == nil || rn.curr == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return rn.curr.TraceDepthSess(s)
}

// GetSequence mirrors RandomNumber::get_sequence.
func (rn *RandomNumber) GetSequence() string {
	return rn.GetSequenceSess(nil)
}

// GetSequenceSess is GetSequence with explicit session residual sticky.
func (rn *RandomNumber) GetSequenceSess(s *Session) string {
	if rn == nil || rn.curr == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return rn.curr.GetSequenceSess(s)
}

// RndUpto mirrors RandomNumber::rnd_upto.
func (rn *RandomNumber) RndUpto(n uint32, f Filter) uint32 {
	return rn.RndUptoSess(nil, n, f)
}

// RndUptoSess is RndUpto with explicit session residual sticky.
func (rn *RandomNumber) RndUptoSess(s *Session, n uint32, f Filter) uint32 {
	if rn == nil || rn.curr == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	return rn.curr.RndUptoFilterSess(s, n, f)
}

// RndFlipcoin mirrors RandomNumber::rnd_flipcoin.
func (rn *RandomNumber) RndFlipcoin(p uint32, f Filter) bool {
	return rn.RndFlipcoinSess(nil, p, f)
}

// RndFlipcoinSess is RndFlipcoin with explicit session residual sticky.
func (rn *RandomNumber) RndFlipcoinSess(s *Session, p uint32, f Filter) bool {
	if rn == nil || rn.curr == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return rn.curr.RndFlipcoinFilterSess(s, p, f)
}

// RandomHexDigits mirrors RandomNumber::RandomHexDigits.
func (rn *RandomNumber) RandomHexDigits(num int) string {
	return rn.RandomHexDigitsSess(nil, num)
}

// RandomHexDigitsSess is RandomHexDigits with explicit session residual sticky.
func (rn *RandomNumber) RandomHexDigitsSess(s *Session, num int) string {
	if rn == nil || rn.curr == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return rn.curr.RandomHexDigitsSess(s, num)
}

// RandomDigits mirrors RandomNumber::RandomDigits.
func (rn *RandomNumber) RandomDigits(num int) string {
	return rn.RandomDigitsSess(nil, num)
}

// RandomDigitsSess is RandomDigits with explicit session residual sticky.
func (rn *RandomNumber) RandomDigitsSess(s *Session, num int) string {
	if rn == nil || rn.curr == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return rn.curr.RandomDigitsSess(s, num)
}

// Kind returns the active generator kind (Go helper for tests / Switch).
func (rn *RandomNumber) Kind() RngKind {
	return rn.KindSess(nil)
}

// KindSess is Kind with explicit session residual sticky.
func (rn *RandomNumber) KindSess(s *Session) RngKind {
	if rn == nil {
		sessNoteError(s, ErrGeneric)
		return RngKindDefault
	}
	return rn.currKind
}
