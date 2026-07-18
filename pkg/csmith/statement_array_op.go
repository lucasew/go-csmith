// Upstream: StatementArrayOp.cpp (make_random, make_random_array_init, make_random_iter_ctrl).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomIterCtrl mirrors StatementArrayOp::make_random_iter_ctrl.
// StatementArrayOp.cpp:64–70 — pure_rnd flip for init 0 or upto(size); incr 1 or upto(size)+1.
func MakeRandomIterCtrl(r *Rng, size int) (init, incr int) {
	// StatementArrayOp.cpp:64–70 — pure_rnd_upto(size); no soft invent incr=1 without RNG/size
	if r == nil || size < 1 {
		return 0, 0
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

// AddVariableToSet mirrors add_variable_to_set — append if not already present.
func AddVariableToSet(set *[]*Variable, v *Variable) {
	if set == nil || v == nil {
		return
	}
	if !IsVariableInSet(*set, v) {
		*set = append(*set, v)
	}
}

// CombineVariableSets mirrors combine_variable_sets.
func CombineVariableSets(a, b []*Variable) []*Variable {
	out := append([]*Variable(nil), a...)
	for _, v := range b {
		AddVariableToSet(&out, v)
	}
	return out
}

// MakeRandomArrayOp mirrors StatementArrayOp::make_random.
// StatementArrayOp.cpp:75–82 — 5% array_init; else make_random_array_loop → for.
// cg is *CGContext (C++ CGContext&).
func MakeRandomArrayOp(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
) Stmt {
	if vs == nil || r == nil || cg == nil {
		return Stmt{Kind: StmtArrayOp}
	}
	// rnd_flipcoin(5) → make_random_array_init
	if r.RndFlipcoin(5) {
		return MakeRandomArrayInit(r, opts, probs, vs, tables, stmtTab, cg)
	}
	// StatementFor::make_random_array_loop
	st := MakeRandomArrayLoop(r, opts, probs, vs, tables, stmtTab, cg)
	if st == nil {
		return Stmt{Kind: StmtArrayOp}
	}
	return *st
}

// MakeRandomArrayLoop mirrors StatementFor::make_random_array_loop.
// StatementFor.cpp:314–347 — select arrays with access 0/1/2 → must_read/write;
// combine with existing RWDirective; make_random for under loop CGContext.
func MakeRandomArrayLoop(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
) *Stmt {
	if r == nil || vs == nil || cg == nil {
		return nil
	}
	// StatementFor.cpp:316–330
	maxN := opts.MaxArrayNumInLoop
	if maxN < 1 {
		maxN = 4
	}
	n := int(r.RndUpto(uint32(maxN)))
	var mustReads, mustWrites []*Variable
	var avs []*ArrayVariable
	for i := 0; i < n; i++ {
		av := vs.SelectArray(r, *cg)
		if av == nil {
			// still burn access RNG for stream parity when select returns nil? upstream always gets av
			_ = r.RndUpto(3)
			continue
		}
		avs = append(avs, av)
		// access: 0 = must read, 1 = must write, 2 = both
		access := int(r.RndUpto(3))
		if access == 0 || access == 2 {
			AddVariableToSet(&mustReads, &av.Variable)
		}
		if access == 1 || access == 2 {
			AddVariableToSet(&mustWrites, &av.Variable)
		}
	}
	// StatementFor.cpp:331–345 — combine with existing directive
	var allMustReads, allMustWrites, noReads, noWrites []*Variable
	if cg.RW != nil {
		allMustReads = CombineVariableSets(cg.RW.MustReadVars, mustReads)
		allMustWrites = CombineVariableSets(cg.RW.MustWriteVars, mustWrites)
		noReads = append([]*Variable(nil), cg.RW.NoReadVars...)
		noWrites = append([]*Variable(nil), cg.RW.NoWriteVars...)
	} else {
		allMustReads = mustReads
		allMustWrites = mustWrites
	}
	rwd := &RWDirective{
		NoReadVars:    noReads,
		NoWriteVars:   noWrites,
		MustReadVars:  allMustReads,
		MustWriteVars: allMustWrites,
	}
	// CGContext(loop, &rwd, nullptr, 0) — no outer IV
	loopCG := *cg
	loopCG.RW = rwd
	loopCG.MustUseArrays = avs
	st := MakeRandomFor(r, opts, probs, vs, tables, stmtTab, &loopCG)
	if st == nil {
		return nil
	}
	// mark body as in_array_loop (Block::in_array_loop) for goto restrictions
	if st.Then != nil {
		st.Then.InArrayLoop = true
	}
	// array-op kind for emission/distribution tracking when used as array op
	st.Kind = StmtArrayOp
	return st
}

// MakeRandomArrayLoopSetup selects arrays for array-loop (test helper / inventory).
// Prefer MakeRandomArrayLoop for full must_read/write directive wiring.
// StatementFor.cpp:314–330 selection half.
func MakeRandomArrayLoopSetup(r *Rng, opts Options, vs *VariableSelector, cg CGContext) []*ArrayVariable {
	if r == nil || vs == nil {
		return nil
	}
	maxN := opts.MaxArrayNumInLoop
	if maxN < 1 {
		maxN = 4
	}
	n := int(r.RndUpto(uint32(maxN)))
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
// StatementArrayOp.cpp:85–155 — select array; per-dim IV (init 0, incr 1) with
// float/volatile/packed/signed-char filters; make_init_value RHS; update facts.
// cg is *CGContext (C++ CGContext&).
func MakeRandomArrayInit(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
) Stmt {
	_ = stmtTab
	_ = tables
	if vs == nil || r == nil || cg == nil {
		return Stmt{Kind: StmtArrayOp}
	}
	av := vs.SelectArray(r, *cg)
	if av == nil {
		return Stmt{Kind: StmtArrayOp}
	}
	// StatementArrayOp.cpp:103 — get_dimension(); no soft invent size 1
	if len(av.Sizes) == 0 {
		return Stmt{Kind: StmtArrayOp}
	}
	// StatementArrayOp.cpp:92–93, 100 — clear effect_stm
	cg.EffectStm = EmptyEffect()

	// StatementArrayOp.cpp:103–136 — per-dimension ctrl vars (do-while SelectLoopCtrlVar)
	var dims []*LoopControl
	invalid := map[*Variable]bool{}
	volCount := 0
	if av.IsVolatile() {
		volCount++
	}
	var facts []*FactPointTo
	if cg.FM != nil {
		facts = cg.FM.GlobalFacts
	}
	for di, size := range av.Sizes {
		// StatementArrayOp.cpp:106–107 — inits 0, incrs 1 (not random iter_ctrl)
		initN, incrN := 0, 1
		var iv *Variable
		// StatementArrayOp.cpp:110–126 — do { SelectLoopCtrlVar } while filters
		// C++ loops until break; SelectLoopCtrlVar creates on miss (no soft invent)
		for tries := 0; tries < 256; tries++ {
			iv = vs.SelectLoopCtrlVar(r, *cg, invalid)
			if iv == nil || HasError() {
				// ERROR_GUARD path
				break
			}
			// float IV rejected (StatementArrayOp.cpp:112–115)
			if iv.Type != nil && iv.Type.IsFloat() {
				invalid[iv] = true
				continue
			}
			if iv.IsVolatile() {
				volCount++
			}
			// StatementArrayOp.cpp:118–123 — strict_volatile / ccomp packed / signed_char
			if (opts.StrictVolatileRule && volCount > 1 && iv.IsVolatile()) ||
				(opts.CComp && iv.IsPackedAggregateFieldVar()) ||
				(!opts.SignedCharIndex && iv.Type != nil && iv.Type.IsSignedChar()) {
				invalid[iv] = true
				continue
			}
			break
		}
		// StatementArrayOp.cpp:128 — cvs.push_back(cv); cv must be live
		if iv == nil {
			return Stmt{Kind: StmtArrayOp}
		}
		invalid[iv] = true
		// StatementArrayOp.cpp:129–131 — read_indices + write_var
		_ = cg.ReadIndices(iv, facts)
		cg.WriteVar(iv)
		// StatementArrayOp.cpp:134 — iv_bounds[cv] = size
		cg.AddIVBound(iv, size)
		_ = di
		dims = append(dims, &LoopControl{
			IV:       iv,
			InitN:    initN,
			LimitN:   size,
			IncrN:    incrN,
			TestOp:   BinCmpLt,
			IncrOp:   AssignAdd,
			SafeIncr: opts.SafeMath,
		})
	}
	// StatementArrayOp.cpp:137 — write_var(av)
	cg.WriteVar(&av.Variable)

	// access with ctrl vars: a[i0][i1]… (C++ always has cvs[i])
	access := av.Name
	for _, d := range dims {
		// no soft invent "[0]" for missing IV
		access += "[" + d.IV.Name + "]"
	}

	// StatementArrayOp.cpp:141–143 — make_init_value in random parent block
	var parent *Block
	if blk := cg.CurrentBlock(); blk != nil {
		parent = blk.RandomParentBlock(r, false)
		if parent == nil {
			parent = blk
		}
	}
	qfer := av.Qfer
	// StatementArrayOp.cpp:141–143 — make_init_value; assert(visit_facts) (no const soft-fallback)
	rhs := vs.MakeInitValue(AccessRead, *cg, av.Type, &qfer, parent, r)
	if rhs == nil {
		return Stmt{Kind: StmtArrayOp}
	}
	// StatementArrayOp.cpp:144 — assert(init->visit_facts(...))
	if cg.FM != nil {
		if !VisitFactsExpression(rhs, cg, opts) {
			return Stmt{Kind: StmtArrayOp}
		}
	}

	// StatementArrayOp.cpp:145–150 — StatementArrayOp + update_fact_for_assign
	if cg.FM != nil {
		// LHS is the collective array variable
		if cg.FM.UpdateFactForAssign(&av.Variable, 0, rhs) {
			if cg.CurrentFunc != nil {
				cg.CurrentFunc.FactChanged = true
			}
		}
	}

	// clear IV list (StatementArrayOp.cpp:154–156)
	for _, d := range dims {
		if d != nil && d.IV != nil {
			cg.RemoveIVBound(d.IV)
		}
	}

	innerBody := &Block{
		Func: cg.CurrentFunc,
		Stmts: []Stmt{{
			Kind:        StmtAssign,
			Expr:        rhs,
			AssignOp:    AssignSimple,
			ArrayAccess: access,
			LhsVar:      &av.Variable,
		}},
	}
	// nest fors: outermost first dim (StatementArrayOp::output_header)
	if len(dims) == 0 {
		return Stmt{Kind: StmtArrayOp, ArrayAccess: access, Then: innerBody}
	}
	st := Stmt{
		Kind:        StmtArrayOp,
		Loop:        dims[len(dims)-1],
		Then:        innerBody,
		ArrayAccess: access,
		StmID:       AllocStmID(),
	}
	for i := len(dims) - 2; i >= 0; i-- {
		st = Stmt{
			Kind:  StmtArrayOp,
			Loop:  dims[i],
			Then:  &Block{Func: cg.CurrentFunc, Stmts: []Stmt{st}},
			StmID: AllocStmID(),
		}
	}
	// map_stm_effect[sa] = effect_stm (StatementArrayOp.cpp:151)
	if cg.FM != nil && st.StmID > 0 {
		cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm.Clone())
	}
	_ = probs
	return st
}
