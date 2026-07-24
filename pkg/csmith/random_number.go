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
	return makeRndNumGeneratorWithOpts(kind, seed, sessOpts(s))
}

// makeRndNumGeneratorWithOpts is the factory used under session ownership.
func makeRndNumGeneratorWithOpts(kind RngKind, seed uint64, opts Options) *Rng {
	switch kind {
	case RngKindDefault:
		return NewRng(seed)
	case RngKindDFS:
		return makeDFSRndNumGeneratorOpts(seed, opts)
	default:
		sessNoteError(nil, ErrGeneric)
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
		g := makeRndNumGeneratorWithOpts(kind, seed, s.Opts)
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
		g = makeRndNumGeneratorWithOpts(kind, rn.seed, s.Opts)
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
		g = makeRndNumGeneratorWithOpts(kind, rn.seed, s.Opts)
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
	if rn == nil || rn.curr == nil {
		sessNoteError(nil, ErrGeneric)
		return name
	}
	if rn.curr.kind == RngKindDFS {
		return rn.curr.GetPrefixedNameDFS(name)
	}
	return GetPrefixedNameDefault(name)
}

// TraceDepth mirrors RandomNumber::trace_depth.
func (rn *RandomNumber) TraceDepth() string {
	if rn == nil || rn.curr == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	return rn.curr.TraceDepth()
}

// GetSequence mirrors RandomNumber::get_sequence.
func (rn *RandomNumber) GetSequence() string {
	if rn == nil || rn.curr == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	return rn.curr.GetSequence()
}

// RndUpto mirrors RandomNumber::rnd_upto.
func (rn *RandomNumber) RndUpto(n uint32, f Filter) uint32 {
	if rn == nil || rn.curr == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	return rn.curr.RndUptoFilter(n, f)
}

// RndFlipcoin mirrors RandomNumber::rnd_flipcoin.
func (rn *RandomNumber) RndFlipcoin(p uint32, f Filter) bool {
	if rn == nil || rn.curr == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	return rn.curr.RndFlipcoinFilter(p, f)
}

// RandomHexDigits mirrors RandomNumber::RandomHexDigits.
func (rn *RandomNumber) RandomHexDigits(num int) string {
	if rn == nil || rn.curr == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	return rn.curr.RandomHexDigits(num)
}

// RandomDigits mirrors RandomNumber::RandomDigits.
func (rn *RandomNumber) RandomDigits(num int) string {
	if rn == nil || rn.curr == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	return rn.curr.RandomDigits(num)
}

// Kind returns the active generator kind (Go helper for tests / Switch).
func (rn *RandomNumber) Kind() RngKind {
	if rn == nil {
		sessNoteError(nil, ErrGeneric)
		return RngKindDefault
	}
	return rn.currKind
}
