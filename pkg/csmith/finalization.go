// Upstream: Finalization.cpp — end-of-generation cleanup.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// DoFinalizationSess clears generation pools on an explicit session bag.
// Mirrors Finalization::doFinalization (Finalization.cpp:45–55).
// Non-Sess DoFinalization deleted — pass testAmbientSession from unit tests.
func DoFinalizationSess(s *Session) {
	s = sessOrAmbient(s)
	// ArrayVariable.cpp static seed in build_init_recursive — fresh generation
	ResetArrayInitSeedSess(s)
	// Function::doFinalization — FuncList/FMList are session-scoped on ProgramGenerator;
	// FactMgr::doFinalization (meta_facts) below.
	// VariableSelector::doFinalization — session VS; AllVars cleared per NewVariableSelector.
	// Variable::doFinalization — ctrl vars
	CtrlVarsDoFinalizationSess(s)
	// Type::doFinalization — derived pointer cache
	TypeDoFinalizationSess(s)
	// Bookkeeper::doFinalization
	BookkeeperDoFinalizationSess(s)
	// Fact::doFinalization + FactPointTo::all_ptrs / all_aliases
	FactDoFinalizationSess(s)
	// FactMgr::doFinalization / meta_facts
	ClearMetaFactsSess(s)
	// Attribute generators
	ClearAttrGeneratorsSess(s)
	// FunctionInvocationUser return-fact registry
	InvocationReturnFactsDoFinalizationSess(s)
	// PartialExpander
	ClearPartialExpanderSess(s)
	// ExtensionMgr::DestroyExtension
	DestroyExtensionSess(s)
	// SafeOpFlags wrapper name registry
	ClearSafeOpWrapperNamesSess(s)
	// StatementGoto::stm_labels
	GotoLabelsDoFinalizationSess(s)
	// Statement sid
	s.NextStmID = 0
	// util.cpp reset_gensym — process gensym_count (DFSProgramGenerator.cpp:92)
	// DefaultProgramGenerator process is one-shot; library multi-Generate needs reset
	ResetDefaultGensymSess(s)
	// Probabilities / Rng / StmtTab process handles are re-installed by
	// ProgramGenerator.Initialize after this call (same generation run).
	// Clearing them here would soft-break nested make_random mid-run.
	// CompatibleChecker process static (re-enable via resolve if needed)
	ResetCompatibleCheckSess(s)
	// OutputMgr monitored_funcs_ / curr_func_
	ClearMonitoredFuncsSess(s)
	// DefaultOutputMgr / DFSOutputMgr process singleton
	ClearOutputMgrSess(s)
	// AbsProgramGenerator::current_generator_
	s.ProgramGen = nil
	// RandomNumber::doFinalization — Finalization.cpp:53
	// Cleared here; ProgramGenerator re-CreateInstance / SetProcessRng after.
	RandomNumberDoFinalizationSess(s)
	// util.cpp errlog
	ClearAnalysisErrLogSess(s)
	// Error state
	s.GenError = ErrSuccess
}
