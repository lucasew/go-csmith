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
	// Fact::doFinalization + FactPointTo::all_ptrs / all_aliases
	FactDoFinalization()
	// FactMgr::doFinalization / meta_facts
	ClearMetaFacts()
	// Attribute generators
	ClearAttrGenerators()
	// FunctionInvocationUser return-fact registry
	InvocationReturnFactsDoFinalization()
	// PartialExpander
	ClearPartialExpander()
	// ExtensionMgr::DestroyExtension
	DestroyExtension()
	// SafeOpFlags wrapper name registry
	ClearSafeOpWrapperNames()
	// StatementGoto::stm_labels
	GotoLabelsDoFinalization()
	// Statement sid
	nextStmID = 0
	// util.cpp reset_gensym — process gensym_count (DFSProgramGenerator.cpp:92)
	// DefaultProgramGenerator process is one-shot; library multi-Generate needs reset
	ResetDefaultGensym()
	// Probabilities / Rng / StmtTab process handles are re-installed by
	// ProgramGenerator.Initialize after this call (same generation run).
	// Clearing them here would soft-break nested make_random mid-run.
	// CompatibleChecker process static (re-enable via resolve if needed)
	ResetCompatibleCheck()
	// OutputMgr monitored_funcs_ / curr_func_
	ClearMonitoredFuncs()
	// DefaultOutputMgr / DFSOutputMgr process singleton
	ClearOutputMgr()
	// AbsProgramGenerator::current_generator_
	ClearProcessProgramGenerator()
	// RandomNumber::doFinalization — Finalization.cpp:53
	// Cleared here; ProgramGenerator re-CreateInstance / SetProcessRng after.
	RandomNumberDoFinalization()
	// util.cpp errlog
	ClearAnalysisErrLog()
	// Error state
	ClearError()
}
