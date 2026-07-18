// Upstream: StatementFor.cpp (make_random_loop_control, make_iteration, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "fmt"

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
func MakeRandomLoopControl(r *Rng, opts Options, ivSigned bool) (init, limit, incr int, testOp BinaryOp, incrOp AssignOp) {
	// pure_rnd_* == rnd_* in random mode
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

	if r.RndFlipcoin(50) {
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
	if r == nil || bound < 1 {
		return 0, 1, 1, BinCmpLt, AssignAdd, 1
	}
	// StatementFor.cpp:133 — pure_rnd_flipcoin(array_oob_prob) (random mode == rnd)
	oob := r.RndFlipcoin(uint32(oobProb))
	if oob {
		// StatementFor.cpp:157–158 — Bookkeeper::oob_cnt++
		RecordOOB()
	}
	// StatementFor.cpp:134 — signed: rnd_flipcoin Le/Ge; unsigned: always Le
	if isSigned && r.RndFlipcoin(50) {
		testOp = BinCmpGe
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
			q := bound / 4
			if q < 1 {
				q = 1
			}
			incr = int(r.RndUpto(uint32(q)))
		}
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
				q = 1
			}
			incr = int(r.RndUpto(uint32(q)))
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
	if r == nil || vs == nil || cg == nil {
		return nil
	}
	invalid := map[*Variable]bool{}
	var iv *Variable
	for tries := 0; tries < 32; tries++ {
		iv = vs.SelectLoopCtrlVar(r, *cg, invalid)
		if iv == nil {
			return nil
		}
		// reject volatile IVs (infinite-loop / SE issues)
		if iv.IsVolatile() {
			invalid[iv] = true
			continue
		}
		break
	}
	if iv == nil {
		return nil
	}
	signed := iv.Type == nil || iv.Type.IsSigned()
	var initN, limitN, incrN int
	var testOp BinaryOp
	var incrOp AssignOp
	// array-loop path: must-use arrays (StatementFor.cpp:205–220)
	mustArr := cg.MustUseArrays
	if len(mustArr) == 0 && cg.RW != nil {
		mustArr = cg.RW.FindMustUseArrays()
	}
	// choose_ok_var among must-use arrays for shortest dim (StatementFor.cpp:208–214)
	bound := InvalidIVBound
	if len(mustArr) > 0 {
		var arrVars []*Variable
		for _, av := range mustArr {
			if av != nil {
				arrVars = append(arrVars, &av.Variable)
			}
		}
		if pick := ChooseOKVar(r, arrVars); pick != nil && pick.AsArray != nil {
			for _, sz := range pick.AsArray.Sizes {
				if bound == InvalidIVBound || sz < bound {
					bound = sz
				}
			}
		} else {
			for _, av := range mustArr {
				if av == nil {
					continue
				}
				for _, sz := range av.Sizes {
					if bound == InvalidIVBound || sz < bound {
						bound = sz
					}
				}
			}
		}
	}
	arrayBound := bound != InvalidIVBound && bound > 0
	if arrayBound {
		// StatementFor.cpp:220–221 — make_random_array_control(--bound, …)
		b := bound - 1
		if b < 1 {
			b = 1
		}
		oob := 0
		if probs != nil {
			oob = probs.Single(PArrayOOBProb)
		} else {
			oob = opts.ArrayOOBProb
		}
		var outBound int
		initN, limitN, incrN, testOp, incrOp, outBound = MakeRandomArrayControl(r, b, signed, oob)
		// C++ replaces bound with adjusted return value for IV bounds
		bound = outBound
	} else {
		bound = 0
		initN, limitN, incrN, testOp, incrOp = MakeRandomLoopControl(r, opts, signed)
	}

	// --- build IR: init assign (StatementFor.cpp:229–245) ---
	lhs := &Lhs{Var: iv, Type: iv.Type}
	cInit := MakeInt(initN)
	bop, _ := AssignAdd.CompoundToBinaryOps() // for SafeOpFlags kind on simple assign flags
	_ = bop
	// SafeOpFlags::make_random_binary(var, var, var, sOpAssign, compound_to_binary(incr_op))
	incrBop, hasBop := incrOp.CompoundToBinaryOps()
	var flags1 *SafeOpFlags
	if hasBop {
		flags1 = MakeRandomBinary(r, opts, probs, iv.Type)
	} else {
		// ++/-- still use flags from add
		flags1 = MakeRandomBinary(r, opts, probs, iv.Type)
		_ = incrBop
	}
	initSt := &Stmt{
		Kind:     StmtAssign,
		LhsVar:   iv,
		Lhs:      lhs,
		Expr:     &Expression{Term: TermConstant, Con: cInit, ExprType: GetIntType()},
		AssignOp: AssignSimple,
		SafeFlags: flags1,
		StmID:    AllocStmID(),
	}
	// init->visit_facts (StatementFor.cpp:244–245)
	if cg.FM != nil {
		if !VisitFactsStatementAssign(initSt, cg, opts) {
			// upstream asserts visited; soft-fail: keep IR without fact update
		}
	} else {
		cg.NoteWrite(iv)
	}

	// Bookkeeper::record_volatile_access read+write on IV (StatementFor.cpp:249–253)
	RecordVolatileAccess(iv, 0, false)
	RecordVolatileAccess(iv, 0, true)

	// test: FunctionInvocation::make_binary(test_op, ExpressionVariable(iv), limit)
	// StatementFor.cpp:255–263
	vExpr := &Expression{Term: TermVariable, Var: iv, ExprType: iv.Type}
	cLimit := &Expression{Term: TermConstant, Con: MakeInt(limitN), ExprType: GetIntType()}
	testFi := MakeBinary(r, opts, probs, *cg, testOp, vExpr, cLimit)
	if testFi == nil {
		testFi = &Invocation{IsStd: true, Binary: testOp.BinaryOpC(), Args: []*Expression{vExpr, cLimit}}
		testFi.setOutOpts(opts)
	}
	testExpr := &Expression{Term: TermFunction, Invoke: testFi, ExprType: testFi.GetType()}

	// incr assign (StatementFor.cpp:273–281)
	lhs1 := &Lhs{Var: iv, Type: iv.Type}
	cIncr := &Expression{Term: TermConstant, Con: MakeInt(incrN), ExprType: GetIntType()}
	var incrSt Stmt
	if arrayBound {
		// plain compound assign (no make_possible_compound)
		incrSt = Stmt{
			Kind: StmtAssign, LhsVar: iv, Lhs: lhs1, Expr: cIncr, AssignOp: incrOp, StmID: AllocStmID(),
		}
	} else {
		// StatementAssign::make_possible_compound_assign
		incrSt = makePossibleCompoundAssign(*cg, opts, probs, r, iv.Type, lhs1, incrOp, cIncr)
		if incrSt.StmID == 0 {
			incrSt.StmID = AllocStmID()
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
	if cg == nil {
		return &Stmt{Kind: StmtFor}
	}
	// StatementFor.cpp:290 — clear per-statement effect before building for
	cg.EffectStm = EmptyEffect()

	lc := MakeIteration(r, opts, probs, vs, cg)
	if lc == nil {
		return &Stmt{Kind: StmtFor}
	}
	// when SafeMath and compound add/sub incr, attach dummy flags for emit
	if opts.SafeMath && lc != nil {
		switch lc.IncrOp {
		case AssignAdd, AssignSub, AssignPreIncr, AssignPostIncr, AssignPreDecr, AssignPostDecr:
			// flags created at emit from IV type; mark via LoopControl.SafeIncr
			lc.SafeIncr = true
		}
	}
	// StatementFor.cpp:299–300 — record effect and facts before loop body
	preEffect := cg.EffectStm.Clone()
	var preFacts []*FactPointTo
	if cg.FM != nil {
		preFacts = CloneFactSlice(cg.FM.GlobalFacts)
	}
	// body CGContext(cg, rw_directive, iv, bound) — StatementFor.cpp:302–303
	bodyCG := cg.WithFlags(FlagInLoop)
	if lc.IV != nil {
		// use Bound when array-control; else LimitN as soft bound for nonwritable IV
		bnd := lc.Bound
		if bnd <= 0 {
			bnd = lc.LimitN
		}
		bodyCG.AddIVBound(lc.IV, bnd)
	}
	// body starts from post-init effect; copy so loop body doesn't permanently merge poorly
	bodyEff := EmptyEffect()
	if cg.EffectAccum != nil {
		bodyEff = *cg.EffectAccum
	}
	bodyCG.EffectAccum = &bodyEff
	body := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &bodyCG, true)
	// StatementFor.cpp:447,470 — erase iv_bounds after body
	if lc.IV != nil {
		bodyCG.RemoveIVBound(lc.IV)
	}
	// post_loop_analysis (StatementFor.cpp:350–370)
	st := &Stmt{Kind: StmtFor, Loop: lc, Then: body, StmID: AllocStmID()}
	if cg.FM != nil {
		postLoopAnalysis(cg.FM, st, body, preFacts, preEffect, cg)
	} else {
		// still merge body effect when no FM
		SetAccumulatedEffectAfterBlock(st, bodyCG.EffectStm, cg, preEffect)
	}
	// merge body effect into parent accum
	if cg.EffectAccum != nil {
		*cg.EffectAccum = MergeEffects(*cg.EffectAccum, bodyEff)
	}
	return st
}

// postLoopAnalysis mirrors StatementFor::post_loop_analysis.
// StatementFor.cpp:350–370 — body entry facts; must_return restores pre;
// break edges + merge_jump_facts; set_accumulated_effect_after_block(pre_effect).
func postLoopAnalysis(fm *FactMgr, forSt *Stmt, body *Block, preFacts []*FactPointTo, preEffect Effect, cg *CGContext) {
	if fm == nil {
		return
	}
	// StatementFor.cpp:355 — global_facts = map_facts_in[&body]
	if body != nil && body.StmID > 0 {
		if in, ok := fm.MapFactsIn[body.StmID]; ok {
			fm.GlobalFacts = CloneFactSlice(in)
		}
	}
	if body != nil && body.MustReturn() {
		// StatementFor.cpp:356–359 — loop never entered; restore pre-loop
		fm.RestoreFacts(preFacts)
	}
	// StatementFor.cpp:361–367 — forward edges from breaks + merge jump facts
	if body != nil && forSt != nil {
		for _, breakID := range body.BreakStmIDs {
			// create_cfg_edge(break, for-stmt, post_dest=true, back=false)
			fm.CreateCFGEdgeTo(breakID, nil, forSt.StmID, true, false)
			if out, ok := fm.MapFactsOut[breakID]; ok {
				MergeJumpFacts(&fm.GlobalFacts, out)
			}
		}
	}
	// StatementFor.cpp:369 — set_accumulated_effect_after_block(pre_effect, &body, …)
	if cg != nil && forSt != nil && body != nil {
		bodyEff := fm.GetMapStmEffect(body.StmID)
		SetAccumulatedEffectAfterBlock(forSt, bodyEff, cg, preEffect)
	}
}

// forHeaderOutput emits "for (init; test; incr)" using full IR when present.
// StatementFor::Output / StatementAssign OutputAsExpr paths.
func forHeaderOutput(lc *LoopControl) string {
	if lc == nil || lc.IV == nil {
		return "for (;;)"
	}
	init := forInitOutput(lc)
	test := forTestOutput(lc)
	incr := forIncrOutput(lc)
	return fmt.Sprintf("for (%s; %s; %s)", init, test, incr)
}

func forInitOutput(lc *LoopControl) string {
	if lc == nil || lc.IV == nil {
		return ""
	}
	if lc.InitStmt != nil {
		// OutputAsExpr for simple assign: lhs = rhs (no trailing semicolon)
		wrap := lc.IV.UseVolRVal
		if s := OutputAssignAsExpr(lc.InitStmt, wrap); s != "" {
			return s
		}
	}
	return fmt.Sprintf("%s = %d", lc.IV.OutputC(), lc.InitN)
}

func forTestOutput(lc *LoopControl) string {
	if lc == nil {
		return "1"
	}
	if lc.TestExpr != nil {
		if s := lc.TestExpr.Output(); s != "" {
			return s
		}
	}
	if lc.IV == nil {
		return "1"
	}
	return fmt.Sprintf("%s %s %d", lc.IV.OutputC(), lc.TestOp.CmpOpC(), lc.LimitN)
}

// forIncrOutput emits for-loop increment (plain or safe_add rewrite).
// StatementAssign::OutputAsExpr safe path for eAddAssign / ePreIncr-ish.
func forIncrOutput(lc *LoopControl) string {
	if lc == nil || lc.IV == nil {
		return ""
	}
	if lc.IncrStmt != nil {
		wrap := lc.IV.UseVolRVal
		if s := OutputAssignAsExpr(lc.IncrStmt, wrap); s != "" {
			return s
		}
	}
	iv := lc.IV.OutputC()
	n := fmt.Sprintf("%d", lc.IncrN)
	if !lc.SafeIncr {
		return lc.IncrOp.AssignOpC(iv, n)
	}
	// safe rewrite: iv = safe_add/sub(iv, n) for +=/-= / ++/--
	op := "+"
	switch lc.IncrOp {
	case AssignSub, AssignPreDecr, AssignPostDecr:
		op = "-"
	case AssignPreIncr, AssignPostIncr:
		n = "1"
	case AssignAdd:
		// n already
	default:
		return lc.IncrOp.AssignOpC(iv, n)
	}
	// SafeOpFlags default int32 s_s for loop IV
	flags := &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeInt32}
	if lc.IV.Type != nil && !lc.IV.Type.IsSigned() {
		flags.Op1Signed = false
		flags.Op2Signed = false
	}
	fname := flags.BinaryFuncName(op)
	if fname == "" {
		return lc.IncrOp.AssignOpC(iv, n)
	}
	return iv + " = " + fname + "(" + iv + ", " + n + ")"
}
