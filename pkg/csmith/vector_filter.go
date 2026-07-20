// Upstream: Filter.h / Filter.cpp / VectorFilter.h / VectorFilter.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// FilterKind mirrors FilterKind (Filter.h).
type FilterKind int

const (
	// FilterKindDefault is FilterKind::fDefault (random mode).
	FilterKindDefault FilterKind = iota
	// FilterKindDFS is FilterKind::fDFS.
	FilterKindDFS
	// FilterKindMax is MAX_FILTER_KIND_SIZE.
	FilterKindMax
)

// FilterMode mirrors VectorFilter::Mode.
type FilterMode int

const (
	// FilterModeOut is VectorFilter::Mode::FilterOut — reject items in the set.
	FilterModeOut FilterMode = iota
	// FilterModeKeep is VectorFilter::Mode::Keep — reject items NOT in the set.
	FilterModeKeep
)

// VectorFilter mirrors Filter + VectorFilter.
// C++: kinds_ bitset all-set in Filter ctor; VectorFilter holds vs_, ptable, mode_.
type VectorFilter struct {
	// kinds mirrors Filter::kinds_ — index by FilterKind; true = enabled for that mode.
	kinds [FilterKindMax]bool

	// items mirrors VectorFilter::vs_.
	items []int
	// table mirrors VectorFilter::ptable (may be nil).
	table *DistributionTable
	// mode mirrors VectorFilter::mode_.
	mode FilterMode
}

// NewVectorFilter mirrors VectorFilter(DistributionTable*) — FilterOut, empty set.
// Filter.cpp:40 — kinds_.set() all true.
func NewVectorFilter(table *DistributionTable) *VectorFilter {
	f := &VectorFilter{table: table, mode: FilterModeOut}
	for i := range f.kinds {
		f.kinds[i] = true
	}
	return f
}

// NewVectorFilterItems mirrors VectorFilter(vector&, Mode).
func NewVectorFilterItems(items []int, mode FilterMode) *VectorFilter {
	f := &VectorFilter{mode: mode, table: nil}
	for i := range f.kinds {
		f.kinds[i] = true
	}
	for _, it := range items {
		f.Add(it)
	}
	return f
}

// Enable mirrors Filter::enable.
func (f *VectorFilter) Enable(kind FilterKind) {
	if f == nil {
		SetError(ErrGeneric)
		return
	}
	if kind < 0 || kind >= FilterKindMax {
		return
	}
	f.kinds[kind] = true
}

// Disable mirrors Filter::disable.
func (f *VectorFilter) Disable(kind FilterKind) {
	if f == nil {
		SetError(ErrGeneric)
		return
	}
	if kind < 0 || kind >= FilterKindMax {
		return
	}
	f.kinds[kind] = false
}

// CurrentKind mirrors Filter::current_kind.
// Filter.cpp:63–68 — random_based → fDefault; dfs_exhaustive → fDFS; else MAX.
func (f *VectorFilter) CurrentKind() FilterKind {
	o := ProcessOptions()
	if o.RandomBased {
		return FilterKindDefault
	}
	if o.DFSExhaustive {
		return FilterKindDFS
	}
	return FilterKindMax
}

// ValidFilter mirrors Filter::valid_filter.
// Filter.cpp:74–79 — kinds_.test(current_kind); false if kind is MAX or disabled.
func (f *VectorFilter) ValidFilter() bool {
	if f == nil {
		SetError(ErrGeneric)
		return false
	}
	k := f.CurrentKind()
	if k < 0 || k >= FilterKindMax {
		return false
	}
	return f.kinds[k]
}

// Add mirrors VectorFilter::add — push item if not already present.
func (f *VectorFilter) Add(item int) *VectorFilter {
	if f == nil {
		SetError(ErrGeneric)
		return f
	}
	for _, x := range f.items {
		if x == item {
			return f
		}
	}
	f.items = append(f.items, item)
	return f
}

// MaxProb mirrors VectorFilter::get_max_prob.
// VectorFilter.cpp:75–77 — ptable ? ptable->get_max() : 100.
// Nil receiver is a Go hole (C++ would not call through null); fail closed 0.
func (f *VectorFilter) MaxProb() int {
	if f == nil {
		SetError(ErrGeneric)
		return 0
	}
	if f.table == nil {
		return 100
	}
	return f.table.Max()
}

// Lookup mirrors VectorFilter::lookup.
// VectorFilter.cpp:79–83 — if !valid_filter() || ptable==nullptr return v;
// else ptable->rnd_num_to_key(v).
func (f *VectorFilter) Lookup(v int) int {
	if f == nil {
		SetError(ErrGeneric)
		return -1
	}
	if !f.ValidFilter() || f.table == nil {
		return v
	}
	return f.table.RndNumToKey(v)
}

// Filter implements Filter — true means reject this rnd draw.
// VectorFilter.cpp:58–66.
// Nil receiver: Go hole fail-closed reject (C++ null would crash).
func (f *VectorFilter) Filter(v uint32) bool {
	if f == nil {
		SetError(ErrGeneric)
		return true
	}
	// VectorFilter.cpp:59–60 — invalid filter never rejects
	if !f.ValidFilter() {
		return false
	}
	key := f.Lookup(int(v))
	found := false
	for _, x := range f.items {
		if x == key {
			found = true
			break
		}
	}
	if f.mode == FilterModeOut {
		return found
	}
	// Keep: reject if NOT in set
	return !found
}
