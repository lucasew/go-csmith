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
	// Sess is the residual sticky bag from New*Sess (Generate run bag or test ambient).
	// Non-Sess dual methods (Filter for interface, Add/MaxProb/Lookup) use vfSess.
	Sess *Session

	// kinds mirrors Filter::kinds_ — index by FilterKind; true = enabled for that mode.
	kinds [FilterKindMax]bool

	// items mirrors VectorFilter::vs_.
	items []int
	// table mirrors VectorFilter::ptable (may be nil).
	table *DistributionTable
	// mode mirrors VectorFilter::mode_.
	mode FilterMode
	// modeKind snapshots Filter::current_kind() from session Options at construction
	// so Filter()/ValidFilter do not residual-read ProcessOptions ambient mid-draw.
	modeKind FilterKind
	// modeKindSet is true when modeKind was snapped from opts (not zero-value default).
	modeKindSet bool
}

// vfSess returns f.Sess. Nil f or unset Sess → testAmbientSession (unit-test nil /
// Filter-interface dual only). Prefer *Sess methods with explicit bag.
func vfSess(f *VectorFilter) *Session {
	if f != nil && f.Sess != nil {
		return f.Sess
	}
	return testAmbientSession
}

// NewVectorFilterSess is NewVectorFilter with current_kind snapshotted from bag opts.
// Filter.cpp:40 — kinds_.set() all true. Non-Sess dual deleted.
func NewVectorFilterSess(s *Session, table *DistributionTable) *VectorFilter {
	f := &VectorFilter{Sess: s, table: table, mode: FilterModeOut}
	for i := range f.kinds {
		f.kinds[i] = true
	}
	f.snapModeKind(s)
	return f
}

// NewVectorFilterItemsSess is NewVectorFilterItems with current_kind snapshotted from bag opts.
// Non-Sess dual deleted.
func NewVectorFilterItemsSess(s *Session, items []int, mode FilterMode) *VectorFilter {
	f := &VectorFilter{Sess: s, mode: mode, table: nil}
	for i := range f.kinds {
		f.kinds[i] = true
	}
	for _, it := range items {
		f.AddSess(s, it)
	}
	f.snapModeKind(s)
	return f
}

func (f *VectorFilter) snapModeKind(s *Session) {
	if f == nil {
		return
	}
	f.modeKind = f.CurrentKindOpts(sessOpts(s))
	f.modeKindSet = true
}

// Enable routes residual sticky via vfSess (f.Sess or unit-test ambient).
// Prefer EnableSess with an explicit bag.
func (f *VectorFilter) Enable(kind FilterKind) {
	f.EnableSess(vfSess(f), kind)
}

// EnableSess is Enable with explicit session residual sticky.
func (f *VectorFilter) EnableSess(s *Session, kind FilterKind) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if kind < 0 || kind >= FilterKindMax {
		return
	}
	f.kinds[kind] = true
}

// Disable routes residual sticky via vfSess (f.Sess or unit-test ambient).
func (f *VectorFilter) Disable(kind FilterKind) {
	f.DisableSess(vfSess(f), kind)
}

// DisableSess is Disable with explicit session residual sticky.
func (f *VectorFilter) DisableSess(s *Session, kind FilterKind) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if kind < 0 || kind >= FilterKindMax {
		return
	}
	f.kinds[kind] = false
}

// CurrentKind mirrors Filter::current_kind.
// Filter.cpp:63–68 — random_based → fDefault; dfs_exhaustive → fDFS; else MAX.

// CurrentKindSess is CurrentKind with explicit session residual sticky.
func (f *VectorFilter) CurrentKind() FilterKind {
	return f.CurrentKindSess(vfSess(f))
}

func (f *VectorFilter) CurrentKindSess(s *Session) FilterKind {
	return f.CurrentKindOpts(sessOpts(s))
}

// CurrentKindOpts is CurrentKind with explicit session Options.
func (f *VectorFilter) CurrentKindOpts(o Options) FilterKind {
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

// ValidFilterSess is ValidFilter with explicit session residual sticky.
func (f *VectorFilter) ValidFilter() bool {
	return f.ValidFilterSess(vfSess(f))
}

func (f *VectorFilter) ValidFilterSess(s *Session) bool {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// Prefer construction snapshot so Filter interface draws stay bag-local.
	k := f.modeKind
	if !f.modeKindSet {
		k = f.CurrentKindSess(s)
	}
	if k < 0 || k >= FilterKindMax {
		return false
	}
	return f.kinds[k]
}

// Add mirrors VectorFilter::add — push item if not already present.

// AddSess is Add with explicit session residual sticky.
func (f *VectorFilter) AddSess(s *Session, item int) *VectorFilter {
	if f == nil {
		sessNoteError(s, ErrGeneric)
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

// Add routes residual sticky via vfSess (f.Sess or unit-test ambient).
func (f *VectorFilter) Add(item int) *VectorFilter {
	return f.AddSess(vfSess(f), item)
}

// MaxProb mirrors VectorFilter::get_max_prob.
// VectorFilter.cpp:75–77 — ptable ? ptable->get_max() : 100.
// Nil receiver is a Go hole (C++ would not call through null); fail closed 0.

// MaxProbSess is MaxProb with explicit session residual sticky.
func (f *VectorFilter) MaxProbSess(s *Session) int {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	if f.table == nil {
		return 100
	}
	return f.table.MaxSess(s)
}

// MaxProb routes residual sticky via vfSess (f.Sess or unit-test ambient).
func (f *VectorFilter) MaxProb() int {
	return f.MaxProbSess(vfSess(f))
}

// Lookup mirrors VectorFilter::lookup.
// VectorFilter.cpp:79–83 — if !valid_filter() || ptable==nullptr return v;
// else ptable->rnd_num_to_key(v).

// LookupSess is Lookup with explicit session residual sticky.
func (f *VectorFilter) LookupSess(s *Session, v int) int {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return -1
	}
	if !f.ValidFilterSess(s) || f.table == nil {
		return v
	}
	return f.table.RndNumToKeySess(s, v)
}

// Lookup routes residual sticky via vfSess (f.Sess or unit-test ambient).
func (f *VectorFilter) Lookup(v int) int {
	return f.LookupSess(vfSess(f), v)
}

// Filter implements Filter interface (RndUptoFilterSess type assert).
// Routes residual sticky via vfSess (f.Sess when set by New*Sess).
// Prefer FilterSess with an explicit bag on the generation path.
func (f *VectorFilter) Filter(v uint32) bool {
	return f.FilterSess(vfSess(f), v)
}

// FilterSess is Filter with explicit session residual sticky.
// VectorFilter.cpp:58–66.
func (f *VectorFilter) FilterSess(s *Session, v uint32) bool {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	// VectorFilter.cpp:59–60 — invalid filter never rejects
	if !f.ValidFilterSess(s) {
		return false
	}
	key := f.LookupSess(s, int(v))
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
