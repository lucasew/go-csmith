// Upstream: VectorFilter.h / VectorFilter.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VectorFilter mirrors VectorFilter over a DistributionTable (FilterOut mode).
// filter(v): lookup key from table, reject if key is in the filter-out set.
type VectorFilter struct {
	table   *DistributionTable
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
func (f *VectorFilter) Add(item int) *VectorFilter {
	if f == nil {
		return f
	}
	f.filterOut[item] = true
	return f
}

// MaxProb mirrors get_max_prob.
func (f *VectorFilter) MaxProb() int {
	if f == nil || f.table == nil {
		return 100
	}
	return f.table.Max()
}

// Lookup mirrors VectorFilter::lookup → ptable->rnd_num_to_key(v).
func (f *VectorFilter) Lookup(v int) int {
	if f == nil || f.table == nil {
		return v
	}
	return f.table.RndNumToKey(v)
}

// Filter implements Filter — true means reject this rnd draw.
// VectorFilter.cpp: filter(v) after lookup; FilterOut mode.
func (f *VectorFilter) Filter(v uint32) bool {
	if f == nil {
		return false
	}
	key := f.Lookup(int(v))
	return f.filterOut[key]
}
