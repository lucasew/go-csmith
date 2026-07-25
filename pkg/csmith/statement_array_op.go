// Upstream: StatementArrayOp.cpp (make_random, make_random_array_init, make_random_iter_ctrl).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomIterCtrl mirrors StatementArrayOp::make_random_iter_ctrl.
// StatementArrayOp.cpp:64–70 — pure_rnd flip for init 0 or upto(size); incr 1 or upto(size)+1.
// Non-Sess MakeRandomIterCtrl deleted — pass run bag or testAmbientSession explicitly.

// MakeRandomIterCtrlSess is MakeRandomIterCtrl with explicit session residual sticky.
func MakeRandomIterCtrlSess(s *Session, r *Rng, size int) (init, incr int) {
	// StatementArrayOp.cpp:64–70 — pure_rnd_upto(size); sticky no invent incr=1 without RNG
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return 0, 0
	}
	if size < 1 {
		return 0, 0
	}
	if r.RndFlipcoinSess(s, 50) {
		init = 0
	} else {
		init = int(r.RndUptoSess(s, uint32(size)))
	}
	if r.RndFlipcoinSess(s, 50) {
		incr = 1
	} else {
		incr = int(r.RndUptoSess(s, uint32(size))) + 1
	}
	return init, incr
}

// AddVariableToSet mirrors add_variable_to_set — append if not already present.
// Variable* always live; nil v / nil set sticky no-op (no invent soft-skip past holes).
// Incomplete *set (nil hole) sticky no-op — no invent append/dup when
// IsVariableInSet is false only because membership cannot be decided past a hole.
// Non-Sess AddVariableToSet deleted — pass run bag or testAmbientSession explicitly.

// AddVariableToSetSess is AddVariableToSet with explicit session residual sticky.
func AddVariableToSetSess(s *Session, set *[]*Variable, v *Variable) {
	// set + Variable always live; sticky incomplete no invent soft no-op past hole
	if set == nil || v == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	// incomplete *set sticky no-op (no invent append/dup past membership hole)
	if !VariablesComplete(*set) {
		sessNoteError(s, ErrGeneric)
		return
	}
	if !IsVariableInSet(*set, v) {
		*set = append(*set, v)
	}
}

// CombineVariableSets mirrors combine_variable_sets.
// Variable* always live in sets; incomplete list fails closed IncompleteVariables
// (not bare nil invent empty-complete combined set via VariablesComplete(nil)).
// Non-Sess CombineVariableSets deleted — pass run bag or testAmbientSession explicitly.

// CombineVariableSetsSess is CombineVariableSets with explicit session residual sticky.
func CombineVariableSetsSess(s *Session, a, b []*Variable) []*Variable {
	if !VariablesComplete(a) || !VariablesComplete(b) {
		// incomplete lists fail closed sticky (no invent soft re-pick combined pool past holes)
		sessNoteError(s, ErrGeneric)
		return IncompleteVariables()
	}
	out := append([]*Variable(nil), a...)
	for _, v := range b {
		AddVariableToSetSess(s, &out, v)
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
	// StatementArrayOp always has VS + RNG + CG; sticky no invent array-op shell without them
	if vs == nil || r == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	// incomplete ambient fails closed sticky (no invent array-op / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	// StatementArrayOp.cpp:77–80 — rnd_flipcoin(5); ERROR_GUARD
	aryInit := r.RndFlipcoinSess(sessFromCG(cg), 5)
	if hasErrCG(cg) {
		return Stmt{}
	}
	if aryInit {
		return MakeRandomArrayInit(r, opts, probs, vs, tables, stmtTab, cg)
	}
	// StatementFor::make_random_array_loop
	st := MakeRandomArrayLoop(r, opts, probs, vs, tables, stmtTab, cg)
	if st == nil || hasErrCG(cg) {
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
	// StatementFor array-loop always has RNG + VS + CG; sticky no invent shell without them
	if r == nil || vs == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent array loop / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// StatementFor.cpp:316–317 — rnd_upto(CGOptions::max_array_num_in_loop())
	// no soft invent 4 when option is 0 (Defaults already set MaxArrayNumInLoop=4)
	maxN := opts.MaxArrayNumInLoop
	if maxN < 0 {
		maxN = 0
	}
	n := int(r.RndUptoSess(sessFromCG(cg), uint32(maxN)))
	if hasErrCG(cg) {
		return nil
	}
	var mustReads, mustWrites []*Variable
	var avs []*ArrayVariable
	for i := 0; i < n; i++ {
		av := vs.SelectArray(r, *cg)
		// StatementFor.cpp:319–328 — select_array then always use av; ERROR_GUARD sticky
		// no soft invent fewer arrays by skipping nil (was continue + burn RndUpto)
		if hasErrCG(cg) || av == nil {
			return nil
		}
		avs = append(avs, av)
		// access: 0 = must read, 1 = must write, 2 = both
		access := int(r.RndUptoSess(sessFromCG(cg), 3))
		if hasErrCG(cg) {
			return nil
		}
		if access == 0 || access == 2 {
			AddVariableToSetSess(sessFromCG(cg), &mustReads, &av.Variable)
		}
		if access == 1 || access == 2 {
			AddVariableToSetSess(sessFromCG(cg), &mustWrites, &av.Variable)
		}
	}
	// StatementFor.cpp:331–345 — combine with existing directive
	var allMustReads, allMustWrites, noReads, noWrites []*Variable
	if cg.RW != nil {
		allMustReads = CombineVariableSetsSess(sessFromCG(cg), cg.RW.MustReadVars, mustReads)
		allMustWrites = CombineVariableSetsSess(sessFromCG(cg), cg.RW.MustWriteVars, mustWrites)
		// incomplete combine / existing No* lists fail closed sticky (no invent partial RW)
		if !VariablesComplete(allMustReads) || !VariablesComplete(allMustWrites) ||
			!VariablesComplete(cg.RW.NoReadVars) || !VariablesComplete(cg.RW.NoWriteVars) {
			noteErrCG(cg, ErrGeneric)
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
	loopCG := cg.CloneSubcontext()
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
	// StatementFor array-loop setup always has RNG + VS; sticky no invent empty pool without them
	if r == nil || vs == nil {
		noteErrCG(&cg, ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent array-loop setup past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(&cg, ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(&cg, ErrGeneric)
		return nil
	}
	maxN := opts.MaxArrayNumInLoop
	if maxN < 0 {
		maxN = 0
	}
	n := int(r.RndUptoSess(sessFromCG(&cg), uint32(maxN)))
	out := make([]*ArrayVariable, 0, n)
	for i := 0; i < n; i++ {
		av := vs.SelectArray(r, cg)
		// StatementFor.cpp:319–328 — no soft invent fewer arrays by skipping nil
		if hasErrCG(&cg) || av == nil {
			return nil
		}
		out = append(out, av)
		// access choice 0/1/2 burns RNG (must_read / must_write / both)
		_ = r.RndUptoSess(sessFromCG(&cg), 3)
		if hasErrCG(&cg) {
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
	// StatementArrayOp always has VS + RNG + CG; sticky no invent array-init shell without them
	if vs == nil || r == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	// incomplete ambient fails closed sticky before EffectStm clear (no invent array init)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	av := vs.SelectArray(r, *cg)
	// StatementArrayOp.cpp:90–91 — ERROR_GUARD after select_array
	if av == nil || hasErrCG(cg) {
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
	if av.IsVolatileSess(sessFromCG(cg)) {
		// residual ERROR sticky — no invent volCount past IsVolatile residual hole
		if hasErrCG(cg) {
			return Stmt{}
		}
		volCount++
	} else if hasErrCG(cg) {
		// residual ERROR sticky — no invent soft-continue non-vol past IsVolatile residual false
		return Stmt{}
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
			if iv == nil || hasErrCG(cg) {
				// ERROR_GUARD path
				break
			}
			// StatementArrayOp.cpp:112–115 — iv->type always live; float rejected
			// Type-nil sticky fail whole array-op (no invent OK-IV soft pool past hole)
			if iv.Type == nil {
				noteErrCG(cg, ErrGeneric)
				iv = nil
				break
			}
			if iv.Type.IsFloatSess(sessFromCG(cg)) {
				// residual ERROR sticky — no invent soft-continue then later IV past IsFloat hole
				if hasErrCG(cg) {
					iv = nil
					break
				}
				invalid[iv] = true
				continue
			}
			// residual ERROR sticky — no invent soft-continue non-float past IsFloat residual false path
			if hasErrCG(cg) {
				iv = nil
				break
			}
			if iv.IsVolatileSess(sessFromCG(cg)) {
				// residual ERROR sticky — no invent soft-continue past IsVolatile hole
				if hasErrCG(cg) {
					iv = nil
					break
				}
				volCount++
			} else if hasErrCG(cg) {
				// residual ERROR sticky — no invent soft-continue non-vol IV past IsVolatile residual false
				iv = nil
				break
			}
			// StatementArrayOp.cpp:118–123 — strict_volatile / ccomp packed / signed_char
			packed := opts.CComp && iv.IsPackedAggregateFieldVarSess(sessFromCG(cg))
			// residual ERROR sticky — no invent soft-continue past packed-field IR hole
			if hasErrCG(cg) {
				iv = nil
				break
			}
			if (opts.StrictVolatileRule && volCount > 1 && iv.IsVolatileSess(sessFromCG(cg))) ||
				packed ||
				(!opts.SignedCharIndex && iv.Type.IsSignedCharSess(sessFromCG(cg))) {
				// residual from second IsVolatile sticky
				if hasErrCG(cg) {
					iv = nil
					break
				}
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
		// StatementArrayOp.cpp:129–131 — read_indices + write_var; assert(read) sticky
		// no soft invent success when ReadIndices fails
		if !cg.ReadIndices(iv, facts) {
			for _, d := range dims {
				if d != nil && d.IV != nil {
					cg.RemoveIVBound(d.IV)
				}
			}
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return Stmt{}
		}
		cg.WriteVar(iv)
		// residual ERROR sticky — no invent soft-continue later dims past WriteVar residual
		if hasErrCG(cg) {
			for _, d := range dims {
				if d != nil && d.IV != nil {
					cg.RemoveIVBound(d.IV)
				}
			}
			return Stmt{}
		}
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
	// residual ERROR sticky — no invent soft-complete array-op past WriteVar residual
	if hasErrCG(cg) {
		for _, x := range dims {
			if x != nil && x.IV != nil {
				cg.RemoveIVBound(x.IV)
			}
		}
		return Stmt{}
	}

	// access with ctrl vars: a[i0][i1]… —
	// ArrayVariable.cpp:708–709 + StatementArrayOp.cpp:245–250 —
	// output_with_indices uses cvs[i]->Output (virtual). Soft invent used
	// IV.Name (bare get_actual_name) so itemized array IVs lost indices in the
	// body assign (seed-48: UP l_91[…][g_106[4]] vs GO …[g_106]; for-header was
	// already OutputC and matched g_106[4]).
	access := av.Name
	for _, d := range dims {
		// sticky no invent "a[]" / "[0]" for missing IV
		if d == nil || d.IV == nil {
			for _, x := range dims {
				if x != nil && x.IV != nil {
					cg.RemoveIVBound(x.IV)
				}
			}
			noteErrCG(cg, ErrGeneric)
			return Stmt{}
		}
		ivOut := d.IV.OutputCSess(sessFromCG(cg), false)
		// residual ERROR sticky — no invent soft-continue access past OutputC residual
		if hasErrCG(cg) {
			for _, x := range dims {
				if x != nil && x.IV != nil {
					cg.RemoveIVBound(x.IV)
				}
			}
			return Stmt{}
		}
		if ivOut == "" {
			for _, x := range dims {
				if x != nil && x.IV != nil {
					cg.RemoveIVBound(x.IV)
				}
			}
			noteErrCG(cg, ErrGeneric)
			return Stmt{}
		}
		access += "[" + ivOut + "]"
	}

	// StatementArrayOp.cpp:141 — get_current_block()->random_parent_block()
	// Block.cpp:295–308 — when CGOptions::global_variables(), blks starts with
	// nullptr (global init site), then self + ancestors. Do not pass allowGlobal=false
	// (seed-2 e217 was U n=2 vs upstream U n=3).
	blk := cg.CurrentBlock()
	if blk == nil {
		for _, d := range dims {
			if d != nil && d.IV != nil {
				cg.RemoveIVBound(d.IV)
			}
		}
		return Stmt{}
	}
	parent := blk.RandomParentBlockSess(sessFromCG(cg), r, opts.GlobalVariables)
	// nil parent is valid when allowGlobal (global init site). ERROR_GUARD only on RNG error.
	if hasErrCG(cg) {
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
			noteErrCG(cg, ErrGeneric)
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
			noteErrCG(cg, ErrGeneric)
			return Stmt{}
		}
	}

	// clear IV list (StatementArrayOp.cpp:154–156)
	for _, d := range dims {
		if d != nil && d.IV != nil {
			cg.RemoveIVBound(d.IV)
		}
	}

	// C++ StatementArrayOp with init_value has body==null — the element assign is
	// emitted inline in Output, not a Statement (no stm_id). IncompleteStmID here.
	// LhsVar = array_var: StatementArrayOp.cpp:237 uses array_var->is_aggregate()
	// for the constant-init tmp rewrite (seed 53: union U0 tmp = …; a[i] = tmp).
	innerBody := &Block{
		Func: cg.CurrentFunc,
		Stmts: []Stmt{{
			Kind:        StmtAssign,
			Expr:        rhs,
			AssignOp:    AssignSimple,
			ArrayAccess: access,
			LhsVar:      &av.Variable,
			StmID:       IncompleteStmID,
		}},
	}
	// nest fors: outermost first dim (StatementArrayOp::output_header).
	// StatementArrayOp.cpp:145–150 — ONE StatementArrayOp for all dimensions
	// (single Statement ctor / single stm_id). Nested StmtArrayOp shells are
	// Output-only (bare "{" between dims, StatementArrayOp.cpp:198–200). Soft
	// invent AllocStmID per dim shifted all later block ids (seed-7: UP 525 vs
	// GO 526 after 2D l_1254 array-init in func_41).
	// Expression ctor: body=0, init_value=e — store init on Expr so get_exprs
	// matches C++ (get_blocks stays empty for array-init shape).
	if len(dims) == 0 {
		// StatementArrayOp is a Statement; always has stm_id
		return Stmt{
			Kind: StmtArrayOp, ArrayAccess: access, Then: innerBody, Expr: rhs,
			LhsVar: &av.Variable, StmID: AllocStmIDSess(sessFromCG(cg)),
		}
	}
	// Single stm_id for the whole multi-dim StatementArrayOp (C++ one ctor).
	// Nested dim shells share that id for Output structure only — they are not
	// separate C++ Statement objects (no second AllocStmID).
	sid := AllocStmIDSess(sessFromCG(cg))
	// Innermost dim carries ArrayAccess + init_value + array_var (for is_aggregate).
	st := Stmt{
		Kind:        StmtArrayOp,
		Loop:        dims[len(dims)-1],
		Then:        innerBody,
		ArrayAccess: access,
		Expr:        rhs,
		LhsVar:      &av.Variable,
		StmID:       sid,
	}
	for i := len(dims) - 2; i >= 0; i-- {
		// keep init_value on outermost only (C++ one StatementArrayOp / one init_value)
		inner := st
		st = Stmt{
			Kind:  StmtArrayOp,
			Loop:  dims[i],
			Then:  &Block{Func: cg.CurrentFunc, Stmts: []Stmt{inner}},
			Expr:  rhs,
			StmID: sid, // same Statement::stm_id as C++ single object
		}
	}
	// map_stm_effect[sa] = effect_stm (StatementArrayOp.cpp:151)
	// StmID always allocated above; FM path always records (no invent soft-skip)
	// Incomplete EffectStm fails closed (no invent map record / create success)
	if cg.FM != nil {
		if StmIDUnset(st.StmID) || !EffectComplete(cg.EffectStm) {
			noteErrCG(cg, ErrGeneric)
			return Stmt{}
		}
		eff := cg.EffectStm.CloneSess(sessFromCG(cg))
		// residual ERROR sticky — no invent soft-map effect past IncompleteEffect Clone residual
		if hasErrCG(cg) {
			return Stmt{}
		}
		cg.FM.SetMapStmEffect(st.StmID, eff)
	}
	_ = probs
	return st
}
