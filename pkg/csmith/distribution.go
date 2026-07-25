// Upstream: ProbabilityTable.h DistributionTable + Probabilities.cpp:1025–1052.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// DistributionTable mirrors DistributionTable (cumulative weight bag of keys).
type DistributionTable struct {
	keys    []int
	probs   []int
	maxProb int
}

// AddEntry mirrors DistributionTable::add_entry.
// Probabilities.cpp:1025–1029 — always push key/prob (including weight 0).
// Incomplete table sticky no-op (no invent soft grow past missing shell).
func (d *DistributionTable) AddEntry(key, prob int) {
	d.AddEntrySess(testAmbientSession, key, prob)
}

// AddEntrySess is AddEntry with explicit session residual sticky.
func (d *DistributionTable) AddEntrySess(s *Session, key, prob int) {
	// DistributionTable always live when building filters; sticky incomplete no invent
	if d == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	d.keys = append(d.keys, key)
	d.probs = append(d.probs, prob)
	d.maxProb += prob
}

// Max mirrors get_max.
// Incomplete table sticky 0 (no invent default domain soft-skip past hole).
func (d *DistributionTable) Max() int {
	return d.MaxSess(testAmbientSession)
}

// MaxSess is Max with explicit session residual sticky.
func (d *DistributionTable) MaxSess(s *Session) int {
	// DistributionTable always live; sticky incomplete no invent max 0 soft-skip
	if d == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	return d.maxProb
}

// KeyToProb mirrors DistributionTable::key_to_prob.
// Probabilities.cpp:1031–1038 — first matching key's weight; 0 if missing.
func (d *DistributionTable) KeyToProb(key int) int {
	return d.KeyToProbSess(testAmbientSession, key)
}

// KeyToProbSess is KeyToProb with explicit session residual sticky.
func (d *DistributionTable) KeyToProbSess(s *Session, key int) int {
	if d == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	for i, k := range d.keys {
		if k == key {
			return d.probs[i]
		}
	}
	return 0
}

// RndNumToKey mirrors rnd_num_to_key — walk cumulative probs.
// Incomplete table sticky -1 (no invent identity key soft-skip past hole).
// OOB rnd (not in [0,max)) returns -1; C++ asserts.
func (d *DistributionTable) RndNumToKey(rnd int) int {
	return d.RndNumToKeySess(testAmbientSession, rnd)
}

// RndNumToKeySess is RndNumToKey with explicit session residual sticky.
func (d *DistributionTable) RndNumToKeySess(s *Session, rnd int) int {
	// DistributionTable always live for lookup; sticky incomplete no invent -1 soft-skip
	if d == nil {
		sessNoteError(s, ErrGeneric)
		return -1
	}
	if rnd < 0 || rnd >= d.maxProb {
		return -1
	}
	for i, p := range d.probs {
		if rnd < p {
			return d.keys[i]
		}
		rnd -= p
	}
	return -1
}

// ThresholdTable mirrors ProbabilityTable used for unequal statement probs:
// keys are thresholds (15,30,…,100), values are eStatementType codes.
// ProbabilityTable::get_value finds first key > rnd.
type ThresholdTable struct {
	// sorted ascending by key
	keys   []int
	values []int
}

// Add mirrors sorted insert of (threshold, value).
// Incomplete table sticky no-op (no invent grow past missing table shell).
func (t *ThresholdTable) Add(key, value int) {
	t.AddSess(testAmbientSession, key, value)
}

// AddSess is Add with explicit session residual sticky.
func (t *ThresholdTable) AddSess(s *Session, key, value int) {
	// ThresholdTable always live when building stmt tables; sticky incomplete no invent
	if t == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if key <= 0 {
		return
	}
	i := 0
	for i < len(t.keys) && t.keys[i] <= key {
		i++
	}
	// insert at i
	t.keys = append(t.keys, 0)
	t.values = append(t.values, 0)
	copy(t.keys[i+1:], t.keys[i:])
	copy(t.values[i+1:], t.values[i:])
	t.keys[i] = key
	t.values[i] = value
}

// GetValue mirrors ProbabilityTable::get_value — first key > k.
// Incomplete table sticky -1 (no invent miss soft-success past missing table shell).
func (t *ThresholdTable) GetValue(k int) int {
	return t.GetValueSess(testAmbientSession, k)
}

// GetValueSess is GetValue with explicit session residual sticky.
func (t *ThresholdTable) GetValueSess(s *Session, k int) int {
	// ThresholdTable always live for draws; sticky incomplete no invent -1 soft-skip
	if t == nil {
		sessNoteError(s, ErrGeneric)
		return -1
	}
	for i, key := range t.keys {
		if key > k {
			return t.values[i]
		}
	}
	return -1
}
