// Upstream: Sequence.h / Sequence.cpp / LinearSequence.* / SequenceFactory.* / SequenceLineParser.h
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"strconv"
	"strings"
)

// LinearSequenceDefaultSep is LinearSequence::default_sep_char.
// LinearSequence.cpp:43.
const LinearSequenceDefaultSep = '_'

// LinearSequence mirrors LinearSequence — DFS choice sequence map.
// LinearSequence.cpp:45–81.
type LinearSequence struct {
	sep byte
	// seqMap mirrors seq_map_ (pos → value). Dense 0..n-1 in practice.
	seqMap map[int]int
}

// NewLinearSequence mirrors LinearSequence::LinearSequence(sep_char).
// LinearSequence.cpp:45–47.
func NewLinearSequence(sep byte) *LinearSequence {
	if sep == 0 {
		sep = LinearSequenceDefaultSep
	}
	return &LinearSequence{
		sep:    sep,
		seqMap: make(map[int]int),
	}
}

// InitSequence mirrors LinearSequence::init_sequence — no-op.
// LinearSequence.cpp:56.
func (s *LinearSequence) InitSequence() {}

// AddNumber mirrors LinearSequence::add_number.
// LinearSequence.cpp:58–60 — seq_map_[k] = v (bound ignored).
// LinearSequence always live; sticky incomplete no invent silent drop.
func (s *LinearSequence) AddNumber(v, bound, k int) {
	_ = bound
	if s == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if s.seqMap == nil {
		s.seqMap = make(map[int]int)
	}
	s.seqMap[k] = v
}

// GetNumber mirrors LinearSequence::get_number — always -1.
// LinearSequence.cpp:62.
func (s *LinearSequence) GetNumber(bound int) int {
	_ = bound
	if s == nil {
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	return -1
}

// GetNumberByPos mirrors LinearSequence::get_number_by_pos.
// LinearSequence.cpp:64–68 — map[pos]; C++ asserts rv >= 0.
// Missing key sticky -1 (no invent 0 for unset pos).
func (s *LinearSequence) GetNumberByPos(pos int) int {
	if s == nil || s.seqMap == nil {
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	rv, ok := s.seqMap[pos]
	if !ok {
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	// C++ assert(rv >= 0); negative stored value sticky
	if rv < 0 {
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	return rv
}

// Clear mirrors LinearSequence::clear.
// LinearSequence.cpp:70.
func (s *LinearSequence) Clear() {
	if s == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	s.seqMap = make(map[int]int)
}

// GetSequence mirrors LinearSequence::get_sequence.
// LinearSequence.cpp:72–79 — join map[0..size-1] with sep; empty map asserts in C++.
// Empty sticky "" (no invent bare sep shell).
func (s *LinearSequence) GetSequence() string {
	if s == nil || s.seqMap == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	n := len(s.seqMap)
	if n == 0 {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// require dense 0..n-1 keys (C++ iterates size()-1 via map.size())
	var b strings.Builder
	for i := 0; i < n; i++ {
		v, ok := s.seqMap[i]
		if !ok {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		if i > 0 {
			b.WriteByte(s.sep)
		}
		b.WriteString(strconv.Itoa(v))
	}
	return b.String()
}

// SequenceLength mirrors LinearSequence::sequence_length.
// LinearSequence.cpp:81.
func (s *LinearSequence) SequenceLength() int {
	if s == nil || s.seqMap == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	return len(s.seqMap)
}

// SepChar mirrors LinearSequence::get_sep_char.
func (s *LinearSequence) SepChar() byte {
	if s == nil {
		sessNoteError(nil, ErrGeneric)
		return LinearSequenceDefaultSep
	}
	return s.sep
}

// --- SequenceFactory ---

// currentSession().SequenceFactorySep mirrors SequenceFactory::current_sep_char_.

// currentSession().SequenceFactoryLive tracks sequences created via MakeSequence (destroy_sequences).

// MakeSequence mirrors SequenceFactory::make_sequence.
// SequenceFactory.cpp:44–52 — always LinearSequence with default_sep_char.
func MakeSequence() *LinearSequence {
	return MakeSequenceSess(nil)
}

// MakeSequenceSess is MakeSequence on an explicit session bag.
func MakeSequenceSess(s *Session) *LinearSequence {
	s = sessOrAmbient(s)
	seq := NewLinearSequence(LinearSequenceDefaultSep)
	s.SequenceFactorySep = LinearSequenceDefaultSep
	s.SequenceFactoryLive = append(s.SequenceFactoryLive, seq)
	return seq
}

// DestroySequences mirrors SequenceFactory::destroy_sequences.
// SequenceFactory.cpp:54–59.
func DestroySequences() {
	DestroySequencesSess(nil)
}

// DestroySequencesSess clears live sequences on an explicit session bag.
func DestroySequencesSess(s *Session) {
	sessOrAmbient(s).SequenceFactoryLive = nil
}

// CurrentSepChar mirrors SequenceFactory::current_sep_char.
func CurrentSepChar() byte { return CurrentSepCharSess(nil) }

// CurrentSepCharSess returns the separator on an explicit session bag.
func CurrentSepCharSess(s *Session) byte { return sessOrAmbient(s).SequenceFactorySep }

// --- SequenceLineParser ---

// ParseSequenceLine mirrors SequenceLineParser::parse_sequence.
// SequenceLineParser.h:124–142 — split on sep, str2int each token; fail if any < 0.
// Leading spaces stripped per token; empty input soft returns empty slice true
// only when no tokens needed — empty string yields one empty token → str2int -1 → false.
func ParseSequenceLine(seq string, sep byte) ([]int, bool) {
	if sep == 0 {
		sep = ','
	}
	var out []int
	for pos := 0; ; {
		for pos < len(seq) && seq[pos] == ' ' {
			pos++
		}
		start := pos
		end := strings.IndexByte(seq[pos:], sep)
		var token string
		if end < 0 {
			token = seq[start:]
			pos = len(seq)
		} else {
			token = seq[start : start+end]
			pos = start + end + 1
		}
		i, ok := sequenceStr2Int(token)
		if !ok || i < 0 {
			return nil, false
		}
		out = append(out, i)
		if end < 0 {
			break
		}
	}
	return out, true
}

// sequenceStr2Int mirrors SequenceLineParser::str2int.
// SequenceLineParser.h:146–152 — stream extract; fail → -1.
func sequenceStr2Int(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1, false
	}
	// C++ stringstream >> i starts -1; non-numeric leaves -1.
	v, err := strconv.Atoi(s)
	if err != nil {
		return -1, false
	}
	return v, true
}
