// Upstream: Expression::func_count / get_called_funcs;
// FunctionInvocation::has_uncertain_call(_recursive);
// Statement::get_called_funcs / get_direct_invocation / find_contained_labels;
// StatementIf::combine_branch_facts.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CollectCalledInvocationsExpr mirrors Expression::get_called_funcs for user calls.
// FunctionInvocation.cpp:369–381 — recurse args, then push user call.
func CollectCalledInvocationsExpr(e *Expression, out *[]*Invocation) {
	if e == nil || out == nil {
		return
	}
	switch e.Term {
	case TermFunction:
		if e.Invoke != nil {
			for _, a := range e.Invoke.Args {
				CollectCalledInvocationsExpr(a, out)
			}
			if e.Invoke.User != nil {
				*out = append(*out, e.Invoke)
			}
		}
	case TermCommaExpr:
		CollectCalledInvocationsExpr(e.CommaLHS, out)
		CollectCalledInvocationsExpr(e.CommaRHS, out)
	case TermAssignment:
		if e.Assign != nil {
			CollectCalledInvocationsStmt(e.Assign, out)
		}
	}
}

// CollectCalledInvocationsStmt mirrors Statement::get_called_funcs.
// Statement.cpp:748–762 — exprs + nested blocks.
func CollectCalledInvocationsStmt(st *Stmt, out *[]*Invocation) {
	if st == nil || out == nil {
		return
	}
	if st.Expr != nil {
		CollectCalledInvocationsExpr(st.Expr, out)
	}
	if st.Lhs != nil {
		// Lhs is not an Expression with calls
	}
	if st.Then != nil {
		CollectCalledInvocationsBlock(st.Then, out)
	}
	if st.Else != nil {
		CollectCalledInvocationsBlock(st.Else, out)
	}
}

// CollectCalledInvocationsBlock walks all statements in a block.
func CollectCalledInvocationsBlock(b *Block, out *[]*Invocation) {
	if b == nil || out == nil {
		return
	}
	for i := range b.Stmts {
		CollectCalledInvocationsStmt(&b.Stmts[i], out)
	}
}

// FuncCount mirrors Expression::func_count.
// Expression.cpp:114–118.
func FuncCount(e *Expression) int {
	var calls []*Invocation
	CollectCalledInvocationsExpr(e, &calls)
	return len(calls)
}

// HasUncertainCall mirrors FunctionInvocation::has_uncertain_call.
// FunctionInvocation.cpp:383–394 — ≥2 params each containing a call.
func (fi *Invocation) HasUncertainCall() bool {
	if fi == nil {
		return false
	}
	cnt := 0
	for _, a := range fi.Args {
		if FuncCount(a) > 0 {
			cnt++
		}
	}
	return cnt >= 2
}

// HasUncertainCallRecursive mirrors FunctionInvocation::has_uncertain_call_recursive.
// FunctionInvocation.cpp:396–406.
func (fi *Invocation) HasUncertainCallRecursive() bool {
	if fi == nil {
		return false
	}
	for _, a := range fi.Args {
		if a != nil && a.Term == TermFunction && a.Invoke != nil {
			if a.Invoke.HasUncertainCallRecursive() {
				return true
			}
		}
	}
	return fi.HasUncertainCall()
}

// HasSimpleParams mirrors FunctionInvocation::has_simple_params.
// FunctionInvocation.cpp:408–416 — no TermFunction args.
func (fi *Invocation) HasSimpleParams() bool {
	if fi == nil {
		return true
	}
	for _, a := range fi.Args {
		if a != nil && a.Term == TermFunction {
			return false
		}
	}
	return true
}

// HasUncertainCallRecursiveExpr mirrors Expression::has_uncertain_call_recursive.
// ExpressionFuncall / Comma / Assign overrides; default false.
func HasUncertainCallRecursiveExpr(e *Expression) bool {
	if e == nil {
		return false
	}
	switch e.Term {
	case TermFunction:
		return e.Invoke != nil && e.Invoke.HasUncertainCallRecursive()
	case TermCommaExpr:
		return HasUncertainCallRecursiveExpr(e.CommaLHS) || HasUncertainCallRecursiveExpr(e.CommaRHS)
	case TermAssignment:
		return HasUncertainCallRecursiveStmt(e.Assign)
	default:
		return false
	}
}

// HasUncertainCallRecursiveStmt mirrors Statement::has_uncertain_call_recursive.
// Assign/Invoke/If via expr; default false.
func HasUncertainCallRecursiveStmt(st *Stmt) bool {
	if st == nil {
		return false
	}
	switch st.Kind {
	case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
		return HasUncertainCallRecursiveExpr(st.Expr)
	case StmtFor, StmtArrayOp:
		// init/test may carry calls via Loop — only body/expr in our model
		if HasUncertainCallRecursiveExpr(st.Expr) {
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
func GetDirectInvocation(st *Stmt) *Invocation {
	if st == nil {
		return nil
	}
	switch st.Kind {
	case StmtAssign, StmtInvoke, StmtIfElse:
		if st.Expr != nil && st.Expr.Term == TermFunction {
			return st.Expr.Invoke
		}
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
// When fm is nil, falls back to SourceLabel set during generation.
func FindContainedLabelsFM(st *Stmt, fm *FactMgr) []string {
	if st == nil {
		return nil
	}
	var labels []string
	findContainedLabels(st, &labels, fm)
	return labels
}

func findContainedLabels(st *Stmt, labels *[]string, fm *FactMgr) {
	if st == nil || labels == nil {
		return
	}
	// Statement.cpp:707–710 — find_jump_label()
	lab := ""
	if fm != nil && st.StmID > 0 {
		lab = FindJumpLabel(fm, st.StmID)
	}
	if lab == "" {
		lab = st.SourceLabel
	}
	if lab != "" {
		*labels = append(*labels, lab)
	}
	if st.Then != nil {
		for i := range st.Then.Stmts {
			findContainedLabels(&st.Then.Stmts[i], labels, fm)
		}
	}
	if st.Else != nil {
		for i := range st.Else.Stmts {
			findContainedLabels(&st.Else.Stmts[i], labels, fm)
		}
	}
}

// CombineBranchFacts mirrors StatementIf::combine_branch_facts.
// StatementIf.cpp:208–236 — merge then/else outs with must_return precision.
func CombineBranchFacts(st *Stmt, preFacts []*FactPointTo, fm *FactMgr) {
	if st == nil || fm == nil || st.Kind != StmtIfElse {
		return
	}
	var thenOut, elseOut []*FactPointTo
	if st.Then != nil && st.Then.StmID > 0 {
		thenOut = fm.MapFactsOut[st.Then.StmID]
	}
	if st.Else != nil && st.Else.StmID > 0 {
		elseOut = fm.MapFactsOut[st.Else.StmID]
	}
	// makeup new vars from branch outs into preFacts snapshot
	MakeupNewVarFacts(&preFacts, thenOut)
	MakeupNewVarFacts(&preFacts, elseOut)

	trueMust := st.Then != nil && st.Then.MustReturn()
	falseMust := st.Else != nil && st.Else.MustReturn()
	switch {
	case trueMust && falseMust:
		fm.GlobalFacts = CloneFactSlice(preFacts)
	case trueMust:
		fm.GlobalFacts = CloneFactSlice(elseOut)
	case falseMust:
		fm.GlobalFacts = CloneFactSlice(thenOut)
		if st.Else != nil && st.Else.StmID > 0 {
			if in, ok := fm.MapFactsIn[st.Else.StmID]; ok {
				MakeupNewVarFacts(&fm.GlobalFacts, in)
			}
		}
	default:
		fm.GlobalFacts = CloneFactSlice(thenOut)
		MergeFacts(&fm.GlobalFacts, elseOut)
	}
}
