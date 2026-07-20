// Upstream: RandomNumber.h / RandomNumber.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// RandomNumber mirrors RandomNumber — process singleton bridge to AbsRndNumGenerator.
// RandomNumber.cpp:39–152.
type RandomNumber struct {
	seed       uint64
	generators map[RngKind]*Rng
	curr       *Rng
	currKind   RngKind
}

// processRN is RandomNumber::instance_.
// Guarded by processOptsMu (same lifetime as ProcessRng).
var processRN *RandomNumber

// MakeRndNumGenerator mirrors AbsRndNumGenerator::make_rndnum_generator.
// AbsRndNumGenerator.cpp:66–84 — Default → NewRng; DFS not ported (sticky nil).
func MakeRndNumGenerator(kind RngKind, seed uint64) *Rng {
	switch kind {
	case RngKindDefault:
		return NewRng(seed)
	case RngKindDFS:
		// DFSRndNumGenerator not ported on fair-rewrite spine yet.
		SetError(ErrGeneric)
		return nil
	default:
		SetError(ErrGeneric)
		return nil
	}
}

// CreateRandomNumberInstance mirrors RandomNumber::CreateInstance.
// RandomNumber.cpp:63–78.
func CreateRandomNumberInstance(kind RngKind, seed uint64) {
	processOptsMu.Lock()
	defer processOptsMu.Unlock()
	if processRN == nil {
		processRN = &RandomNumber{
			seed:       seed,
			generators: make(map[RngKind]*Rng),
		}
		g := MakeRndNumGenerator(kind, seed)
		if g == nil {
			// sticky already set; leave instance with nil curr (fail closed)
			return
		}
		processRN.generators[kind] = g
		processRN.curr = g
		processRN.currKind = kind
		processRng = g
		return
	}
	// Existing instance: switch to requested generator (must already exist).
	g := processRN.generators[kind]
	if g == nil {
		SetError(ErrGeneric)
		return
	}
	processRN.curr = g
	processRN.currKind = kind
	processRng = g
}

// GetRandomNumber mirrors RandomNumber::GetInstance.
// RandomNumber.cpp:80–83 — C++ asserts; nil is library fail-closed.
func GetRandomNumber() *RandomNumber {
	processOptsMu.RLock()
	defer processOptsMu.RUnlock()
	if processRN == nil {
		SetError(ErrGeneric)
		return nil
	}
	return processRN
}

// GetRndNumGenerator mirrors RandomNumber::GetRndNumGenerator.
// RandomNumber.cpp:85–88.
func GetRndNumGenerator() *Rng {
	processOptsMu.RLock()
	defer processOptsMu.RUnlock()
	if processRN == nil || processRN.curr == nil {
		SetError(ErrGeneric)
		return nil
	}
	return processRN.curr
}

// SwitchRndNumGenerator mirrors RandomNumber::SwitchRndNumGenerator.
// RandomNumber.cpp:95–110 — create missing generator from seed; return previous kind.
func SwitchRndNumGenerator(kind RngKind) RngKind {
	processOptsMu.Lock()
	defer processOptsMu.Unlock()
	if processRN == nil || processRN.curr == nil {
		SetError(ErrGeneric)
		return RngKindDefault
	}
	old := processRN.currKind
	g := processRN.generators[kind]
	if g == nil {
		g = MakeRndNumGenerator(kind, processRN.seed)
		if g == nil {
			return old
		}
		processRN.generators[kind] = g
	}
	processRN.curr = g
	processRN.currKind = kind
	processRng = g
	return old
}

// RandomNumberDoFinalization mirrors RandomNumber::doFinalization.
// RandomNumber.cpp:142–152 — drop generators and instance.
func RandomNumberDoFinalization() {
	processOptsMu.Lock()
	defer processOptsMu.Unlock()
	processRN = nil
	processRng = nil
}

// --- instance methods (RandomNumber.cpp:112–140) ---

// GetPrefixedName mirrors RandomNumber::get_prefixed_name → curr_generator_.
func (rn *RandomNumber) GetPrefixedName(name string) string {
	if rn == nil || rn.curr == nil {
		SetError(ErrGeneric)
		return name
	}
	return GetPrefixedNameDefault(name)
}

// TraceDepth mirrors RandomNumber::trace_depth.
func (rn *RandomNumber) TraceDepth() string {
	if rn == nil || rn.curr == nil {
		SetError(ErrGeneric)
		return ""
	}
	return rn.curr.TraceDepth()
}

// GetSequence mirrors RandomNumber::get_sequence.
func (rn *RandomNumber) GetSequence() string {
	if rn == nil || rn.curr == nil {
		SetError(ErrGeneric)
		return ""
	}
	return rn.curr.GetSequence()
}

// RndUpto mirrors RandomNumber::rnd_upto.
func (rn *RandomNumber) RndUpto(n uint32, f Filter) uint32 {
	if rn == nil || rn.curr == nil {
		SetError(ErrGeneric)
		return 0
	}
	return rn.curr.RndUptoFilter(n, f)
}

// RndFlipcoin mirrors RandomNumber::rnd_flipcoin.
func (rn *RandomNumber) RndFlipcoin(p uint32, f Filter) bool {
	if rn == nil || rn.curr == nil {
		SetError(ErrGeneric)
		return false
	}
	return rn.curr.RndFlipcoinFilter(p, f)
}

// RandomHexDigits mirrors RandomNumber::RandomHexDigits.
func (rn *RandomNumber) RandomHexDigits(num int) string {
	if rn == nil || rn.curr == nil {
		SetError(ErrGeneric)
		return ""
	}
	return rn.curr.RandomHexDigits(num)
}

// RandomDigits mirrors RandomNumber::RandomDigits.
func (rn *RandomNumber) RandomDigits(num int) string {
	if rn == nil || rn.curr == nil {
		SetError(ErrGeneric)
		return ""
	}
	return rn.curr.RandomDigits(num)
}

// Kind returns the active generator kind (Go helper for tests / Switch).
func (rn *RandomNumber) Kind() RngKind {
	if rn == nil {
		SetError(ErrGeneric)
		return RngKindDefault
	}
	return rn.currKind
}
