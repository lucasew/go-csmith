// Upstream: Expression::func_count / get_called_funcs;
// FunctionInvocation::has_uncertain_call(_recursive);
// Statement::get_called_funcs / get_direct_invocation / find_contained_labels;
// StatementIf::combine_branch_facts.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// InvocationsComplete reports every Invocation* is live (no nil holes).
// Note: InvocationsComplete(nil)==true (complete empty). Fail-closed incomplete
// wipes must use IncompleteInvocationsSlice() so len(nil)==0 cannot invent empty success.
func InvocationsComplete(calls []*Invocation) bool {
	for _, c := range calls {
		if c == nil {
			return false
		}
	}
	return true
}

// IncompleteInvocationsSlice is the fail-closed incomplete call-list marker.
// InvocationsComplete returns false. Distinct from complete empty (nil or {}).
func IncompleteInvocationsSlice() []*Invocation {
	return []*Invocation{nil}
}

// CollectCalledInvocationsExpr mirrors Expression::get_called_funcs for user calls.
// FunctionInvocation.cpp:369–381 — recurse args, then push user call.
// Incomplete IR sets *out sticky IncompleteInvocationsSlice (not bare nil —
// len(nil)==0 invents empty-complete call list / soft re-pick past holes).
// out always live; sticky (no invent soft-skip collect past hole).
func CollectCalledInvocationsExpr(e *Expression, out *[]*Invocation) {
	CollectCalledInvocationsExprSess(nil, e, out)
}

func CollectCalledInvocationsExprSess(s *Session, e *Expression, out *[]*Invocation) {
	if out == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if !collectCalledInvocationsExpr(e, out) {
		*out = IncompleteInvocationsSlice()
		sessNoteError(s, ErrGeneric)
	}
}

// collectCalledInvocationsExpr returns false on incomplete IR (*out cleared by caller).
func collectCalledInvocationsExpr(e *Expression, out *[]*Invocation) bool {
	if e == nil || out == nil {
		return false
	}
	switch e.Term {
	case TermConstant, TermVariable:
		return true
	case TermFunction:
		// ExpressionFuncall always has live invoke; param_value[i] always live
		if e.Invoke == nil {
			return false
		}
		for _, a := range e.Invoke.Args {
			if a == nil {
				return false
			}
			if !collectCalledInvocationsExpr(a, out) {
				return false
			}
		}
		if e.Invoke.User != nil {
			*out = append(*out, e.Invoke)
		}
		return true
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			return false
		}
		if !collectCalledInvocationsExpr(e.CommaLHS, out) {
			return false
		}
		return collectCalledInvocationsExpr(e.CommaRHS, out)
	case TermAssignment:
		if e.Assign == nil {
			return false
		}
		return collectCalledInvocationsStmt(e.Assign, out)
	default:
		// unknown term — incomplete IR
		return false
	}
}

// CollectCalledInvocationsStmt mirrors Statement::get_called_funcs.
// Statement.cpp:748–762 — get_exprs + get_blocks.
// Incomplete IR sets *out sticky IncompleteInvocationsSlice (not bare nil invent empty).
// out always live; sticky (no invent soft-skip collect past hole).
func CollectCalledInvocationsStmt(st *Stmt, out *[]*Invocation) {
	CollectCalledInvocationsStmtSess(nil, st, out)
}

func CollectCalledInvocationsStmtSess(s *Session, st *Stmt, out *[]*Invocation) {
	if out == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if !collectCalledInvocationsStmt(st, out) {
		*out = IncompleteInvocationsSlice()
		sessNoteError(s, ErrGeneric)
	}
}

func collectCalledInvocationsStmt(st *Stmt, out *[]*Invocation) bool {
	if st == nil || out == nil {
		return false
	}
	// StatementFor::get_exprs → test only (not st.Expr).
	// StatementArrayOp.h:65–68 — if (init_value) only; NOT For test.
	// Kind-gated: no invent treat Loop-on-wrong-kind as for get_exprs.
	// Fair: array_init numeric LoopControl has no TestExpr (seed-2 ComputeSummary).
	switch st.Kind {
	case StmtFor:
		if st.Loop == nil || st.Loop.TestExpr == nil {
			return false
		}
		if !collectCalledInvocationsExpr(st.Loop.TestExpr, out) {
			return false
		}
	case StmtArrayOp:
		// optional init_value; body path has none (walk get_blocks)
		if st.Expr != nil {
			if !collectCalledInvocationsExpr(st.Expr, out) {
				return false
			}
		}
	case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
		// C++ get_exprs always yields live Expression* for these kinds
		// incomplete nil Expr fails closed (no invent empty call list as success)
		if st.Expr == nil {
			return false
		}
		if !collectCalledInvocationsExpr(st.Expr, out) {
			return false
		}
	default:
		// other kinds: optional expr if present
		if st.Expr != nil {
			if !collectCalledInvocationsExpr(st.Expr, out) {
				return false
			}
		}
	}
	// get_blocks → Then/Else
	for _, b := range GetBlocksStmt(st) {
		if b == nil {
			return false
		}
		if !collectCalledInvocationsBlock(b, out) {
			return false
		}
	}
	return true
}

// CollectCalledInvocationsBlock walks all statements in a block.
// Incomplete IR sets *out sticky IncompleteInvocationsSlice (not bare nil invent empty).
// out always live; sticky (no invent soft-skip collect past hole).
func CollectCalledInvocationsBlock(b *Block, out *[]*Invocation) {
	CollectCalledInvocationsBlockSess(nil, b, out)
}

func CollectCalledInvocationsBlockSess(s *Session, b *Block, out *[]*Invocation) {
	if out == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if !collectCalledInvocationsBlock(b, out) {
		*out = IncompleteInvocationsSlice()
		sessNoteError(s, ErrGeneric)
	}
}

func collectCalledInvocationsBlock(b *Block, out *[]*Invocation) bool {
	if b == nil || out == nil {
		return false
	}
	for i := range b.Stmts {
		if !collectCalledInvocationsStmt(&b.Stmts[i], out) {
			return false
		}
	}
	return true
}

// FuncCount mirrors Expression::func_count.
// Expression.cpp:114–118.
// Incomplete IR fails closed as -1 (no invent empty call count past holes).
func FuncCount(e *Expression) int {
	return FuncCountSess(nil, e)
}

func FuncCountSess(s *Session, e *Expression) int {
	var calls []*Invocation
	CollectCalledInvocationsExprSess(s, e, &calls)
	if !InvocationsComplete(calls) {
		return -1
	}
	return len(calls)
}

// HasUncertainCall mirrors FunctionInvocation::has_uncertain_call.
// FunctionInvocation.cpp:383–394 — ≥2 params each containing a call.
// Nil invoke / nil arg / incomplete FuncCount sticky true
// (no invent certain order / no-call soft-skip past hole).
func (fi *Invocation) HasUncertainCall() bool {
	return fi.HasUncertainCallSess(nil)
}

func (fi *Invocation) HasUncertainCallSess(s *Session) bool {
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	cnt := 0
	for _, a := range fi.Args {
		if a == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		n := FuncCountSess(s, a)
		if n < 0 {
			// FuncCount incomplete already stickies when needed; keep restrictive true
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return true
		}
		if n > 0 {
			cnt++
		}
	}
	return cnt >= 2
}

// HasUncertainCallRecursive mirrors FunctionInvocation::has_uncertain_call_recursive.
// FunctionInvocation.cpp:396–406.
// Nil invoke / nil arg sticky true (no invent skip hole as non-call).
func (fi *Invocation) HasUncertainCallRecursive() bool {
	return fi.HasUncertainCallRecursiveSess(nil)
}

func (fi *Invocation) HasUncertainCallRecursiveSess(s *Session) bool {
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	for _, a := range fi.Args {
		if a == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if a.Term == TermFunction {
			if a.Invoke == nil {
				sessNoteError(s, ErrGeneric)
				return true
			}
			if a.Invoke.HasUncertainCallRecursiveSess(s) {
				// residual ERROR sticky — no invent uncertain true past nested recurse hole
				if sessHasError(s) {
					return true
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue later args past nested residual
			if sessHasError(s) {
				return true
			}
		}
	}
	ok := fi.HasUncertainCallSess(s)
	// residual ERROR sticky — no invent certain soft-skip past HasUncertainCall hole
	if sessHasError(s) {
		return true
	}
	return ok
}

// HasSimpleParams mirrors FunctionInvocation::has_simple_params.
// FunctionInvocation.cpp:408–416 — no TermFunction args.
// Nil invoke / nil arg sticky false (no invent simple past hole).
func (fi *Invocation) HasSimpleParams() bool {
	return fi.HasSimpleParamsSess(nil)
}

func (fi *Invocation) HasSimpleParamsSess(s *Session) bool {
	// C++ always has live FunctionInvocation*; nil shell sticky not-simple
	// (no invent simple-params success without args IR)
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	for _, a := range fi.Args {
		// param_value[i] always live; nil hole sticky not-simple
		if a == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		if a.Term == TermFunction {
			return false
		}
	}
	return true
}

// HasUncertainCallRecursiveExpr mirrors Expression::has_uncertain_call_recursive.
// ExpressionFuncall / Comma / Assign overrides; default false.
// Incomplete IR sticky true (no invent "no uncertain call" soft-skip past hole).
func HasUncertainCallRecursiveExpr(e *Expression) bool {
	return HasUncertainCallRecursiveExprSess(nil, e)
}

func HasUncertainCallRecursiveExprSess(s *Session, e *Expression) bool {
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	switch e.Term {
	case TermFunction:
		if e.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		ok := e.Invoke.HasUncertainCallRecursiveSess(s)
		// residual ERROR sticky — no invent certain soft-skip past Invoke recurse residual
		if sessHasError(s) {
			return true
		}
		return ok
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if HasUncertainCallRecursiveExprSess(s, e.CommaLHS) {
			// residual ERROR sticky — no invent uncertain true past LHS hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue RHS past LHS residual
		if sessHasError(s) {
			return true
		}
		if HasUncertainCallRecursiveExprSess(s, e.CommaRHS) {
			if sessHasError(s) {
				return true
			}
			return true
		}
		if sessHasError(s) {
			return true
		}
		return false
	case TermAssignment:
		if e.Assign == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		ok := HasUncertainCallRecursiveStmtSess(s, e.Assign)
		// residual ERROR sticky — no invent certain soft-skip past Assign recurse hole
		if sessHasError(s) {
			return true
		}
		return ok
	default:
		return false
	}
}

// HasUncertainCallRecursiveStmt mirrors Statement::has_uncertain_call_recursive.
// Statement.h:185 — base returns false. Only StatementAssign and StatementExpr
// override (StatementAssign.cpp:411–412 / StatementExpr.cpp:134–135) and
// delegate to Expression::has_uncertain_call_recursive.
// Soft invent treated StmtIfElse/Return/For/ArrayOp/jump like Assign (walk
// expr/body). That fired Statement.cpp:969 special validate for a long-lived
// func_1 if whose pre_facts was empty (capture before pointer globals), wiping
// post-combine may-null lattices (seed-250 g_67 → init-only; Lhs F0 miss).
// C++ StatementIf never overrides — special path never runs for if (combine
// result kept). StatementIf.cpp:79 is condition re-analyze at make_random only.}

func HasUncertainCallRecursiveStmt(st *Stmt) bool {
	return HasUncertainCallRecursiveStmtSess(nil, st)
}

func HasUncertainCallRecursiveStmtSess(s *Session, st *Stmt) bool {
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	switch st.Kind {
	case StmtAssign, StmtInvoke:
		// StatementAssign / StatementExpr overrides only
		if st.Expr == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		ok := HasUncertainCallRecursiveExprSess(s, st.Expr)
		// residual ERROR sticky — no invent certain soft-skip past expr recurse residual
		if sessHasError(s) {
			return true
		}
		return ok
	default:
		// Statement.h:185 — base false (If/For/Return/ArrayOp/jump/…)
		return false
	}
}

// GetDirectInvocation mirrors Statement::get_direct_invocation.
// Statement.cpp:714–734 — assign RHS, invoke, or if-test when TermFunction.
// Expression* always live for these kinds; incomplete Expr/Invoke fails closed
// as Failed shell (no invent nil "no call" for broken IR).}

func GetDirectInvocation(st *Stmt) *Invocation {
	return GetDirectInvocationSess(nil, st)
}

func GetDirectInvocationSess(s *Session, st *Stmt) *Invocation {
	// Statement always live for call extract; sticky no invent "no call" without it
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	switch st.Kind {
	case StmtAssign, StmtIfElse:
		// StatementAssign/If always have live get_expr/get_test
		if st.Expr == nil {
			// incomplete Expr sticky Failed shell (no invent nil "no call" soft-skip)
			sessNoteError(s, ErrGeneric)
			return &Invocation{Failed: true}
		}
		if st.Expr.Term != TermFunction {
			return nil
		}
		if st.Expr.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return &Invocation{Failed: true}
		}
		return st.Expr.Invoke
	case StmtInvoke:
		// StatementExpr always has live get_invoke
		if st.Expr == nil || st.Expr.Term != TermFunction {
			sessNoteError(s, ErrGeneric)
			return &Invocation{Failed: true}
		}
		if st.Expr.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return &Invocation{Failed: true}
		}
		return st.Expr.Invoke
	}
	return nil
}

// FindContainedLabels mirrors Statement::find_contained_labels without FactMgr.
// Uses SourceLabel (set at generation when dest is labeled).}

func FindContainedLabels(st *Stmt) []string {
	return FindContainedLabelsFM(st, nil)
}

// LabelsComplete reports every label string slot is present (no nil-like hole
// marker). Incomplete lists use IncompleteLabelsSlice() so len(nil)==0 cannot
// invent empty-complete label success.
func LabelsComplete(labels []string) bool {
	// incomplete marker is a single empty string with a sentinel prefix
	if len(labels) == 1 && labels[0] == incompleteLabelSentinel {
		return false
	}
	return true
}

// incompleteLabelSentinel is not a valid C identifier/label token from CFG.
const incompleteLabelSentinel = "\x00incomplete_labels"

// IncompleteLabelsSlice is the fail-closed incomplete label-list marker.
func IncompleteLabelsSlice() []string {
	return []string{incompleteLabelSentinel}
}

// FindContainedLabelsFM mirrors Statement::find_contained_labels.
// Statement.cpp:706–720 — find_jump_label (CFG goto) then nested blocks.
// When fm is nil, falls back to SourceLabel set during generation (no CFG).
// With FactMgr: same as PreOutput — only CFG/registry labels; no invent SourceLabel
// when find_jump_label is empty. Incomplete CFG/tree fails closed sticky
// IncompleteLabelsSlice (not bare nil invent empty-complete / soft re-pick past hole).
func FindContainedLabelsFM(st *Stmt, fm *FactMgr) []string {
	if st == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteLabelsSlice()
	}
	// incomplete CFG fails whole label collect sticky (no invent partial / empty complete)
	if fm != nil && !CFGEdgesComplete(fm.CFGEdges) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteLabelsSlice()
	}
	var labels []string
	if !findContainedLabels(st, &labels, fm) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteLabelsSlice()
	}
	return labels
}

// findContainedLabels walks get_blocks for labels. Returns false on incomplete
// Block* hole (no invent partial label list past missing if-arm).
func findContainedLabels(st *Stmt, labels *[]string, fm *FactMgr) bool {
	if st == nil || labels == nil {
		return false
	}
	// Statement.cpp:707–710 — find_jump_label()
	lab := ""
	if fm != nil {
		// PreOutput: with FM, never fall back to SourceLabel
		// Statement::stm_id always live; StmID 0 fails closed (no invent skip
		// self-label and still claim complete label list from children only)
		if StmIDUnset(st.StmID) {
			return false
		}
		lab = FindJumpLabel(fm, st.StmID)
	} else if st.SourceLabel != "" {
		lab = st.SourceLabel
	}
	if lab != "" {
		*labels = append(*labels, lab)
	}
	// get_blocks only — no invent labels via stray Then on assign
	blks := GetBlocksStmt(st)
	for _, b := range blks {
		if b == nil {
			return false
		}
	}
	for _, b := range blks {
		for i := range b.Stmts {
			if !findContainedLabels(&b.Stmts[i], labels, fm) {
				return false
			}
		}
	}
	return true
}

// CombineBranchFacts mirrors StatementIf::combine_branch_facts.
// StatementIf.cpp:208–236 — merge then/else outs with must_return precision.
// C++ operates on full FactVec (ePointTo + eUnionWrite). Soft invent was
// point-to-only GlobalFacts merge leaving UnionFacts at else-exit last-writes
// so IsNonreadableField over/under-filtered choose_var after every if
// (seed-7 eligible pool half-size vs upstream).
// preUnion is the eUnionWrite partition of pre_facts (mutated in place for makeup
// like preFacts, then used by caller for set_fact_in).
// Fact maps always complete; nil holes fail closed (both partitions wiped).
// Statement + FactMgr always live; sticky (no invent soft-skip combine past hole).
// Non-if Kind is complete no-op.
func CombineBranchFacts(st *Stmt, preFacts *[]*FactPointTo, preUnion *[]*FactUnion, fm *FactMgr) {
	if st == nil || fm == nil || preFacts == nil || preUnion == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if st.Kind != StmtIfElse {
		return
	}
	// StatementIf always has live if_true / if_false blocks after make_random
	// incomplete arms fail closed sticky (no invent empty then/else via nil-out + FactsComplete)
	if st.Then == nil || st.Else == nil {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// Block::stm_id always live; StmID 0 + FactsComplete(nil) would invent empty
	// arm outs as complete (no invent soft empty map for missing block id)
	if StmIDUnset(st.Then.StmID) || StmIDUnset(st.Else.StmID) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	thenOut := fm.GetMapFactsOut(st.Then.StmID)
	elseOut := fm.GetMapFactsOut(st.Else.StmID)
	thenOutU := fm.GetMapUnionFactsOut(st.Then.StmID)
	elseOutU := fm.GetMapUnionFactsOut(st.Else.StmID)
	// Fact* always live in maps used for branch combine (both partitions)
	if !FactsComplete(*preFacts) || !FactsComplete(thenOut) || !FactsComplete(elseOut) ||
		!UnionFactsComplete(*preUnion) || !UnionFactsComplete(thenOutU) || !UnionFactsComplete(elseOutU) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// makeup new vars from branch outs into pre snapshot (full FactVec)
	// sequential: first failure must not invent second makeup from cleared empty
	if !MakeupNewVarFactsSess(fmSess(fm), preFacts, thenOut) || !MakeupNewVarFactsSess(fmSess(fm), preFacts, elseOut) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	if !makeupNewUnionFacts(preUnion, thenOutU) || !makeupNewUnionFacts(preUnion, elseOutU) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}

	trueMust := st.Then.MustReturn()
	// residual ERROR sticky — no invent soft-continue branch-merge past Then MustReturn residual
	if sessHasError(fmSess(fm)) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		return
	}
	falseMust := st.Else.MustReturn()
	// residual ERROR sticky — no invent soft-continue branch-merge past Else MustReturn residual
	if sessHasError(fmSess(fm)) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		return
	}
	switch {
	case trueMust && falseMust:
		// StatementIf.cpp:217–218 — outputs = pre_facts (full)
		fm.SetGlobalFacts(CloneFactSlice(*preFacts), "auto_call_analysis_596")
		if sessHasError(fmSess(fm)) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			return
		}
		cl := CloneUnionFactSliceDeep(*preUnion)
		if sessHasError(fmSess(fm)) || !UnionFactsComplete(cl) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
		if cl == nil {
			fm.UnionFacts = []*FactUnion{}
		} else {
			fm.UnionFacts = cl
		}
	case trueMust:
		// StatementIf.cpp:219–222 — outputs = map_facts_out[if_false]
		fm.SetGlobalFacts(CloneFactSlice(elseOut), "auto_call_analysis_603")
		if sessHasError(fmSess(fm)) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			return
		}
		cl := CloneUnionFactSliceDeep(elseOutU)
		if sessHasError(fmSess(fm)) || !UnionFactsComplete(cl) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
		if cl == nil {
			fm.UnionFacts = []*FactUnion{}
		} else {
			fm.UnionFacts = cl
		}
	case falseMust:
		// StatementIf.cpp:223–227 — outputs = map_facts_out[if_true] + makeup from if_false in
		fm.SetGlobalFacts(CloneFactSlice(thenOut), "auto_call_analysis_610")
		if sessHasError(fmSess(fm)) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			return
		}
		cl := CloneUnionFactSliceDeep(thenOutU)
		if sessHasError(fmSess(fm)) || !UnionFactsComplete(cl) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
		if cl == nil {
			fm.UnionFacts = []*FactUnion{}
		} else {
			fm.UnionFacts = cl
		}
		// StatementIf.cpp:227 — makeup_new_var_facts(outputs, map_facts_in[&if_false])
		in := fm.GetMapFactsIn(st.Else.StmID)
		inU := fm.GetMapUnionFactsIn(st.Else.StmID)
		if !FactsComplete(in) || !UnionFactsComplete(inU) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		if !MakeupNewVarFacts(&fm.GlobalFacts, in) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
		if !makeupNewUnionFacts(&fm.UnionFacts, inU) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
	default:
		// StatementIf.cpp:228–230 — outputs = then_out; merge_facts(outputs, else_out)
		fm.SetGlobalFacts(CloneFactSlice(thenOut), "auto_call_analysis_632")
		if !FactsComplete(fm.GlobalFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		if !FactsComplete(elseOut) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		_ = MergeFactsSess(fmSess(fm), &fm.GlobalFacts, elseOut)
		if !FactsComplete(fm.GlobalFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
		// eUnionWrite half of merge_facts (Fact.cpp:192–199 + FactUnion::join)
		// Deep-clone then arm outs so merge_fact join cannot alias map_facts_out.
		u := CloneUnionFactSliceDeep(thenOutU)
		if sessHasError(fmSess(fm)) || !UnionFactsComplete(u) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
		if u == nil {
			u = []*FactUnion{}
		}
		// Fact.cpp:192–199 merge_facts → merge_fact (not always-join).
		// MergeUnionFact matches Fact.cpp:149–171; MergeUnionFactInto always joins.
		for _, nf := range elseOutU {
			if nf == nil {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
			u = MergeUnionFact(u, nf)
			if !UnionFactsComplete(u) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				if !sessHasError(fmSess(fm)) {
					sessNoteError(fmSess(fm), ErrGeneric)
				}
				return
			}
		}
		fm.UnionFacts = u
	}
}
