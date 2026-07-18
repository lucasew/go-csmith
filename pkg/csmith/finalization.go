// Upstream: Finalization.cpp — end-of-generation cleanup.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// DoFinalization mirrors Finalization::doFinalization.
// Finalization.cpp:45–55 — clear package-level pools between runs.
func DoFinalization() {
	// Variable::doFinalization — ctrl vars
	CtrlVarsDoFinalization()
	// Bookkeeper::doFinalization
	BookkeeperDoFinalization()
	// FactPointTo::all_ptrs / all_aliases
	ClearPointToAggregates()
	// FactMgr::meta_facts
	ClearMetaFacts()
	// Attribute generators
	ClearAttrGenerators()
	// FunctionInvocationUser return-fact registry
	InvocationReturnFactsDoFinalization()
	// PartialExpander
	ClearPartialExpander()
	// SafeOpFlags wrapper name registry
	ClearSafeOpWrapperNames()
	// Statement sid
	nextStmID = 0
	// Error state
	ClearError()
}
