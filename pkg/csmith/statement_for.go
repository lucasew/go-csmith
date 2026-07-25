// Upstream: StatementFor.cpp (make_random_loop_control, make_iteration, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// LoopControl holds IR for a counting for-loop (init/test/incr).
// StatementFor.cpp make_iteration — numeric params plus full assign/test IR.
type LoopControl struct {
	IV     *Variable
	InitN  int
	LimitN int
	IncrN  int
	TestOp BinaryOp
	IncrOp AssignOp
	// SafeIncr: emit safe_add/sub rewrite for increment (avoid_signed_overflow).
	SafeIncr bool
	// Bound is array shortest-dim bound used for control (0 = non-array loop).
	// StatementFor.cpp:make_iteration bound out-param.
	Bound int
	// InitStmt is StatementAssign init (simple assign of InitN).
	InitStmt *Stmt
	// TestExpr is ExpressionFuncall comparing IV to LimitN.
	TestExpr *Expression
	// IncrStmt is StatementAssign for the increment.
	IncrStmt *Stmt
}

// MakeRandomLoopControl mirrors make_random_loop_control.
// StatementFor.cpp:61–113.
// Sticky ERROR_RETURN leaves partial values; callers must check HasError.
func MakeRandomLoopControl(r *Rng, opts Options, ivSigned bool) (init, limit, incr int, testOp BinaryOp, incrOp AssignOp) {
	// pure_rnd_* == rnd_* in random mode (ERROR_RETURN still honors sticky Error)
	// C++ always has RNG; sticky no invent fixed init/limit/incr when r nil
	if r == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if r.RndFlipcoin(50) {
		init = 0
	} else {
		init = int(r.RndUpto(60)) - 30
	}
	if ivSigned {
		limit = int(r.RndUpto(60)) - 30
	} else {
		limit = int(r.RndUpto(60)) + 1
	}
	tOps := []BinaryOp{BinCmpLt, BinCmpLe, BinCmpGt, BinCmpGe, BinCmpEq, BinCmpNe}
	testOp = tOps[r.RndUpto(uint32(len(tOps)))]
	// StatementFor.cpp:79 — ERROR_RETURN after test_op pick
	if sessHasError(nil) {
		return
	}

	if r.RndFlipcoin(50) {
		// StatementFor.cpp:82 — ERROR_RETURN after flip into +=/-= branch
		if sessHasError(nil) {
			return
		}
		incr = int(r.RndUpto(10))
		if testOp == BinCmpNe && incr > 1 {
			// avoid infinite loop: limit = (limit-init)/incr*incr + init
			if incr != 0 {
				limit = (limit-init)/incr*incr + init
			}
		}
		if limit >= init {
			incrOp = AssignAdd
		} else {
			incrOp = AssignSub
		}
		if incr == 0 {
			incr = 1
		}
		if opts.FastExecution && incr != 0 && (limit-init)%incr == 0 &&
			(testOp == BinCmpGe || testOp == BinCmpLe) {
			if incrOp == AssignAdd {
				limit++
			} else {
				limit--
			}
		}
	} else {
		// StatementFor.cpp:102 — ERROR_RETURN after flip into ++/-- branch
		if sessHasError(nil) {
			return
		}
		// ++/-- pre or post
		if (limit < init) || (limit == init && testOp == BinCmpGe) {
			if r.RndFlipcoin(50) {
				incrOp = AssignPreDecr
			} else {
				incrOp = AssignPostDecr
			}
		} else {
			if r.RndFlipcoin(50) {
				incrOp = AssignPreIncr
			} else {
				incrOp = AssignPostIncr
			}
		}
		if (incrOp == AssignPreIncr && !opts.PreIncrOperator) ||
			(incrOp == AssignPostIncr && !opts.PostIncrOperator) {
			incrOp = AssignAdd
		}
		if (incrOp == AssignPreDecr && !opts.PreDecrOperator) ||
			(incrOp == AssignPostDecr && !opts.PostDecrOperator) {
			incrOp = AssignSub
		}
		incr = 1
	}
	return init, limit, incr, testOp, incrOp
}

// MakeRandomArrayControl mirrors make_random_array_control.
// StatementFor.cpp:128–161 — bound is shortest dim-1; OOB via ArrayOOBProb;
// returns adjusted IV bound (out-param `bound` in C++).
func MakeRandomArrayControl(r *Rng, bound int, isSigned bool, oobProb int) (init, limit, incr int, testOp BinaryOp, incrOp AssignOp, outBound int) {
	return MakeRandomArrayControlSess(nil, r, bound, isSigned, oobProb)
}

// MakeRandomArrayControlSess is MakeRandomArrayControl with OOB bookkeeping on bag s.
func MakeRandomArrayControlSess(s *Session, r *Rng, bound int, isSigned bool, oobProb int) (init, limit, incr int, testOp BinaryOp, incrOp AssignOp, outBound int) {
	// StatementFor.cpp:128+ — pure_rnd_*; bound is unsigned in C++ (0 allowed after --size)
	// sticky no invent fixed array-loop control when RNG missing
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return 0, 0, 0, 0, 0, 0
	}
	if bound < 0 {
		return 0, 0, 0, 0, 0, 0
	}
	// StatementFor.cpp:133 — pure_rnd_flipcoin(array_oob_prob) (random mode == rnd)
	oob := r.RndFlipcoin(uint32(oobProb))
	if oob {
		// StatementFor.cpp:157–158 — Bookkeeper::oob_cnt++
		RecordOOBSess(s)
	}
	// StatementFor.cpp:134 —
	//   test_op = is_signed ? (rnd_flipcoin(50) ? eCmpLe : eCmpGe) : eCmpLe
	// flip true → Le, false → Ge (do not invert; seed-2 e364 was Ge path extra draws)
	if isSigned {
		if r.RndFlipcoin(50) {
			testOp = BinCmpLe
		} else {
			testOp = BinCmpGe
		}
	} else {
		testOp = BinCmpLe
	}
	if testOp == BinCmpLe {
		// StatementFor.cpp:135–146 — increment from near 0
		if oob {
			init = -1000
		} else if r.RndFlipcoin(50) {
			init = 0
		} else {
			// pure_rnd_upto(bound/2); RndUpto(0) returns 0 (no soft invent skip)
			half := bound / 2
			if half < 1 {
				init = 0
			} else {
				init = int(r.RndUpto(uint32(half)))
			}
		}
		limit = bound
		incrOp = AssignAdd
		if r.RndFlipcoin(50) {
			incr = 1
		} else {
			// pure_rnd_upto(bound/4); no soft invent q=1 when bound/4==0
			q := bound / 4
			if q < 1 {
				incr = 0
			} else {
				incr = int(r.RndUpto(uint32(q)))
			}
		}
		// StatementFor.cpp:144–145 — if (incr == 0) incr = 1
		if incr == 0 {
			incr = 1
		}
		// StatementFor.cpp:145 — bound = ((bound-init)/incr)*incr + init
		outBound = ((bound-init)/incr)*incr + init
	} else {
		// StatementFor.cpp:147–156 — decrement from near last index
		if r.RndFlipcoin(50) {
			init = bound
		} else {
			// pure_rnd_upto(bound/2); no soft invent skip rng when 0
			off := 0
			if bound/2 > 0 {
				off = int(r.RndUpto(uint32(bound / 2)))
			}
			init = bound - off
		}
		if oob {
			limit = -1000
		} else if r.RndFlipcoin(50) {
			limit = 0
		} else {
			if bound/2 > 0 {
				limit = int(r.RndUpto(uint32(bound / 2)))
			}
		}
		incrOp = AssignSub
		if r.RndFlipcoin(50) {
			incr = 1
		} else {
			q := bound / 4
			if q < 1 {
				incr = 0
			} else {
				incr = int(r.RndUpto(uint32(q)))
			}
		}
		if incr == 0 {
			incr = 1
		}
		// StatementFor.cpp:156 — bound = init
		outBound = init
	}
	if outBound < 0 {
		outBound = 0
	}
	return init, limit, incr, testOp, incrOp, outBound
}

// MakeIteration mirrors StatementFor::make_iteration.
// StatementFor.cpp:164–283 — SelectLoopCtrlVar; array or free control;
// StatementAssign init (SafeOpFlags binary), visit_facts; test binary;
// compound/simple incr assign.
func MakeIteration(r *Rng, opts Options, probs *Probabilities, vs *VariableSelector, cg *CGContext) *LoopControl {
	// StatementFor always has RNG + VS + CG; sticky no invent iteration shell without them
	if r == nil || vs == nil || cg == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// StatementFor.cpp:170–172 — assert(fm); assert(blk)
	// non-sticky soft re-pick without FactMgr / current block (sticky poisons for soft factory)
	if cg.FM == nil || cg.CurrentBlock() == nil {
		return nil
	}
	// incomplete ambient fails closed sticky before EffectStm clear (no invent soft re-pick)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if !FactsComplete(cg.FM.GlobalFacts) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// StatementFor.cpp:176 — clear effect_stm before select
	cg.EffectStm = EmptyEffect()

	// StatementFor.cpp:181–189 — do { SelectLoopCtrlVar; skip volatile } while (true)
	// C++ ERROR_GUARD(nullptr) on select fail; cap high toward infinite re-pick
	invalid := map[*Variable]bool{}
	var iv *Variable
	for tries := 0; tries < 256; tries++ {
		iv = vs.SelectLoopCtrlVar(r, *cg, invalid)
		if iv == nil || sessHasError(nil) {
			return nil
		}
		// reject volatile IVs (infinite-loop / SE issues)
		if iv.IsVolatile() {
			// residual ERROR sticky — no invent soft-continue past IsVolatile hole
			if sessHasError(nil) {
				return nil
			}
			invalid[iv] = true
			continue
		}
		// residual ERROR sticky — no invent soft-continue non-vol IV past IsVolatile residual false
		if sessHasError(nil) {
			return nil
		}
		// C++ isArray always ArrayVariable*; missing AsArray sticky on loop-ctrl pick
		// (no invent IV shell past broken array IR)
		if iv.IsArray && iv.AsArray == nil {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		break
	}
	if iv == nil {
		return nil
	}
	// StatementFor.cpp:191–194 — read_indices assert; write_var; read_var
	var facts []*FactPointTo
	if cg.FM != nil {
		facts = cg.FM.GlobalFacts
	}
	if !cg.ReadIndices(iv, facts) {
		// assert(read); no soft invent continue after failed index visit
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return nil
	}
	cg.WriteVar(iv)
	if sessHasError(nil) {
		return nil
	}
	cg.ReadVar(iv)
	if sessHasError(nil) {
		return nil
	}

	// StatementFor.cpp:222 — assert(var->type)
	if iv.Type == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	signed := iv.Type.IsSigned()
	// residual ERROR sticky — no invent for bounds past IsSigned residual hole
	if sessHasError(nil) {
		return nil
	}
	var initN, limitN, incrN int
	var testOp BinaryOp
	var incrOp AssignOp
	// array-loop path: must-use arrays (StatementFor.cpp:204–216)
	// StatementFor.cpp:204–208 — only rw_directive->find_must_use_arrays (deduped
	// must_read then must_write). Do not use MustUseArrays selection-order list:
	// that invents duplicates when SelectArray returns the same collective twice
	// (seed-2 e358: Go choose_ok_var n=2 vs upstream n=1 after itemize).
	bound := InvalidIVBound
	if cg.RW != nil {
		mustArr := cg.RW.FindMustUseArrays()
		// FindMustUseArrays nil = incomplete must-use lists (no invent empty)
		if mustArr == nil {
			// residual ERROR already sticky, or empty complete nil slice?
			// FindMustUseArrays returns nil only on sticky hole; empty is non-nil empty
			if sessHasError(nil) {
				return nil
			}
		}
		if len(mustArr) > 0 {
			// StatementFor.cpp:208–214 — choose_ok_var among must-use arrays; assert(av)
			// choose_ok_var itemizes collective arrays (burns per-dim RndUpto)
			arrVars := make([]*Variable, 0, len(mustArr))
			for _, av := range mustArr {
				if av == nil {
					sessNoteError(nil, ErrGeneric)
					return nil
				}
				arrVars = append(arrVars, &av.Variable)
			}
			pick := ChooseOKVar(r, arrVars)
			// StatementFor.cpp:210–211 — assert(av); library fail closed
			if pick == nil || pick.AsArray == nil {
				if !sessHasError(nil) {
					sessNoteError(nil, ErrGeneric)
				}
				return nil
			}
			// StatementFor.cpp:211–214 — min dimension of chosen array
			// (C++ bound starts as INVALID_BOUND max-unsigned; first size always taken)
			for _, sz := range pick.AsArray.Sizes {
				if bound == InvalidIVBound || sz < bound {
					bound = sz
				}
			}
		}
	}
	// StatementFor.cpp:218–227 — array_control adjusts bound; else INVALID_BOUND stays
	// StatementFor.cpp:218 — if (bound != INVALID_BOUND) even when size is 1 (--bound may be 0)
	arrayBound := bound != InvalidIVBound && bound > 0
	if arrayBound {
		// StatementFor.cpp:220–221 — make_random_array_control(--bound, …); no soft invent b=1
		b := bound - 1
		if b < 0 {
			b = 0
		}
		oob := 0
		if probs != nil {
			oob = probs.Single(PArrayOOBProb)
		} else {
			oob = opts.ArrayOOBProb
		}
		var outBound int
		initN, limitN, incrN, testOp, incrOp, outBound = MakeRandomArrayControlSess(cg.Sess, r, b, signed, oob)
		// C++ replaces bound with adjusted return value for IV bounds
		bound = outBound
	} else {
		// StatementFor.cpp:200 / 223–226 — leave bound = INVALID_BOUND
		bound = InvalidIVBound
		initN, limitN, incrN, testOp, incrOp = MakeRandomLoopControl(r, opts, signed)
		// StatementFor.cpp:79/82/102 ERROR_RETURN inside make_random_loop_control
		if sessHasError(nil) {
			return nil
		}
	}

	// --- build IR: init attach flags for incr (StatementFor.cpp:229–245) ---
	lhs := &Lhs{Var: iv, Type: iv.Type}
	cInit := MakeInt(initN)
	// StatementFor.cpp:229–230 — Constant::make_int; ERROR_GUARD(nullptr)
	if sessHasError(nil) {
		return nil
	}
	// SafeOpFlags::make_random_binary(var, var, var, sOpAssign, compound_to_binary(incr_op))
	// StatementFor.cpp:236–239 — not MakeRandomBinary (sOpBinary/BinAdd soft invent)
	incrBop, _ := incrOp.CompoundToBinaryOps()
	// when no compound map, incrBop is MaxBinaryOp (C++ MAX_BINARY_OP)
	flags1 := MakeRandomBinaryKind(r, opts, probs, iv.Type, iv.Type, iv.Type, SafeOpAssign, incrBop)
	if flags1 == nil || sessHasError(nil) {
		return nil
	}
	initSt := &Stmt{
		Kind:      StmtAssign,
		LhsVar:    iv,
		Lhs:       lhs,
		Expr:      &Expression{Term: TermConstant, Con: cInit, ExprType: GetIntType()},
		AssignOp:  AssignSimple,
		SafeFlags: flags1,
		StmID:     AllocStmIDSess(cgSess(cg)),
	}
	// init->visit_facts (StatementFor.cpp:244–245) — assert(visited) sticky
	if !VisitFactsStatementAssign(initSt, cg, opts) {
		// C++ assert(visited); sticky make_iteration failure (no invent soft re-pick past fail)
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return nil
	}

	// Bookkeeper::record_volatile_access read+write on IV (StatementFor.cpp:249–253)
	RecordVolatileAccessSess(cgSess(cg), iv, 0, false)
	RecordVolatileAccessSess(cgSess(cg), iv, 0, true)

	// test: FunctionInvocation::make_binary(test_op, ExpressionVariable(iv), limit)
	// StatementFor.cpp:255–263 — ERROR_GUARD on null (no soft bare Invocation)
	vExpr := &Expression{Term: TermVariable, Var: iv, ExprType: iv.Type}
	cLimit := &Expression{Term: TermConstant, Con: MakeInt(limitN), ExprType: GetIntType()}
	testFi := MakeBinary(r, opts, probs, *cg, testOp, vExpr, cLimit)
	if testFi == nil || sessHasError(nil) {
		return nil
	}
	testTy := testFi.GetType()
	// residual ERROR sticky — no invent for-test ExprType past GetType residual hole
	if sessHasError(nil) {
		return nil
	}
	testExpr := &Expression{Term: TermFunction, Invoke: testFi, ExprType: testTy}

	// incr assign (StatementFor.cpp:273–281)
	lhs1 := &Lhs{Var: iv, Type: iv.Type}
	cIncr := &Expression{Term: TermConstant, Con: MakeInt(incrN), ExprType: GetIntType()}
	var incrSt Stmt
	if arrayBound {
		// plain compound assign (no make_possible_compound)
		incrSt = Stmt{
			Kind: StmtAssign, LhsVar: iv, Lhs: lhs1, Expr: cIncr, AssignOp: incrOp, StmID: AllocStmIDSess(cgSess(cg)),
		}
	} else {
		// StatementAssign::make_possible_compound_assign
		incrSt = makePossibleCompoundAssign(*cg, opts, probs, r, iv.Type, lhs1, incrOp, cIncr, gensymFromVS(vs))
		if StmIDUnset(incrSt.StmID) {
			incrSt.StmID = AllocStmIDSess(cgSess(cg))
		}
	}

	safeIncr := false
	if opts.SafeMath {
		switch incrOp {
		case AssignAdd, AssignSub, AssignPreIncr, AssignPostIncr, AssignPreDecr, AssignPostDecr:
			safeIncr = true
		}
	}

	return &LoopControl{
		IV:       iv,
		InitN:    initN,
		LimitN:   limitN,
		IncrN:    incrN,
		TestOp:   testOp,
		IncrOp:   incrOp,
		SafeIncr: safeIncr,
		Bound:    bound,
		InitStmt: initSt,
		TestExpr: testExpr,
		IncrStmt: &incrSt,
	}
}

// MakeRandomFor mirrors StatementFor::make_random — iteration + body with IN_LOOP.
// StatementFor.cpp:287–308.
// cg is *CGContext (C++ CGContext&) so effect_stm clear and post_loop stick.
func MakeRandomFor(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
) *Stmt {
	// StatementFor.cpp nullptr factory sticky — nil (no invent Kind-only shell)
	// always has RNG + CGContext sticky
	if r == nil || cg == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// StatementFor.cpp:288–289 — assert(fm); non-sticky soft re-pick without FactMgr
	if cg.FM == nil {
		return nil
	}
	// incomplete ambient fails closed sticky before EffectStm clear (no invent soft re-pick)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if !FactsComplete(cg.FM.GlobalFacts) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// StatementFor.cpp:290 — clear per-statement effect before building for
	cg.EffectStm = EmptyEffect()

	lc := MakeIteration(r, opts, probs, vs, cg)
	// StatementFor.cpp:296 make_iteration null / ERROR paths → nullptr
	if lc == nil || sessHasError(nil) {
		return nil
	}
	// when SafeMath and compound add/sub incr, attach dummy flags for emit
	if opts.SafeMath {
		switch lc.IncrOp {
		case AssignAdd, AssignSub, AssignPreIncr, AssignPostIncr, AssignPreDecr, AssignPostDecr:
			// flags created at emit from IV type; mark via LoopControl.SafeIncr
			lc.SafeIncr = true
		}
	}
	// StatementFor.cpp:299–300 — record effect and facts before loop body
	// incomplete GlobalFacts fail closed (no invent cleaned pre-loop snapshot)
	if !FactsComplete(cg.FM.GlobalFacts) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	preEffect := cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-for past EffectStm Clone residual
	if sessHasError(nil) {
		return nil
	}
	if !EffectComplete(preEffect) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	preFacts := CloneFactSlice(cg.FM.GlobalFacts)
	// residual ERROR sticky — no invent soft-for past CloneFactSlice residual
	if sessHasError(nil) {
		return nil
	}
	// FactMgr.cpp set_fact_in / StatementFor.cpp:299–300 — pre_facts is full FactVec
	// including eUnionWrite; snapshot UnionFacts with point-to for post_loop restore.
	if !UnionFactsComplete(cg.FM.UnionFacts) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// Deep: pre_facts eUnionWrite must not alias live FactUnion objects that
	// body generation renews/joins (StatementFor.cpp:299–300 snapshot).
	preUnion := CloneUnionFactSliceDeep(cg.FM.UnionFacts)
	if sessHasError(nil) || !UnionFactsComplete(preUnion) {
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return nil
	}
	// body CGContext(cg, rw_directive, iv, bound) — StatementFor.cpp:302–303 /
	// CGContext.cpp:95–105. WithLoopBody: IN_LOOP, expr_depth=0, curr_rhs=nil,
	// empty effect_stm, shared effect_accum, iv_bounds[iv]=bound.
	// Incomplete parent accum fails closed (no invent body under incomplete shell)
	if cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	var iv *Variable
	bound := 0
	if lc.IV != nil {
		iv = lc.IV
		bound = lc.Bound
	}
	// StatementFor.cpp:302 — body_cg_context(cg, cg.rw_directive, iv, bound)
	bodyCG := cg.WithLoopBody(cg.RW, iv, bound)
	// bodyCG.EffectAccum aliases cg.EffectAccum (CGContext.cpp:101)
	body := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &bodyCG, true)
	// StatementFor.cpp:304 ERROR_GUARD_AND_DEL3 after body
	if sessHasError(nil) || body == nil {
		if lc.IV != nil {
			bodyCG.RemoveIVBound(lc.IV)
		}
		return nil
	}
	// StatementFor.cpp:447,470 — erase iv_bounds after body
	if lc.IV != nil {
		bodyCG.RemoveIVBound(lc.IV)
	}
	// post_loop_analysis (StatementFor.cpp:350–370)
	st := &Stmt{Kind: StmtFor, Loop: lc, Then: body, StmID: AllocStmIDSess(cgSess(cg))}
	postLoopAnalysis(cg.FM, st, body, preFacts, preUnion, preEffect, cg)
	// incomplete post-loop GlobalFacts / map_stm fail closed (no invent for success)
	if !FactsComplete(cg.FM.GlobalFacts) ||
		!EffectComplete(cg.FM.GetMapStmEffect(st.StmID)) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// no MergeEffects: body writes already hit the shared parent EffectAccum
	return st
}

// postLoopAnalysis mirrors StatementFor::post_loop_analysis.
// StatementFor.cpp:350–370 — body entry facts; must_return restores pre;
// break edges + merge_jump_facts; set_accumulated_effect_after_block(pre_effect).
// preUnion is the eUnionWrite partition of pre_facts FactVec (snapshotted with preFacts).
func postLoopAnalysis(fm *FactMgr, forSt *Stmt, body *Block, preFacts []*FactPointTo, preUnion []*FactUnion, preEffect Effect, cg *CGContext) {
	if fm == nil {
		return
	}
	// StatementFor.cpp:355 — body Block always live with stm_id after make
	// StmID 0 fails closed sticky (no invent keep prior GlobalFacts soft-skipping map_facts_in)
	if body == nil || StmIDUnset(body.StmID) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(nil, ErrGeneric)
		return
	}
	// StatementFor.cpp:355 — global_facts = map_facts_in[&body]
	// Full FactVec: point-to + eUnionWrite (AssignGlobalFactsFromMapIn).
	fm.AssignGlobalFactsFromMapIn(body.StmID)
	// residual ERROR sticky — no invent soft-must-return path past map_in assign residual
	if sessHasError(nil) || !FactsComplete(fm.GlobalFacts) || !UnionFactsComplete(fm.UnionFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return
	}
	// StatementFor.cpp:355 — map_facts_in[&body] is entry (no body locals).
	// If map_in was polluted with body LocalVars, break merge invents garbage
	// (FactMgr.cpp:575–579) because remove_loop_local already dropped them
	// (seed-7 for 640 / l_1402). Drop subjects only — match entry semantics.
	if len(body.LocalVars) > 0 {
		fm.GlobalFacts = DropFactSubjectsByVars(fm.GlobalFacts, body.LocalVars)
		fm.UnionFacts = DropUnionSubjectsByVars(fm.UnionFacts, body.LocalVars)
		if !FactsComplete(fm.GlobalFacts) || !UnionFactsComplete(fm.UnionFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return
		}
	}
	// StatementFor.cpp:355–359 — non-must_return keeps map_facts_in[body] as
	// global_facts (full FactVec after AssignGlobalFactsFromMapIn above). Soft invent
	// rewrote UnionFacts from preUnion (make_iteration snapshot) + makeup, which
	// undid fair post_creation find_fixed_point set_fact_in (self-back / sequential
	// last_written). C++ has no such restore: only must_return restores pre_facts.
	// Entry eUnionWrite pollution was fixed by installing map_in unions before FP
	// (Block::post_creation_analysis / block.go); do not re-clobber here.
	// preUnion is still used by the must_return path below.
	if body.MustReturn() {
		// residual ERROR sticky — no invent soft-restore pre-loop past MustReturn residual true
		if sessHasError(nil) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			return
		}
		// StatementFor.cpp:356–359 — loop never entered; restore pre-loop full FactVec
		if !FactsComplete(preFacts) || !UnionFactsComplete(preUnion) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			sessNoteError(nil, ErrGeneric)
			return
		}
		fm.RestoreFactsPair(preFacts, preUnion)
	} else if sessHasError(nil) {
		// residual ERROR sticky — no invent soft-continue break-merge past MustReturn residual false
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		return
	}
	// StatementFor.cpp:361–367 — forward edges from breaks + merge jump facts
	// GetMapFactsOut: breakID 0 Incomplete; incomplete mid-join wipe + stop
	// Full FactVec merge includes eUnionWrite (map_out union partition).
	if forSt != nil {
		for _, breakID := range body.BreakStmIDs {
			// create_cfg_edge(break, for-stmt, post_dest=true, back=false)
			fm.CreateCFGEdgeTo(breakID, nil, forSt.StmID, true, false)
			out := fm.GetMapFactsOut(breakID)
			outU := fm.GetMapUnionFactsOut(breakID)
			if !FactsComplete(out) || !FactsComplete(fm.GlobalFacts) ||
				!UnionFactsComplete(outU) || !UnionFactsComplete(fm.UnionFacts) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				sessNoteError(nil, ErrGeneric)
				return
			}
			if _, ok := tryMergeJumpFacts(&fm.GlobalFacts, out); !ok {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				sessNoteError(nil, ErrGeneric)
				return
			}
			// eUnionWrite half of merge_jump_facts on full FactVec
			if !mergeJumpUnionFacts(&fm.UnionFacts, outU) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				if !sessHasError(nil) {
					sessNoteError(nil, ErrGeneric)
				}
				return
			}
		}
	}
	// StatementFor.cpp:369 — set_accumulated_effect_after_block(pre_effect, &body, …)
	// for-stmt stm_id always live; incomplete body/pre effect fails closed sticky
	if cg != nil && forSt != nil {
		if StmIDUnset(forSt.StmID) {
			sessNoteError(nil, ErrGeneric)
			return
		}
		bodyEff := fm.GetMapStmEffect(body.StmID)
		if !EffectComplete(bodyEff) || !EffectComplete(preEffect) {
			sessNoteError(nil, ErrGeneric)
			return
		}
		SetAccumulatedEffectAfterBlock(forSt, bodyEff, cg, preEffect)
		if !EffectComplete(fm.GetMapStmEffect(forSt.StmID)) {
			sessNoteError(nil, ErrGeneric)
		}
	}
}

// forHeaderOutput emits "for (init; test; incr)" using full IR when present.
// StatementFor::Output / StatementAssign OutputAsExpr paths.
// Not used for StatementArrayOp (that uses arrayOpHeaderOutput with numeric inits).
func forHeaderOutput(lc *LoopControl) string {
	// StatementFor always has init/test/incr IR; incomplete sticky empty (no soft invent for(;;))
	if lc == nil || lc.IV == nil {
		if lc != nil && lc.IV == nil {
			sessNoteError(nil, ErrGeneric)
		}
		return ""
	}
	init := forInitOutput(lc)
	// residual ERROR sticky — no invent soft-continue test/incr past init residual
	if sessHasError(nil) {
		return ""
	}
	test := forTestOutput(lc)
	// residual ERROR sticky — no invent soft-continue incr past test residual
	if sessHasError(nil) {
		return ""
	}
	incr := forIncrOutput(lc)
	// residual ERROR sticky — no invent soft-continue header past incr residual
	if sessHasError(nil) {
		return ""
	}
	// StatementFor.cpp:408–414 — always live init/test/incr; sticky no invent empty segments
	if init == "" || test == "" || incr == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	return fmt.Sprintf("for (%s; %s; %s)", init, test, incr)
}

// arrayOpHeaderOutput mirrors StatementArrayOp::output_header one dimension.
// StatementArrayOp.cpp:194–220 — for (cv = init; cv < size; cv += incr)
// (ccomp: cv = cv + incr). Numeric inits/incrs/sizes are the C++ IR (not StatementAssign).
func arrayOpHeaderOutput(lc *LoopControl, opts Options) string {
	// StatementArrayOp always has live LoopControl + IV sticky
	if lc == nil || lc.IV == nil {
		if lc != nil && lc.IV == nil {
			sessNoteError(nil, ErrGeneric)
		}
		return ""
	}
	// StatementArrayOp.cpp:194–220 — cv->Output always live; sticky no invent for ( = 0; …)
	iv := lc.IV.OutputC()
	// residual ERROR sticky — no invent soft-continue header past OutputC residual
	if sessHasError(nil) {
		return ""
	}
	if iv == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString("for (")
	b.WriteString(iv)
	b.WriteString(" = ")
	b.WriteString(itoa(lc.InitN))
	b.WriteString("; ")
	b.WriteString(iv)
	// StatementArrayOp.cpp:207–208 — incr>0 → cv < size; else cv >= 0
	if lc.IncrN > 0 {
		b.WriteString(" < ")
		b.WriteString(itoa(lc.LimitN))
	} else {
		b.WriteString(" >= 0")
	}
	b.WriteString("; ")
	b.WriteString(iv)
	if opts.CComp {
		// StatementArrayOp.cpp:211–215 — avoid volatile +=
		b.WriteString(" = ")
		b.WriteString(iv)
		b.WriteString(" + ")
		b.WriteString(itoa(lc.IncrN))
	} else {
		b.WriteString(" += ")
		b.WriteString(itoa(lc.IncrN))
	}
	b.WriteString(")")
	return b.String()
}

func forInitOutput(lc *LoopControl) string {
	// StatementFor.cpp:408–410 — init->OutputAsExpr; always live StatementAssign
	// sticky no invent "iv = InitN" when InitStmt missing
	if lc == nil || lc.InitStmt == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	wrap := false
	if lc.IV != nil {
		wrap = lc.IV.UseVolRVal
	}
	out := OutputAssignAsExpr(lc.InitStmt, wrap)
	// residual ERROR sticky — no invent soft-empty init past OutputAssign residual
	if sessHasError(nil) {
		return ""
	}
	return out
}

func forTestOutput(lc *LoopControl) string {
	// StatementFor.cpp:412 — test.Output; sticky no invent "iv < LimitN" for missing test
	if lc == nil || lc.TestExpr == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	out := lc.TestExpr.Output()
	// residual ERROR sticky — no invent soft-empty test past Output residual
	if sessHasError(nil) {
		return ""
	}
	return out
}

// forIncrOutput emits for-loop increment via IncrStmt OutputAsExpr.
// StatementFor.cpp:414 — incr->OutputAsExpr; always live StatementAssign.
// sticky no invent iv+=IncrN / safe_* from LoopControl numbers when IncrStmt missing.
func forIncrOutput(lc *LoopControl) string {
	if lc == nil || lc.IncrStmt == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	wrap := false
	if lc.IV != nil {
		wrap = lc.IV.UseVolRVal
	}
	out := OutputAssignAsExpr(lc.IncrStmt, wrap)
	// residual ERROR sticky — no invent soft-empty incr past OutputAssign residual
	if sessHasError(nil) {
		return ""
	}
	return out
}
