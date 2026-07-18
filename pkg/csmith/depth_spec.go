// Upstream: DepthSpec.h / DepthSpec.cpp (depth_guard for DFS exhaustive mode).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Depth guard results (DepthSpec.h).
const (
	// GoodDepth is GOOD_DEPTH.
	GoodDepth = 0
	// BadDepth is BAD_DEPTH.
	BadDepth = -1
)

// DepthGuardByDepth mirrors DepthSpec::depth_guard_by_depth.
// DepthSpec.cpp:330–335 — always GOOD_DEPTH when !dfs_exhaustive (random mode).
func DepthGuardByDepth(opts Options, depthNeeded int) int {
	_ = depthNeeded
	if opts.DFSExhaustive {
		// DFS backtracking not ported; treat as GOOD to avoid inventing DFS pads
		return GoodDepth
	}
	return GoodDepth
}

// DepthGuardByType mirrors DepthSpec::depth_guard_by_type.
// DepthSpec.cpp:337+ — always GOOD when !dfs_exhaustive.
func DepthGuardByType(opts Options, _ string) int {
	if opts.DFSExhaustive {
		return GoodDepth
	}
	return GoodDepth
}
