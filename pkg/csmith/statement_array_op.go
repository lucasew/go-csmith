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
// Variable* always live; nil v is not a list hole (no-op single arg).
// Incomplete *set (nil hole) fails closed no-op — no invent append/dup when
// IsVariableInSet is false only because membership cannot be decided past a hole.
func AddVariableToSet(set *[]*Variable, v *Variable) {
	if set == nil || v == nil {
		return
	}
	if !VariablesComplete(*set) {
		return
	}
	if !IsVariableInSet(*set, v) {
		*set = append(*set, v)
	}
}

// CombineVariableSets mirrors combine_variable_sets.
// Variable* always live in sets; incomplete list fails closed IncompleteVariables
// (not bare nil invent empty-complete combined set via VariablesComplete(nil)).
func CombineVariableSets(a, b []*Variable) []*Variable {
	if !VariablesComplete(a) || !VariablesComplete(b) {
		return IncompleteVariables()
	}
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
		return Stmt{}
	}
	// StatementArrayOp.cpp:77–80 — rnd_flipcoin(5); ERROR_GUARD
	aryInit := r.RndFlipcoin(5)
	if HasError() {
		return Stmt{}
	}
	if aryInit {
		return MakeRandomArrayInit(r, opts, probs, vs, tables, stmtTab, cg)
	}
	// StatementFor::make_random_array_loop
	st := MakeRandomArrayLoop(r, opts, probs, vs, tables, stmtTab, cg)
	if st == nil || HasError() {
		return Stmt{}
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
	// StatementFor.cpp:316–317 — rnd_upto(CGOptions::max_array_num_in_loop())
	// no soft invent 4 when option is 0 (Defaults already set MaxArrayNumInLoop=4)
	maxN := opts.MaxArrayNumInLoop
	if maxN < 0 {
		maxN = 0
	}
	n := int(r.RndUpto(uint32(maxN)))
	if HasError() {
		return nil
	}
	var mustReads, mustWrites []*Variable
	var avs []*ArrayVariable
	for i := 0; i < n; i++ {
		av := vs.SelectArray(r, *cg)
		// StatementFor.cpp:319–328 — select_array then always use av; ERROR_GUARD sticky
		// no soft invent fewer arrays by skipping nil (was continue + burn RndUpto)
		if HasError() || av == nil {
			return nil
		}
		avs = append(avs, av)
		// access: 0 = must read, 1 = must write, 2 = both
		access := int(r.RndUpto(3))
		if HasError() {
			return nil
		}
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
		// incomplete combine / existing No* lists fail closed (no invent partial RW)
		if !VariablesComplete(allMustReads) || !VariablesComplete(allMustWrites) ||
			!VariablesComplete(cg.RW.NoReadVars) || !VariablesComplete(cg.RW.NoWriteVars) {
			return nil
		}
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
	// StatementFor.cpp:344–346 — make_random → StatementFor* (not StatementArrayOp)
	st := MakeRandomFor(r, opts, probs, vs, tables, stmtTab, &loopCG)
	if st == nil {
		return nil
	}
	// mark body as in_array_loop (Block::in_array_loop) for goto restrictions
	// (C++ body Block::in_array_loop from iv_bounds during nested make_random)
	if st.Then != nil {
		st.Then.InArrayLoop = true
	}
	// Kind stays StmtFor — no soft invent StmtArrayOp shell over for IR
	return st
}

// MakeRandomArrayLoopSetup selects arrays for array-loop (test helper / inventory).
// Prefer MakeRandomArrayLoop for full must_read/write directive wiring.
// StatementFor.cpp:314–330 selection half.
// select_array always used; nil fails closed whole setup (no invent fewer arrays).
func MakeRandomArrayLoopSetup(r *Rng, opts Options, vs *VariableSelector, cg CGContext) []*ArrayVariable {
	if r == nil || vs == nil {
		return nil
	}
	maxN := opts.MaxArrayNumInLoop
	if maxN < 0 {
		maxN = 0
	}
	n := int(r.RndUpto(uint32(maxN)))
	out := make([]*ArrayVariable, 0, n)
	for i := 0; i < n; i++ {
		av := vs.SelectArray(r, cg)
		// StatementFor.cpp:319–328 — no soft invent fewer arrays by skipping nil
		if HasError() || av == nil {
			return nil
		}
		out = append(out, av)
		// access choice 0/1/2 burns RNG (must_read / must_write / both)
		_ = r.RndUpto(3)
		if HasError() {
			return nil
		}
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
		return Stmt{}
	}
	av := vs.SelectArray(r, *cg)
	// StatementArrayOp.cpp:90–91 — ERROR_GUARD after select_array
	if av == nil || HasError() {
		return Stmt{}
	}
	// StatementArrayOp.cpp:103 — get_dimension(); no soft invent size 1
	if len(av.Sizes) == 0 {
		return Stmt{}
	}
	// StatementArrayOp.cpp:100 — get_fact_mgr always live for visit/update
	if cg.FM == nil {
		return Stmt{}
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
			// drop partial IV bounds before fail
			for _, d := range dims {
				if d != nil && d.IV != nil {
					cg.RemoveIVBound(d.IV)
				}
			}
			return Stmt{}
		}
		invalid[iv] = true
		// StatementArrayOp.cpp:129–131 — read_indices + write_var; assert(read)
		// no soft invent success when ReadIndices fails
		if !cg.ReadIndices(iv, facts) {
			for _, d := range dims {
				if d != nil && d.IV != nil {
					cg.RemoveIVBound(d.IV)
				}
			}
			return Stmt{}
		}
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

	// StatementArrayOp.cpp:141 — get_current_block()->random_parent_block()
	// Block::random_parent_block always includes self when allow_global=false
	blk := cg.CurrentBlock()
	if blk == nil {
		for _, d := range dims {
			if d != nil && d.IV != nil {
				cg.RemoveIVBound(d.IV)
			}
		}
		return Stmt{}
	}
	parent := blk.RandomParentBlock(r, false)
	// no soft invent parent=blk when RandomParentBlock fails
	if parent == nil {
		for _, d := range dims {
			if d != nil && d.IV != nil {
				cg.RemoveIVBound(d.IV)
			}
		}
		return Stmt{}
	}
	qfer := av.Qfer
	// StatementArrayOp.cpp:141–143 — make_init_value; assert(visit_facts) (no const soft-fallback)
	rhs := vs.MakeInitValue(AccessRead, *cg, av.Type, &qfer, parent, r)
	if rhs == nil {
		for _, d := range dims {
			if d != nil && d.IV != nil {
				cg.RemoveIVBound(d.IV)
			}
		}
		return Stmt{}
	}
	// StatementArrayOp.cpp:144 — assert(init->visit_facts(...)); no soft invent skip
	if cg.FM != nil {
		if !VisitFactsExpression(rhs, cg, opts) {
			for _, d := range dims {
				if d != nil && d.IV != nil {
					cg.RemoveIVBound(d.IV)
				}
			}
			SetError(ErrGeneric)
			return Stmt{}
		}
	}

	// StatementArrayOp.cpp:145–150 — StatementArrayOp + update_fact_for_assign
	if cg.FM != nil {
		// LHS is the collective array variable
		// UpdateFactForAssign sets Func.FactChanged when lattice changes
		_ = cg.FM.UpdateFactForAssign(&av.Variable, 0, rhs)
		// incomplete assign must not invent array-op stmt with wiped GlobalFacts
		if !FactsComplete(cg.FM.GlobalFacts) {
			for _, d := range dims {
				if d != nil && d.IV != nil {
					cg.RemoveIVBound(d.IV)
				}
			}
			return Stmt{}
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
			// Statement base ctor always assigns stm_id (Statement.cpp:364–367)
			Kind:        StmtAssign,
			Expr:        rhs,
			AssignOp:    AssignSimple,
			ArrayAccess: access,
			LhsVar:      &av.Variable,
			StmID:       AllocStmID(),
		}},
	}
	// nest fors: outermost first dim (StatementArrayOp::output_header)
	if len(dims) == 0 {
		// StatementArrayOp is a Statement; always has stm_id
		return Stmt{Kind: StmtArrayOp, ArrayAccess: access, Then: innerBody, StmID: AllocStmID()}
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
	// StmID always allocated above; FM path always records (no invent soft-skip)
	if cg.FM != nil {
		if st.StmID <= 0 {
			return Stmt{}
		}
		cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm.Clone())
	}
	_ = probs
	return st
}
