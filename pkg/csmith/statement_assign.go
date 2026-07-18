// Upstream: StatementAssign.cpp (InitProbabilityTable, AssignOpsProbability, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// NewAssignOpsTable mirrors StatementAssign::InitProbabilityTable.
// StatementAssign.cpp:68–81.
func NewAssignOpsTable(opts Options) *DistributionTable {
	var t DistributionTable
	t.AddEntry(int(AssignSimple), 70)
	t.AddEntry(int(AssignBitAnd), 10)
	t.AddEntry(int(AssignBitXor), 10)
	t.AddEntry(int(AssignBitOr), 10)
	if opts.PreIncrOperator {
		t.AddEntry(int(AssignPreIncr), 5)
	}
	if opts.PreDecrOperator {
		t.AddEntry(int(AssignPreDecr), 5)
	}
	if opts.PostIncrOperator {
		t.AddEntry(int(AssignPostIncr), 5)
	}
	if opts.PostDecrOperator {
		t.AddEntry(int(AssignPostDecr), 5)
	}
	return &t
}

// AssignOpsProbability mirrors StatementAssign::AssignOpsProbability.
// StatementAssign.cpp:84–106.
func AssignOpsProbability(r *Rng, opts Options, table *DistributionTable, typ *Type) AssignOp {
	if !opts.CompoundAssignment {
		return AssignSimple
	}
	if typ != nil && (!typ.IsSimple() || typ.IsFloat()) {
		return AssignSimple
	}
	if table == nil {
		table = NewAssignOpsTable(opts)
	}
	f := NewVectorFilter(table)
	// signed ints: filter out ++/-- (upstream avoids for signed)
	if typ != nil && typ.IsSigned() {
		f.Add(int(AssignPreIncr)).Add(int(AssignPreDecr)).Add(int(AssignPostIncr)).Add(int(AssignPostDecr))
	}
	v := r.RndUptoFilter(uint32(f.MaxProb()), f)
	return AssignOp(f.Lookup(int(v)))
}

// SafeAssign mirrors StatementAssign::safe_assign — bit ops need no overflow wrap.
// StatementAssign.cpp:233–242.
func SafeAssign(op AssignOp) bool {
	switch op {
	case AssignBitAnd, AssignBitXor, AssignBitOr:
		return true
	default:
		return false
	}
}

// MakeRandomAssign mirrors StatementAssign::make_random.
// StatementAssign.cpp:111–231 — RHS then LHS contexts; merge effects; facts update.
func MakeRandomAssign(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	typ *Type,
) Stmt {
	ClearError()
	assignTab := NewAssignOpsTable(opts)
	op := AssignOpsProbability(r, opts, assignTab, typ)
	if typ == nil {
		// Type::SelectLType(!SE-free, op)
		noVol := !cg.EffectContext().IsSideEffectFree()
		typ = SelectLType(r, opts, probs, cg.Types, noVol, op)
		op = AssignOpsProbability(r, opts, assignTab, typ)
	}
	// StatementAssign.cpp:211–216 — float LHS forces simple if op doesn't work
	if typ != nil && typ.IsFloat() && !AssignOpWorksForFloat(op) {
		op = AssignSimple
	}

	// StatementAssign.cpp:131–140 — running effect + separate RHS/LHS accum
	runningEff := cg.EffectContext()
	rhsAccum := EmptyEffect()
	rhsCG := cg
	rhsCG.effectContext = runningEff
	rhsCG.EffectAccum = &rhsAccum
	rhsCG.EffectStm = EmptyEffect()

	var rhs *Expression
	qfer := NewCVQualifiers([]bool{false}, []bool{false})
	qfer.Wildcard = true

	if op.NeedNoRHS() {
		rhs = &Expression{Term: TermConstant, Con: MakeInt(1)}
		// standalone ++/--: any qualifier fits (wildcard)
	} else {
		// strict_volatile: skip volatile struct/union LHS types
		if opts.StrictVolatileRule && typ != nil && typ.IsVolatileStructUnion() {
			SetError(ErrGeneric)
			return Stmt{Kind: StmtAssign}
		}
		rhs = MakeRandomExpression(r, opts, tables, vs, rhsCG, typ, nil, false, false, MaxTermTypes, rhsCG.ExprDepth)
		if rhs == nil {
			rhs = MakeRandomExpression(r, opts, tables, vs, rhsCG, typ, nil, true, false, TermConstant, rhsCG.ExprDepth)
		}
		if rhs == nil {
			SetError(ErrGeneric)
			return Stmt{Kind: StmtAssign}
		}
		// derive qfer from expression (non-const for LHS)
		if q := expressionQualifiers(rhs); q != nil {
			qfer = *q
			// lhs should not be const
			if len(qfer.IsConsts) > 0 {
				qfer.IsConsts[0] = false
			}
		}
		// compound: fold RHS effects into running context; force non-vol LHS
		if op != AssignSimple {
			runningEff = runningEff.AddEffect(rhsAccum)
			if len(qfer.IsVolatiles) > 0 {
				qfer.IsVolatiles[0] = false
			}
		}
		if opts.StrictVolatileRule {
			runningEff = runningEff.AddEffect(rhsAccum)
			if qfer.IsVolatile() {
				if len(qfer.IsVolatiles) > 0 {
					qfer.IsVolatiles[0] = false
				}
			}
		}
	}
	// merge RHS effects into caller
	cg.MergeParamContext(rhsCG, true)

	// StatementAssign.cpp:183 — write_var_set(rhs_accum.get_lhs_write_vars())
	if lw := rhsAccum.LhsWriteVars(); len(lw) > 0 {
		runningEff = runningEff.WriteVarSet(lw)
	}

	// LHS context after RHS (StatementAssign.cpp:185–199)
	lhsAccum := EmptyEffect()
	lhsCG := cg
	lhsCG.effectContext = runningEff
	lhsCG.EffectAccum = &lhsAccum
	lhsCG.EffectStm = rhsCG.EffectStm
	lhsCG.CurrRHS = rhs

	// StatementAssign.cpp:195–200 — compound_assign = (op != simple); no_signed_overflow = need_no_rhs(op)
	compound := op != AssignSimple
	lhs := MakeRandomLhs(r, opts, probs, vs, lhsCG, typ, compound, op.NeedNoRHS())
	var lhsVar *Variable
	if lhs != nil {
		lhsVar = lhs.Var
	}
	if lhs == nil {
		SetError(ErrGeneric)
		return Stmt{Kind: StmtAssign}
	}

	// RHS cast to L type when needed (StatementAssign.cpp:207–208 — lang_cpp)
	if rhs != nil && typ != nil {
		rhs.CheckAndSetCastOpts(typ, opts)
	}
	if opts.CComp && lhsVar != nil && lhsVar.IsBitfield {
		if rhs != nil {
			rhs.CastType = typ
		}
	}
	// float op downgrade
	if lhsVar != nil && lhsVar.Type != nil && lhsVar.Type.IsFloat() && !AssignOpWorksForFloat(op) {
		op = AssignSimple
	}
	if rhs != nil && rhs.GetType() != nil && rhs.GetType().IsFloat() && !AssignOpWorksForFloat(op) {
		op = AssignSimple
	}

	// StatementAssign.cpp:218–223 — CompatibleChecker
	if CompatibleCheckExprVar(opts, lhsVar, rhs) {
		SetError(ErrCompatibleCheck)
		// regenerate RHS once as constant (practical recovery vs hard fail)
		rhs = &Expression{Term: TermConstant, Con: MakeRandom(typ, opts, r)}
		if rhs.Con != nil {
			rhs.CheckAndSetCastOpts(typ, opts)
		}
		ClearError()
	}

	cg.MergeParamContext(lhsCG, true)

	// StatementAssign.cpp:228 — make_possible_compound_assign (safe math flags/tmps)
	st := makePossibleCompoundAssign(cg, opts, probs, r, typ, lhs, op, rhs)
	lhsIndir := 0
	if st.Lhs != nil {
		lhsIndir = st.Lhs.IndirectLevel()
		if lhsIndir > 0 || (opts.WrapVolatiles && st.Lhs.IsVolatile()) {
			st.ArrayAccess = st.Lhs.Output(opts.WrapVolatiles)
		}
	}
	// FactMgr::update_fact_for_assign
	if cg.FM != nil && st.LhsVar != nil {
		if lhsIndir == 0 {
			cg.FM.UpdateFactForAssign(st.LhsVar, 0, st.Expr)
		}
		cg.NoteWrite(st.LhsVar)
	} else if st.LhsVar != nil {
		cg.NoteWrite(st.LhsVar)
	}
	return st
}

// MakeDummyFlags mirrors SafeOpFlags::make_dummy_flags.
// SafeOpFlags.cpp:61–63.
// MakeDummyFlags mirrors SafeOpFlags::make_dummy_flags.
// SafeOpFlags.cpp:61–63 — unsigned int8, is_func false.
func MakeDummyFlags() *SafeOpFlags {
	return &SafeOpFlags{Op1Signed: false, Op2Signed: false, IsFunc: false, Size: SafeInt8}
}

// makePossibleCompoundAssign mirrors StatementAssign::make_possible_compound_assign.
// StatementAssign.cpp:244–301 — attach SafeOpFlags + math_notmp temps for compound ops.
func makePossibleCompoundAssign(
	cg CGContext,
	opts Options,
	probs *Probabilities,
	r *Rng,
	typ *Type,
	lhs *Lhs,
	op AssignOp,
	rhs *Expression,
) Stmt {
	st := Stmt{Kind: StmtAssign, AssignOp: op, Expr: rhs, Lhs: lhs}
	if lhs != nil {
		st.LhsVar = lhs.Var
	}
	if !opts.SafeMath {
		return st
	}
	bop, ok := op.CompoundToBinaryOps()
	if !ok {
		// simple assign — no flags
		return st
	}
	// SafeAssign bit ops use dummy flags (OutputAsExpr still uses simple form)
	if SafeAssign(op) {
		st.SafeFlags = MakeDummyFlags()
		return st
	}
	// MakeRandomBinary for arithmetic/shift compound (sOpAssign)
	// SafeOpFlags.cpp:169–215 via make_random_binary(..., sOpAssign, bop)
	lt := typ
	if lhs != nil {
		if t := lhs.GetType(); t != nil {
			lt = t
		}
	}
	flags := MakeRandomBinaryKind(r, opts, probs, lt, lt, lt, SafeOpAssign, bop)
	st.SafeFlags = flags
	// math_notmp temps on current block
	if opts.MathNoTmp && flags != nil {
		if blk := cg.CurrentBlock(); blk != nil {
			st1 := EInt
			if t := flags.LHSType(); t != nil && t.IsSimple() {
				st1 = t.Simple()
			}
			st2 := st1
			if bop == BinLShift || bop == BinRShift {
				if t := flags.RHSType(); t != nil && t.IsSimple() {
					st2 = t.Simple()
				}
			}
			var sym *GenSym
			// gensym via VS not available; use nil → t_1 style falls back
			_ = sym
			st.Tmp1 = blk.CreateNewTmpVar(nil, st1)
			st.Tmp2 = blk.CreateNewTmpVar(nil, st2)
		}
	}
	return st
}

// OutputAssignSimple mirrors StatementAssign::OutputSimple.
// StatementAssign.cpp:515–537 — lhs op rhs or pre/post incr forms.
func OutputAssignSimple(st *Stmt, wrapVol bool) string {
	if st == nil {
		return ""
	}
	lhs := assignLhsText(st, wrapVol)
	if lhs == "" {
		return ""
	}
	rhs := "0"
	if st.Expr != nil {
		rhs = st.Expr.Output()
	}
	return st.AssignOp.AssignOpC(lhs, rhs)
}

func assignLhsText(st *Stmt, wrapVol bool) string {
	if st == nil {
		return ""
	}
	if st.ArrayAccess != "" {
		return st.ArrayAccess
	}
	if st.Lhs != nil {
		return st.Lhs.Output(wrapVol)
	}
	if st.LhsVar != nil {
		return st.LhsVar.OutputLhsC()
	}
	return ""
}

// OutputAssignAsExpr mirrors StatementAssign::OutputAsExpr.
// StatementAssign.cpp:542–625 — safe math rewrite for +=/-= when SafeFlags set.
func OutputAssignAsExpr(st *Stmt, wrapVol bool) string {
	return OutputAssignAsExprOpts(st, wrapVol, Defaults())
}

// OutputAssignAsExprOpts is OutputAsExpr with options for wrapper id filtering.
func OutputAssignAsExprOpts(st *Stmt, wrapVol bool, opts Options) string {
	if st == nil {
		return ""
	}
	lhs := assignLhsText(st, wrapVol)
	if lhs == "" {
		return ""
	}
	rhs := "0"
	if st.Expr != nil {
		rhs = st.Expr.Output()
	}
	// pre/post incr without safe flags
	if st.AssignOp.NeedNoRHS() && st.SafeFlags == nil {
		return st.AssignOp.AssignOpC(lhs, "")
	}
	// avoid_signed_overflow path when SafeFlags present
	if st.SafeFlags != nil {
		switch st.AssignOp {
		case AssignSimple, AssignBitAnd, AssignBitXor, AssignBitOr:
			// safe_assign ops / simple — OutputSimple form
			return OutputAssignSimple(st, wrapVol)
		case AssignPreIncr:
			return "++" + lhs
		case AssignPreDecr:
			return "--" + lhs
		case AssignPostIncr:
			return lhs + "++"
		case AssignPostDecr:
			return lhs + "--"
		case AssignAdd, AssignSub:
			bop, ok := st.AssignOp.CompoundToBinaryOps()
			if !ok {
				break
			}
			fname := st.SafeFlags.BinaryFuncName(bop.BinaryOpC())
			if fname == "" {
				break
			}
			id := SafeOpFlagsToID(fname)
			// don't use wrapper if filtered out by --safe-math-wrapper
			if !SafeMathWrapperAllowed(opts, id) {
				return OutputAssignSimple(st, wrapVol)
			}
			var b strings.Builder
			b.WriteString(lhs)
			b.WriteString(" = ")
			b.WriteString(fname)
			b.WriteString("(")
			if st.Tmp1 != "" {
				b.WriteString(st.Tmp1)
				b.WriteString(", ")
			}
			b.WriteString(lhs)
			b.WriteString(", ")
			if st.Tmp2 != "" {
				b.WriteString(st.Tmp2)
				b.WriteString(", ")
			}
			b.WriteString(rhs)
			if opts.IdentifyWrappers {
				b.WriteString(", ")
				b.WriteString(Int2Str(id))
			}
			b.WriteString(")")
			return b.String()
		default:
			// other compound: try generic safe rewrite
			if bop, ok := st.AssignOp.CompoundToBinaryOps(); ok {
				opStr := bop.BinaryOpC()
				fname := st.SafeFlags.BinaryFuncName(opStr)
				if fname != "" {
					id := SafeOpFlagsToID(fname)
					if !SafeMathWrapperAllowed(opts, id) {
						return OutputAssignSimple(st, wrapVol)
					}
					var b strings.Builder
					b.WriteString(lhs)
					b.WriteString(" = ")
					b.WriteString(fname)
					b.WriteString("(")
					if st.Tmp1 != "" {
						b.WriteString(st.Tmp1 + ", ")
					}
					b.WriteString(lhs + ", ")
					if st.Tmp2 != "" {
						b.WriteString(st.Tmp2 + ", ")
					}
					switch st.AssignOp {
					case AssignPreIncr, AssignPostIncr, AssignPreDecr, AssignPostDecr:
						if opts.MarkMutableConst {
							b.WriteString("(1)")
						} else {
							b.WriteString("1")
						}
					default:
						b.WriteString(rhs)
					}
					if opts.IdentifyWrappers {
						b.WriteString(", " + Int2Str(id))
					}
					b.WriteString(")")
					return b.String()
				}
			}
		}
	}
	return OutputAssignSimple(st, wrapVol)
}

// expressionQualifiers mirrors Expression::get_qualifiers for qfer seed.
// Uses Expression.GetQualifiers (ExpressionVariable/Assign/Funcall/Comma).
func expressionQualifiers(e *Expression) *CVQualifiers {
	if e == nil {
		return nil
	}
	q := e.GetQualifiers()
	// empty vectors → treat as no seed (match prior nil for bare constants)
	if len(q.IsConsts) == 0 && len(q.IsVolatiles) == 0 && !q.Wildcard {
		return nil
	}
	return &q
}

// VisitFactsExpression mirrors Expression::visit_facts dispatch by term.
// Constant always true; Variable/Lhs paths; comma sequential; assign delegates.
func VisitFactsExpression(e *Expression, cg *CGContext, opts Options) bool {
	if e == nil || cg == nil {
		return true
	}
	switch e.Term {
	case TermConstant:
		return true
	case TermVariable:
		return cg.VisitFactsExpressionVariable(e, opts)
	case TermCommaExpr:
		if !VisitFactsExpression(e.CommaLHS, cg, opts) {
			return false
		}
		return VisitFactsExpression(e.CommaRHS, cg, opts)
	case TermAssignment:
		if e.Assign == nil {
			return true
		}
		return VisitFactsStatementAssign(e.Assign, cg, opts)
	case TermFunction:
		if e.Invoke != nil {
			return VisitFactsInvocation(e.Invoke, cg, opts)
		}
		return true
	default:
		return true
	}
}

// VisitFactsInvocation mirrors FunctionInvocation::visit_facts.
// FunctionInvocation.cpp:502–555 — ordered params (unordered path available but
// upstream sets unordered=false); then user revisit when NeedsRevisit.
// Binary &&/|| use FunctionInvocationBinary::visit_facts short-circuit merge.
func VisitFactsInvocation(fi *Invocation, cg *CGContext, opts Options) bool {
	if fi == nil || cg == nil {
		return true
	}
	if fi.Failed {
		return false
	}
	// FunctionInvocationBinary.cpp:487–490 — ordered standard ops
	if fi.IsStd && !fi.IsUnary {
		if op, ok := BinaryOpFromString(fi.Binary); ok && IsOrderedBinary(op) {
			return VisitFactsBinaryOrdered(fi, cg, opts)
		}
	}
	// upstream: bool unordered = false; // has_uncertain_call();
	unordered := false
	isFuncCall := fi.User != nil
	if unordered {
		var facts []*FactPointTo
		if cg.FM != nil {
			facts = CloneFactSlice(cg.FM.GlobalFacts)
		}
		if !fi.VisitUnorderedParams(&facts, cg, opts) {
			return false
		}
		if cg.FM != nil {
			cg.FM.GlobalFacts = facts
		}
	} else {
		running := cg.EffectContext()
		for i, arg := range fi.Args {
			paramAccum := EmptyEffect()
			paramCG := *cg
			paramCG.effectContext = running
			paramCG.EffectAccum = &paramAccum
			if arg != nil && !VisitFactsExpression(arg, &paramCG, opts) {
				_ = i
				return false
			}
			running = running.AddEffect(paramAccum)
			// merge_param_context; include_lhs for std ops only
			cg.MergeParamContext(paramCG, !isFuncCall)
		}
	}
	if isFuncCall {
		// FunctionInvocation.cpp:530–551 — revisit user callee when DFA needed
		if fi.User.NeedsRevisit() && fi.User.Body != nil && cg.FM != nil {
			facts := CloneFactSlice(cg.FM.GlobalFacts)
			if !RevisitUserInvocation(fi, &facts, cg, opts) {
				return false
			}
			cg.FM.GlobalFacts = facts
			// fold feffect from accum during revisit
			if cg.InConflict(fi.User.FEffect) {
				return false
			}
			cg.AddVisibleEffectAt(fi.User.FEffect, cg.CurrentBlock())
		} else if fi.User.IsEffectKnown() {
			// static effect path (no fact/pointer change)
			if cg.InConflict(fi.User.FEffect) {
				return false
			}
			cg.AddVisibleEffectAt(fi.User.FEffect, cg.CurrentBlock())
			// also add_external_effect of feffect
			cg.AddExternalEffect(fi.User.FEffect)
		}
		// propagate fact_changed to caller (FunctionInvocation.cpp:96)
		if cg.CurrentFunc != nil && fi.User.FactChanged {
			cg.CurrentFunc.FactChanged = true
		}
	}
	return true
}

// VisitFactsStatementAssign mirrors StatementAssign::visit_facts.
// StatementAssign.cpp:358–390 — RHS first, compound folds RHS into LHS context.
func VisitFactsStatementAssign(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil || st.Kind != StmtAssign {
		return false
	}
	runningEff := cg.EffectContext()
	rhsAccum := EmptyEffect()
	rhsCG := *cg
	rhsCG.effectContext = runningEff
	rhsCG.EffectAccum = &rhsAccum
	// clone EffectStm start clean for sub-context? use copy of current stm
	rhsCG.EffectStm = cg.EffectStm

	if st.Expr != nil && !VisitFactsExpression(st.Expr, &rhsCG, opts) {
		return false
	}
	if st.AssignOp != AssignSimple {
		runningEff = runningEff.AddEffect(rhsAccum)
	}
	cg.MergeParamContext(rhsCG, true)

	lhsAccum := EmptyEffect()
	lhsCG := *cg
	lhsCG.effectContext = runningEff
	lhsCG.EffectAccum = &lhsAccum
	lhsCG.EffectStm = rhsCG.EffectStm
	lhsCG.CurrRHS = st.Expr

	if st.Lhs != nil {
		if !lhsCG.VisitFactsLhs(st.Lhs, opts) {
			return false
		}
	} else if st.LhsVar != nil {
		tmp := &Lhs{Var: st.LhsVar, Type: st.LhsVar.Type}
		if !lhsCG.VisitFactsLhs(tmp, opts) {
			return false
		}
	}
	cg.MergeParamContext(lhsCG, true)

	// FactMgr::update_fact_for_assign
	if cg.FM != nil && st.LhsVar != nil {
		indir := 0
		if st.Lhs != nil {
			indir = st.Lhs.IndirectLevel()
		}
		if indir == 0 {
			cg.FM.UpdateFactForAssign(st.LhsVar, 0, st.Expr)
		}
		// FactMgr.cpp: map_stm_effect[this] = effect_stm
		if st.StmID > 0 {
			cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
			cg.FM.SetMapFactsOut(st.StmID, cg.FM.GlobalFacts)
		}
	}
	return true
}
