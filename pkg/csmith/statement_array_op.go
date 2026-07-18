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
	// StatementFor::make_random_array_loop — select arrays, attach must-use, then for
	// StatementFor.cpp:314–347
	avs := MakeRandomArrayLoopSetup(r, opts, vs, cg)
	loopCG := cg
	loopCG.MustUseArrays = avs
	st := *MakeRandomFor(r, opts, probs, vs, tables, stmtTab, loopCG)
	// mark body as in_array_loop (Block::in_array_loop) for goto restrictions
	if st.Then != nil {
		st.Then.InArrayLoop = true
	}
	return st
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
// StatementArrayOp.cpp:85+ — per-dimension ctrl vars; nested for header; body assigns a[i0][i1]….
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
	if len(av.Sizes) == 0 {
		av.Sizes = []int{1}
	}
	// per-dim loop control (StatementArrayOp.cpp:105–120)
	var dims []*LoopControl
	invalid := map[*Variable]bool{}
	for _, size := range av.Sizes {
		init, incr := MakeRandomIterCtrl(r, size)
		iv := vs.SelectLoopCtrlVar(r, cg, invalid)
		if iv == nil {
			iv = vs.GenerateNewGlobal(AccessWrite, cg, GetIntType(), nil, r)
		}
		if iv != nil {
			invalid[iv] = true
		}
		dims = append(dims, &LoopControl{
			IV:       iv,
			InitN:    init,
			LimitN:   size,
			IncrN:    incr,
			TestOp:   BinCmpLt,
			IncrOp:   AssignAdd,
			SafeIncr: opts.SafeMath,
		})
	}
	bodyCG := cg.WithFlags(FlagInLoop)
	// access with ctrl vars: a[i0][i1]… (not constant itemize)
	access := av.Name
	for _, d := range dims {
		if d != nil && d.IV != nil {
			access += "[" + d.IV.Name + "]"
		} else {
			access += "[0]"
		}
	}
	rhs := MakeRandomExpression(r, opts, tables, vs, bodyCG, av.Type, nil, true, false, MaxTermTypes, cg.ExprDepth)
	if rhs == nil {
		rhs = MakeRandomExpression(r, opts, tables, vs, bodyCG, av.Type, nil, true, false, TermConstant, cg.ExprDepth)
	}
	// aggregate constant init may need tmp — emit direct assign for simple
	innerBody := &Block{
		Func: cg.CurrentFunc,
		Stmts: []Stmt{{
			Kind:        StmtAssign,
			Expr:        rhs,
			AssignOp:    AssignSimple,
			ArrayAccess: access,
		}},
	}
	// nest fors: outermost first dim (StatementArrayOp::output_header)
	// StmtArrayOp.Loop = outer; Then nests further ArrayOp fors or final body
	st := Stmt{
		Kind:        StmtArrayOp,
		Loop:        dims[len(dims)-1],
		Then:        innerBody,
		ArrayAccess: access,
	}
	for i := len(dims) - 2; i >= 0; i-- {
		st = Stmt{
			Kind: StmtArrayOp,
			Loop: dims[i],
			Then: &Block{Func: cg.CurrentFunc, Stmts: []Stmt{st}},
		}
	}
	_ = probs
	return st
}
