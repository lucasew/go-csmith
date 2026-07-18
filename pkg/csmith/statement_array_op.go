// Upstream: StatementArrayOp.cpp (simplified array-loop statement).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomArrayOp selects/creates an array and emits a simple counted for-loop
// that writes one itemized element (subset of StatementArrayOp).
func MakeRandomArrayOp(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg CGContext,
) Stmt {
	if vs == nil || r == nil {
		return Stmt{Kind: StmtArrayOp}
	}
	av := vs.SelectArray(r, cg)
	if av == nil {
		return Stmt{Kind: StmtArrayOp}
	}
	// Itemize for a concrete element access
	item := av.Itemize(r)
	// Body under IN_LOOP: assign to itemized element
	bodyCG := cg
	bodyCG.Flags |= 2 // IN_LOOP
	// Build a tight loop using first dimension as bound if available
	bound := 1
	if len(av.Sizes) > 0 {
		bound = av.Sizes[0]
	}
	// loop control variable
	iv := vs.SelectLoopCtrlVar(r, cg, nil)
	if iv == nil {
		iv = vs.GenerateNewGlobal(AccessWrite, cg, GetIntType(), nil, r)
	}
	lc := &LoopControl{
		IV:     iv,
		InitN:  0,
		LimitN: bound,
		IncrN:  1,
		TestOp: BinCmpLt,
		IncrOp: AssignAdd,
	}
	// body: one assign item = expr (use item as conceptual Lhs name via Expr on a synthetic)
	// We store ArrayAccess on Stmt for emit
	rhs := MakeRandomExpression(r, opts, tables, vs, bodyCG, av.Type, nil, true, false, MaxTermTypes, cg.ExprDepth)
	if rhs == nil {
		rhs = MakeRandomExpression(r, opts, tables, vs, bodyCG, av.Type, nil, true, false, TermConstant, cg.ExprDepth)
	}
	body := &Block{
		Func: cg.CurrentFunc,
		Stmts: []Stmt{{
			Kind:       StmtAssign,
			LhsVar:     &item.Variable, // name is array name; emit uses ArrayAccess
			Expr:       rhs,
			AssignOp:   AssignSimple,
			ArrayAccess: item.OutputAccess(),
		}},
	}
	_ = probs
	_ = tables
	_ = stmtTab
	return Stmt{
		Kind:        StmtArrayOp,
		Loop:        lc,
		Then:        body,
		ArrayAccess: item.OutputAccess(),
	}
}
