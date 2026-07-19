// Upstream: VectorFilter.h / VectorFilter.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VectorFilter mirrors VectorFilter over a DistributionTable (FilterOut mode).
// filter(v): lookup key from table, reject if key is in the filter-out set.
type VectorFilter struct {
	table     *DistributionTable
	filterOut map[int]bool
}

// NewVectorFilter mirrors VectorFilter(DistributionTable*).
func NewVectorFilter(table *DistributionTable) *VectorFilter {
	return &VectorFilter{
		table:     table,
		filterOut: make(map[int]bool),
	}
}

// Add mirrors VectorFilter::add — filter out this term/key.
// Incomplete filter sticky no-op (no invent soft grow past missing shell).
func (f *VectorFilter) Add(item int) *VectorFilter {
	// VectorFilter always live; sticky incomplete no invent nil soft-skip
	if f == nil {
		SetError(ErrGeneric)
		return f
	}
	f.filterOut[item] = true
	return f
}

// MaxProb mirrors get_max_prob.
// DistributionTable* always live on a real filter; nil table sticky 0
// (no invent default domain 100 for RndUptoFilter / soft re-pick).
func (f *VectorFilter) MaxProb() int {
	// VectorFilter + table always live; sticky incomplete no invent domain 0 soft-skip
	if f == nil || f.table == nil {
		SetError(ErrGeneric)
		return 0
	}
	return f.table.Max()
}

// Lookup mirrors VectorFilter::lookup → ptable->rnd_num_to_key(v).
// nil table sticky -1 (no invent identity key passthrough / soft re-pick).
func (f *VectorFilter) Lookup(v int) int {
	// VectorFilter + table always live; sticky incomplete no invent identity key
	if f == nil || f.table == nil {
		SetError(ErrGeneric)
		return -1
	}
	return f.table.RndNumToKey(v)
}

// Filter implements Filter — true means reject this rnd draw.
// VectorFilter.cpp: filter(v) after lookup; FilterOut mode.
// Incomplete filter (nil f/table) sticky reject-all (no invent accept-all).
func (f *VectorFilter) Filter(v uint32) bool {
	// VectorFilter + table always live; sticky incomplete reject-all (restrictive)
	if f == nil || f.table == nil {
		SetError(ErrGeneric)
		return true
	}
	key := f.Lookup(int(v))
	return f.filterOut[key]
}
