// Upstream: StatementFor.cpp (make_random_loop_control, make_iteration, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "fmt"

// LoopControl holds IR for a counting for-loop (init/test/incr).
type LoopControl struct {
	IV     *Variable
	InitN  int
	LimitN int
	IncrN  int
	TestOp BinaryOp
	IncrOp AssignOp
	// SafeIncr: emit safe_add/sub rewrite for increment (avoid_signed_overflow).
	SafeIncr bool
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

// MakeIteration mirrors StatementFor::make_iteration without SafeOpFlags/visit_facts.
// StatementFor.cpp:164–283.
func MakeIteration(r *Rng, opts Options, probs *Probabilities, vs *VariableSelector, cg CGContext) *LoopControl {
	if r == nil || vs == nil {
		return nil
	}
	invalid := map[*Variable]bool{}
	var iv *Variable
	for tries := 0; tries < 32; tries++ {
		iv = vs.SelectLoopCtrlVar(r, cg, invalid)
		if iv == nil {
			return nil
		}
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
	initN, limitN, incrN, testOp, incrOp := MakeRandomLoopControl(r, opts, signed)
	_ = probs
	return &LoopControl{
		IV:     iv,
		InitN:  initN,
		LimitN: limitN,
		IncrN:  incrN,
		TestOp: testOp,
		IncrOp: incrOp,
	}
}

// MakeRandomFor mirrors StatementFor::make_random — iteration + body with IN_LOOP.
// StatementFor.cpp:287–308.
func MakeRandomFor(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg CGContext,
) *Stmt {
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
	bodyCG := cg
	bodyCG.Flags |= 2 // IN_LOOP
	body := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, bodyCG, true)
	return &Stmt{Kind: StmtFor, Loop: lc, Then: body}
}

// forIncrOutput emits for-loop increment (plain or safe_add rewrite).
// StatementAssign::OutputAsExpr safe path for eAddAssign / ePreIncr-ish.
func forIncrOutput(lc *LoopControl) string {
	if lc == nil || lc.IV == nil {
		return ""
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
