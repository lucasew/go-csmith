// Upstream: FactMgr.h / FactMgr.cpp (per-function DFA facts; light skeleton).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// FactMgr mirrors FactMgr for a function — global_facts + stm maps (stubs).
// Full point-to / union field analysis not ported; holds placeholders for call sites.
type FactMgr struct {
	// Func is the owning function (FactMgr.cpp constructor).
	Func *Function
	// GlobalFacts mirrors global_facts (vector of facts; opaque for now).
	GlobalFacts []any
}

// NewFactMgr constructs a FactMgr for f (FactMgr::FactMgr(Function*)).
func NewFactMgr(f *Function) *FactMgr {
	return &FactMgr{Func: f}
}

// FactMgrMap is Function::FMList session map (func → FactMgr).
type FactMgrMap struct {
	byFunc map[*Function]*FactMgr
}

// NewFactMgrMap creates an empty FMList.
func NewFactMgrMap() *FactMgrMap {
	return &FactMgrMap{byFunc: make(map[*Function]*FactMgr)}
}

// ForFunc returns (creating if needed) the FactMgr for f.
// get_fact_mgr_for_func.
func (m *FactMgrMap) ForFunc(f *Function) *FactMgr {
	if m == nil || f == nil {
		return nil
	}
	if m.byFunc == nil {
		m.byFunc = make(map[*Function]*FactMgr)
	}
	if fm, ok := m.byFunc[f]; ok {
		return fm
	}
	fm := NewFactMgr(f)
	m.byFunc[f] = fm
	return fm
}
