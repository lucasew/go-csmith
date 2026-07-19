// Upstream: Expression::func_count / get_called_funcs;
// FunctionInvocation::has_uncertain_call(_recursive);
// Statement::get_called_funcs / get_direct_invocation / find_contained_labels;
// StatementIf::combine_branch_facts.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CollectCalledInvocationsExpr mirrors Expression::get_called_funcs for user calls.
// FunctionInvocation.cpp:369–381 — recurse args, then push user call.
// Incomplete IR clears *out (no invent partial call list past holes).
func CollectCalledInvocationsExpr(e *Expression, out *[]*Invocation) {
	if out == nil {
		return
	}
	if !collectCalledInvocationsExpr(e, out) {
		*out = nil
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
// Incomplete IR clears *out (no invent partial call list / skip for-test).
func CollectCalledInvocationsStmt(st *Stmt, out *[]*Invocation) {
	if out == nil {
		return
	}
	if !collectCalledInvocationsStmt(st, out) {
		*out = nil
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
func CollectCalledInvocationsBlock(b *Block, out *[]*Invocation) {
	if out == nil {
		return
	}
	if !collectCalledInvocationsBlock(b, out) {
		*out = nil
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
	if !collectCalledInvocationsExpr(e, &calls) {
		return -1
	}
	return len(calls)
}

// HasUncertainCall mirrors FunctionInvocation::has_uncertain_call.
// FunctionInvocation.cpp:383–394 — ≥2 params each containing a call.
// Nil invoke / nil arg / incomplete FuncCount fails closed true
// (no invent certain order / no-call).
func (fi *Invocation) HasUncertainCall() bool {
	if fi == nil {
		return true
	}
	cnt := 0
	for _, a := range fi.Args {
		if a == nil {
			return true
		}
		n := FuncCount(a)
		if n < 0 {
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
// Nil invoke / nil arg fails closed true (no invent skip hole as non-call).
func (fi *Invocation) HasUncertainCallRecursive() bool {
	if fi == nil {
		return true
	}
	for _, a := range fi.Args {
		if a == nil {
			return true
		}
		if a.Term == TermFunction {
			if a.Invoke == nil {
				return true
			}
			if a.Invoke.HasUncertainCallRecursive() {
				return true
			}
		}
	}
	return fi.HasUncertainCall()
}

// HasSimpleParams mirrors FunctionInvocation::has_simple_params.
// FunctionInvocation.cpp:408–416 — no TermFunction args.
// Nil arg fails closed false (no invent simple past hole).
func (fi *Invocation) HasSimpleParams() bool {
	// C++ always has live FunctionInvocation*; nil shell fails closed not-simple
	// (no invent simple-params success without args IR)
	if fi == nil {
		return false
	}
	for _, a := range fi.Args {
		// param_value[i] always live; nil hole fails closed not-simple
		if a == nil {
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
// Incomplete IR fails closed true (no invent "no uncertain call").
func HasUncertainCallRecursiveExpr(e *Expression) bool {
	if e == nil {
		return true
	}
	switch e.Term {
	case TermFunction:
		if e.Invoke == nil {
			return true
		}
		return e.Invoke.HasUncertainCallRecursive()
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			return true
		}
		return HasUncertainCallRecursiveExpr(e.CommaLHS) || HasUncertainCallRecursiveExpr(e.CommaRHS)
	case TermAssignment:
		if e.Assign == nil {
			return true
		}
		return HasUncertainCallRecursiveStmt(e.Assign)
	default:
		return false
	}
}

// HasUncertainCallRecursiveStmt mirrors Statement::has_uncertain_call_recursive.
// Assign/Invoke/If via expr; for via get_exprs test; default false.
// Incomplete for/expr fails closed true (no invent skip for-test calls).
func HasUncertainCallRecursiveStmt(st *Stmt) bool {
	if st == nil {
		return true
	}
	switch st.Kind {
	case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
		// C++ always has live get_exprs entries for these kinds
		if st.Expr == nil {
			return true
		}
		return HasUncertainCallRecursiveExpr(st.Expr)
	case StmtFor, StmtArrayOp:
		// StatementFor::get_exprs → test (not st.Expr)
		if st.Loop == nil || st.Loop.TestExpr == nil {
			return true
		}
		if HasUncertainCallRecursiveExpr(st.Loop.TestExpr) {
			return true
		}
		if st.Then != nil {
			for i := range st.Then.Stmts {
				if HasUncertainCallRecursiveStmt(&st.Then.Stmts[i]) {
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
	if st == nil {
		return nil
	}
	switch st.Kind {
	case StmtAssign, StmtIfElse:
		// StatementAssign/If always have live get_expr/get_test
		if st.Expr == nil {
			return &Invocation{Failed: true}
		}
		if st.Expr.Term != TermFunction {
			return nil
		}
		if st.Expr.Invoke == nil {
			return &Invocation{Failed: true}
		}
		return st.Expr.Invoke
	case StmtInvoke:
		// StatementExpr always has live get_invoke
		if st.Expr == nil || st.Expr.Term != TermFunction {
			return &Invocation{Failed: true}
		}
		if st.Expr.Invoke == nil {
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

// FindContainedLabelsFM mirrors Statement::find_contained_labels.
// Statement.cpp:706–720 — find_jump_label (CFG goto) then nested blocks.
// When fm is nil, falls back to SourceLabel set during generation (no CFG).
// With FactMgr: same as PreOutput — only CFG/registry labels; no invent SourceLabel
// when find_jump_label is empty. Incomplete CFGEdges holes fail closed (nil list).
func FindContainedLabelsFM(st *Stmt, fm *FactMgr) []string {
	if st == nil {
		return nil
	}
	// CFGEdge* always live; nil hole fails whole label collect (no invent partial)
	if fm != nil {
		for _, e := range fm.CFGEdges {
			if e == nil {
				return nil
			}
		}
	}
	var labels []string
	if !findContainedLabels(st, &labels, fm) {
		return nil
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
		if st.StmID > 0 {
			lab = FindJumpLabel(fm, st.StmID)
		}
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
func CombineBranchFacts(st *Stmt, preFacts []*FactPointTo, fm *FactMgr) {
	if st == nil || fm == nil || st.Kind != StmtIfElse {
		return
	}
	// StatementIf always has live if_true / if_false blocks after make_random
	// incomplete arms fail closed (no invent empty then/else via nil-out + FactsComplete)
	if st.Then == nil || st.Else == nil {
		fm.GlobalFacts = nil
		return
	}
	var thenOut, elseOut []*FactPointTo
	if st.Then.StmID > 0 {
		thenOut = fm.MapFactsOut[st.Then.StmID]
	}
	if st.Else.StmID > 0 {
		elseOut = fm.MapFactsOut[st.Else.StmID]
	}
	// Fact* always live in maps used for branch combine
	if !FactsComplete(preFacts) || !FactsComplete(thenOut) || !FactsComplete(elseOut) {
		fm.GlobalFacts = nil
		return
	}
	// makeup new vars from branch outs into preFacts snapshot
	// sequential: first failure must not invent second makeup from cleared empty
	if !MakeupNewVarFacts(&preFacts, thenOut) || !MakeupNewVarFacts(&preFacts, elseOut) {
		fm.GlobalFacts = nil
		return
	}

	trueMust := st.Then.MustReturn()
	falseMust := st.Else.MustReturn()
	switch {
	case trueMust && falseMust:
		fm.GlobalFacts = CloneFactSlice(preFacts)
	case trueMust:
		fm.GlobalFacts = CloneFactSlice(elseOut)
	case falseMust:
		fm.GlobalFacts = CloneFactSlice(thenOut)
		// StatementIf.cpp:227 — makeup_new_var_facts(outputs, map_facts_in[&if_false])
		// C++ map[] always; missing → empty makeup; incomplete fails closed
		if st.Else.StmID > 0 {
			in := fm.MapFactsIn[st.Else.StmID]
			if !FactsComplete(in) {
				fm.GlobalFacts = nil
				return
			}
			if !MakeupNewVarFacts(&fm.GlobalFacts, in) {
				fm.GlobalFacts = nil
				return
			}
		}
	default:
		fm.GlobalFacts = CloneFactSlice(thenOut)
		// incomplete clone or else merge must not invent partial global
		if thenOut != nil && fm.GlobalFacts == nil {
			return
		}
		if !FactsComplete(elseOut) {
			fm.GlobalFacts = nil
			return
		}
		// MergeFacts clears GlobalFacts on incomplete mid-join — fail closed
		_ = MergeFacts(&fm.GlobalFacts, elseOut)
		if !FactsComplete(fm.GlobalFacts) {
			fm.GlobalFacts = nil
			return
		}
	}
}
