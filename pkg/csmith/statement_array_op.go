// Upstream: StatementArrayOp.cpp (make_random, make_random_array_init, make_random_iter_ctrl).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomIterCtrl mirrors StatementArrayOp::make_random_iter_ctrl.
// StatementArrayOp.cpp:64–70 — pure_rnd flip for init 0 or upto(size); incr 1 or upto(size)+1.
func MakeRandomIterCtrl(r *Rng, size int) (init, incr int) {
	if r == nil || size < 1 {
		return 0, 1
	}
	if r.RndFlipcoin(50) {
		init = 0
	} else {
		init = int(r.RndUpto(uint32(size)))
	}
	if r.RndFlipcoin(50) {
		incr = 1
	} else {
		incr = int(r.RndUpto(uint32(size))) + 1
	}
	return init, incr
}

// MakeRandomArrayOp mirrors StatementArrayOp::make_random.
// StatementArrayOp.cpp:75–82 — 5% array_init; else make_random_array_loop → for.
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
	// rnd_flipcoin(5) → make_random_array_init
	if r.RndFlipcoin(5) {
		return MakeRandomArrayInit(r, opts, probs, vs, tables, stmtTab, cg)
	}
	// StatementFor::make_random_array_loop — select arrays then MakeRandomFor
	// (must_read/write directives deferred; still a real IN_LOOP for body)
	_ = MakeRandomArrayLoopSetup(r, opts, vs, cg)
	return *MakeRandomFor(r, opts, probs, vs, tables, stmtTab, cg)
}

// MakeRandomArrayLoopSetup mirrors make_random_array_loop array selection (side effects).
// StatementFor.cpp:314–348 — rnd_upto(max_array_num_in_loop) arrays via select_array.
func MakeRandomArrayLoopSetup(r *Rng, opts Options, vs *VariableSelector, cg CGContext) []*ArrayVariable {
	if r == nil || vs == nil {
		return nil
	}
	n := int(r.RndUpto(uint32(opts.MaxArrayNumInLoop)))
	var out []*ArrayVariable
	for i := 0; i < n; i++ {
		av := vs.SelectArray(r, cg)
		if av != nil {
			out = append(out, av)
		}
		// access choice 0/1/2 burns RNG (must_read / must_write / both)
		_ = r.RndUpto(3)
	}
	return out
}

// MakeRandomArrayInit mirrors StatementArrayOp::make_random_array_init.
// StatementArrayOp.cpp:85+ — select_array; per-dim loop ctrl; body assigns itemized cells.
func MakeRandomArrayInit(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg CGContext,
) Stmt {
	_ = stmtTab
	if vs == nil || r == nil {
		return Stmt{Kind: StmtArrayOp}
	}
	av := vs.SelectArray(r, cg)
	if av == nil {
		return Stmt{Kind: StmtArrayOp}
	}
	// first dimension drives our single for emit (multi-dim nested deferred)
	size := 1
	if len(av.Sizes) > 0 {
		size = av.Sizes[0]
	}
	init, incr := MakeRandomIterCtrl(r, size)
	iv := vs.SelectLoopCtrlVar(r, cg, nil)
	if iv == nil {
		iv = vs.GenerateNewGlobal(AccessWrite, cg, GetIntType(), nil, r)
	}
	// burn RNG for other dimensions' ctrl vars like upstream loop over dim
	for i := 1; i < len(av.Sizes); i++ {
		_ = vs.SelectLoopCtrlVar(r, cg, nil)
		_, _ = MakeRandomIterCtrl(r, av.Sizes[i])
	}
	lc := &LoopControl{
		IV:     iv,
		InitN:  init,
		LimitN: size,
		IncrN:  incr,
		TestOp: BinCmpLt,
		IncrOp: AssignAdd,
	}
	bodyCG := cg
	bodyCG.Flags |= 2 // IN_LOOP
	item := av.Itemize(r)
	rhs := MakeRandomExpression(r, opts, tables, vs, bodyCG, av.Type, nil, true, false, MaxTermTypes, cg.ExprDepth)
	if rhs == nil {
		rhs = MakeRandomExpression(r, opts, tables, vs, bodyCG, av.Type, nil, true, false, TermConstant, cg.ExprDepth)
	}
	body := &Block{
		Func: cg.CurrentFunc,
		Stmts: []Stmt{{
			Kind:        StmtAssign,
			LhsVar:      &item.Variable,
			Expr:        rhs,
			AssignOp:    AssignSimple,
			ArrayAccess: item.OutputAccess(),
		}},
	}
	_ = probs
	return Stmt{
		Kind:        StmtArrayOp,
		Loop:        lc,
		Then:        body,
		ArrayAccess: item.OutputAccess(),
	}
}
