// Upstream: Finalization.cpp — end-of-generation cleanup.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// DoFinalization mirrors Finalization::doFinalization.
// Finalization.cpp:45–55 — clear package-level pools between runs.
func DoFinalization() {
	// Function::doFinalization — FuncList/FMList are session-scoped on ProgramGenerator;
	// FactMgr::doFinalization (meta_facts) below.
	// VariableSelector::doFinalization — session VS; AllVars cleared per NewVariableSelector.
	// Variable::doFinalization — ctrl vars
	CtrlVarsDoFinalization()
	// Type::doFinalization — derived pointer cache (process-wide)
	TypeDoFinalization()
	// Bookkeeper::doFinalization
	BookkeeperDoFinalization()
	// FactPointTo::all_ptrs / all_aliases
	ClearPointToAggregates()
	// FactMgr::doFinalization / meta_facts
	ClearMetaFacts()
	// Attribute generators
	ClearAttrGenerators()
	// FunctionInvocationUser return-fact registry
	InvocationReturnFactsDoFinalization()
	// PartialExpander
	ClearPartialExpander()
	// SafeOpFlags wrapper name registry
	ClearSafeOpWrapperNames()
	// StatementGoto::stm_labels
	GotoLabelsDoFinalization()
	// Statement sid
	nextStmID = 0
	// Probabilities::DestroyInstance — session Probs, no process singleton
	// RandomNumber::doFinalization — session Rng
	// Error state
	ClearError()
}
