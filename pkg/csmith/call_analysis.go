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
	if out == nil {
		SetError(ErrGeneric)
		return
	}
	if !collectCalledInvocationsExpr(e, out) {
		*out = IncompleteInvocationsSlice()
		SetError(ErrGeneric)
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
	if out == nil {
		SetError(ErrGeneric)
		return
	}
	if !collectCalledInvocationsStmt(st, out) {
		*out = IncompleteInvocationsSlice()
		SetError(ErrGeneric)
	}
}

func collectCalledInvocationsStmt(st *Stmt, out *[]*Invocation) bool {
	if st == nil || out == nil {
		return false
	}
	// StatementFor/ArrayOp::get_exprs → test only (not st.Expr)
	// Kind-gated: no invent treat Loop-on-wrong-kind as for get_exprs
	if st.Kind == StmtFor || st.Kind == StmtArrayOp {
		if st.Loop == nil || st.Loop.TestExpr == nil {
			return false
		}
		if !collectCalledInvocationsExpr(st.Loop.TestExpr, out) {
			return false
		}
	} else {
		switch st.Kind {
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
	if out == nil {
		SetError(ErrGeneric)
		return
	}
	if !collectCalledInvocationsBlock(b, out) {
		*out = IncompleteInvocationsSlice()
		SetError(ErrGeneric)
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
	var calls []*Invocation
	CollectCalledInvocationsExpr(e, &calls)
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
	if fi == nil {
		SetError(ErrGeneric)
		return true
	}
	cnt := 0
	for _, a := range fi.Args {
		if a == nil {
			SetError(ErrGeneric)
			return true
		}
		n := FuncCount(a)
		if n < 0 {
			// FuncCount incomplete already stickies when needed; keep restrictive true
			if !HasError() {
				SetError(ErrGeneric)
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
	if fi == nil {
		SetError(ErrGeneric)
		return true
	}
	for _, a := range fi.Args {
		if a == nil {
			SetError(ErrGeneric)
			return true
		}
		if a.Term == TermFunction {
			if a.Invoke == nil {
				SetError(ErrGeneric)
				return true
			}
			if a.Invoke.HasUncertainCallRecursive() {
				// residual ERROR sticky — no invent uncertain true past nested recurse hole
				if HasError() {
					return true
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue later args past nested residual
			if HasError() {
				return true
			}
		}
	}
	ok := fi.HasUncertainCall()
	// residual ERROR sticky — no invent certain soft-skip past HasUncertainCall hole
	if HasError() {
		return true
	}
	return ok
}

// HasSimpleParams mirrors FunctionInvocation::has_simple_params.
// FunctionInvocation.cpp:408–416 — no TermFunction args.
// Nil invoke / nil arg sticky false (no invent simple past hole).
func (fi *Invocation) HasSimpleParams() bool {
	// C++ always has live FunctionInvocation*; nil shell sticky not-simple
	// (no invent simple-params success without args IR)
	if fi == nil {
		SetError(ErrGeneric)
		return false
	}
	for _, a := range fi.Args {
		// param_value[i] always live; nil hole sticky not-simple
		if a == nil {
			SetError(ErrGeneric)
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
	if e == nil {
		SetError(ErrGeneric)
		return true
	}
	switch e.Term {
	case TermFunction:
		if e.Invoke == nil {
			SetError(ErrGeneric)
			return true
		}
		ok := e.Invoke.HasUncertainCallRecursive()
		// residual ERROR sticky — no invent certain soft-skip past Invoke recurse residual
		if HasError() {
			return true
		}
		return ok
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			SetError(ErrGeneric)
			return true
		}
		if HasUncertainCallRecursiveExpr(e.CommaLHS) {
			// residual ERROR sticky — no invent uncertain true past LHS hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue RHS past LHS residual
		if HasError() {
			return true
		}
		if HasUncertainCallRecursiveExpr(e.CommaRHS) {
			if HasError() {
				return true
			}
			return true
		}
		if HasError() {
			return true
		}
		return false
	case TermAssignment:
		if e.Assign == nil {
			SetError(ErrGeneric)
			return true
		}
		ok := HasUncertainCallRecursiveStmt(e.Assign)
		// residual ERROR sticky — no invent certain soft-skip past Assign recurse hole
		if HasError() {
			return true
		}
		return ok
	default:
		return false
	}
}

// HasUncertainCallRecursiveStmt mirrors Statement::has_uncertain_call_recursive.
// Assign/Invoke/If via expr; for via get_exprs test; default false.
// Incomplete for/expr sticky true (no invent skip for-test calls).
func HasUncertainCallRecursiveStmt(st *Stmt) bool {
	if st == nil {
		SetError(ErrGeneric)
		return true
	}
	switch st.Kind {
	case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
		// C++ always has live get_exprs entries for these kinds
		if st.Expr == nil {
			SetError(ErrGeneric)
			return true
		}
		ok := HasUncertainCallRecursiveExpr(st.Expr)
		// residual ERROR sticky — no invent certain soft-skip past expr recurse residual
		if HasError() {
			return true
		}
		return ok
	case StmtFor, StmtArrayOp:
		// StatementFor::get_exprs → test (not st.Expr)
		if st.Loop == nil || st.Loop.TestExpr == nil {
			SetError(ErrGeneric)
			return true
		}
		if HasUncertainCallRecursiveExpr(st.Loop.TestExpr) {
			// residual ERROR sticky — no invent uncertain true past test-expr hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue body past test residual
		if HasError() {
			return true
		}
		// get_blocks body only — sticky no invent soft-skip nil body as "no uncertain call"
		blks := GetBlocksStmt(st)
		// residual ERROR sticky — no invent certain soft-skip past GetBlocksStmt residual
		if HasError() {
			return true
		}
		for _, b := range blks {
			if b == nil {
				SetError(ErrGeneric)
				return true
			}
			for i := range b.Stmts {
				if HasUncertainCallRecursiveStmt(&b.Stmts[i]) {
					// residual ERROR sticky — no invent uncertain true past nested stmt hole
					if HasError() {
						return true
					}
					return true
				}
				// residual ERROR sticky — no invent soft-continue later stmts past residual
				if HasError() {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// GetDirectInvocation mirrors Statement::get_direct_invocation.
// Statement.cpp:714–734 — assign RHS, invoke, or if-test when TermFunction.
// Expression* always live for these kinds; incomplete Expr/Invoke fails closed
// as Failed shell (no invent nil "no call" for broken IR).
func GetDirectInvocation(st *Stmt) *Invocation {
	// Statement always live for call extract; sticky no invent "no call" without it
	if st == nil {
		SetError(ErrGeneric)
		return nil
	}
	switch st.Kind {
	case StmtAssign, StmtIfElse:
		// StatementAssign/If always have live get_expr/get_test
		if st.Expr == nil {
			// incomplete Expr sticky Failed shell (no invent nil "no call" soft-skip)
			SetError(ErrGeneric)
			return &Invocation{Failed: true}
		}
		if st.Expr.Term != TermFunction {
			return nil
		}
		if st.Expr.Invoke == nil {
			SetError(ErrGeneric)
			return &Invocation{Failed: true}
		}
		return st.Expr.Invoke
	case StmtInvoke:
		// StatementExpr always has live get_invoke
		if st.Expr == nil || st.Expr.Term != TermFunction {
			SetError(ErrGeneric)
			return &Invocation{Failed: true}
		}
		if st.Expr.Invoke == nil {
			SetError(ErrGeneric)
			return &Invocation{Failed: true}
		}
		return st.Expr.Invoke
	}
	return nil
}

// FindContainedLabels mirrors Statement::find_contained_labels without FactMgr.
// Uses SourceLabel (set at generation when dest is labeled).
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
		SetError(ErrGeneric)
		return IncompleteLabelsSlice()
	}
	// incomplete CFG fails whole label collect sticky (no invent partial / empty complete)
	if fm != nil && !CFGEdgesComplete(fm.CFGEdges) {
		SetError(ErrGeneric)
		return IncompleteLabelsSlice()
	}
	var labels []string
	if !findContainedLabels(st, &labels, fm) {
		SetError(ErrGeneric)
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
		if st.StmID <= 0 {
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
// Fact maps always complete; nil holes fail closed (GlobalFacts nil, no invent
// partial then/else join).
// Statement + FactMgr always live; sticky (no invent soft-skip combine past hole).
// Non-if Kind is complete no-op.
func CombineBranchFacts(st *Stmt, preFacts []*FactPointTo, fm *FactMgr) {
	if st == nil || fm == nil {
		SetError(ErrGeneric)
		return
	}
	if st.Kind != StmtIfElse {
		return
	}
	// StatementIf always has live if_true / if_false blocks after make_random
	// incomplete arms fail closed sticky (no invent empty then/else via nil-out + FactsComplete)
	if st.Then == nil || st.Else == nil {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	// Block::stm_id always live; StmID 0 + FactsComplete(nil) would invent empty
	// arm outs as complete (no invent soft empty map for missing block id)
	if st.Then.StmID <= 0 || st.Else.StmID <= 0 {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	thenOut := fm.GetMapFactsOut(st.Then.StmID)
	elseOut := fm.GetMapFactsOut(st.Else.StmID)
	// Fact* always live in maps used for branch combine
	if !FactsComplete(preFacts) || !FactsComplete(thenOut) || !FactsComplete(elseOut) {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	// makeup new vars from branch outs into preFacts snapshot
	// sequential: first failure must not invent second makeup from cleared empty
	if !MakeupNewVarFacts(&preFacts, thenOut) || !MakeupNewVarFacts(&preFacts, elseOut) {
		fm.GlobalFacts = IncompleteFactSlice()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return
	}

	trueMust := st.Then.MustReturn()
	// residual ERROR sticky — no invent soft-continue branch-merge past Then MustReturn residual
	if HasError() {
		fm.GlobalFacts = IncompleteFactSlice()
		return
	}
	falseMust := st.Else.MustReturn()
	// residual ERROR sticky — no invent soft-continue branch-merge past Else MustReturn residual
	if HasError() {
		fm.GlobalFacts = IncompleteFactSlice()
		return
	}
	switch {
	case trueMust && falseMust:
		fm.GlobalFacts = CloneFactSlice(preFacts)
		// residual ERROR sticky — no invent soft-complete GlobalFacts past CloneFactSlice residual
		if HasError() {
			fm.GlobalFacts = IncompleteFactSlice()
			return
		}
	case trueMust:
		fm.GlobalFacts = CloneFactSlice(elseOut)
		// residual ERROR sticky — no invent soft-complete GlobalFacts past CloneFactSlice residual
		if HasError() {
			fm.GlobalFacts = IncompleteFactSlice()
			return
		}
	case falseMust:
		fm.GlobalFacts = CloneFactSlice(thenOut)
		// residual ERROR sticky — no invent soft-complete GlobalFacts past CloneFactSlice residual
		if HasError() {
			fm.GlobalFacts = IncompleteFactSlice()
			return
		}
		// StatementIf.cpp:227 — makeup_new_var_facts(outputs, map_facts_in[&if_false])
		// C++ map[] always; missing → empty makeup; incomplete fails closed
		in := fm.GetMapFactsIn(st.Else.StmID)
		if !FactsComplete(in) {
			fm.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		if !MakeupNewVarFacts(&fm.GlobalFacts, in) {
			fm.GlobalFacts = IncompleteFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
			return
		}
	default:
		fm.GlobalFacts = CloneFactSlice(thenOut)
		// incomplete clone must not invent empty-complete GlobalFacts (bare nil)
		if !FactsComplete(fm.GlobalFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		if !FactsComplete(elseOut) {
			fm.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		// MergeFacts clears GlobalFacts on incomplete mid-join — fail closed sticky
		_ = MergeFacts(&fm.GlobalFacts, elseOut)
		if !FactsComplete(fm.GlobalFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
			return
		}
	}
}
