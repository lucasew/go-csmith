// Upstream: StatementAssign.cpp (InitProbabilityTable, AssignOpsProbability, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

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

	// LHS context after RHS (StatementAssign.cpp:185–199)
	lhsAccum := EmptyEffect()
	lhsCG := cg
	lhsCG.effectContext = runningEff
	lhsCG.EffectAccum = &lhsAccum
	lhsCG.EffectStm = rhsCG.EffectStm
	lhsCG.CurrRHS = rhs

	compound := op != AssignSimple
	// need_no_rhs path uses compound-like for Lhs? upstream passes need_no_rhs as no_signed_overflow flag
	lhs := MakeRandomLhs(r, opts, probs, vs, lhsCG, typ, compound || op.NeedNoRHS())
	var lhsVar *Variable
	if lhs != nil {
		lhsVar = lhs.Var
	}
	if lhs == nil {
		SetError(ErrGeneric)
		return Stmt{Kind: StmtAssign}
	}

	// RHS cast to L type when needed (StatementAssign.cpp:207–208)
	if rhs != nil && typ != nil {
		rhs.CheckAndSetCast(typ)
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
			rhs.CheckAndSetCast(typ)
		}
		ClearError()
	}

	cg.MergeParamContext(lhsCG, true)

	st := Stmt{Kind: StmtAssign, LhsVar: lhsVar, Expr: rhs, AssignOp: op, Lhs: lhs}
	lhsIndir := 0
	if lhs != nil {
		lhsIndir = lhs.IndirectLevel()
		if lhsIndir > 0 || (opts.WrapVolatiles && lhs.IsVolatile()) {
			st.ArrayAccess = lhs.Output(opts.WrapVolatiles)
		}
	}
	// FactMgr::update_fact_for_assign
	if cg.FM != nil && lhsVar != nil {
		if lhsIndir == 0 {
			cg.FM.UpdateFactForAssign(lhsVar, 0, rhs)
		}
		cg.NoteWrite(lhsVar)
	} else if lhsVar != nil {
		cg.NoteWrite(lhsVar)
	}
	return st
}

// expressionQualifiers approximates Expression::get_qualifiers for qfer seed.
func expressionQualifiers(e *Expression) *CVQualifiers {
	if e == nil {
		return nil
	}
	switch e.Term {
	case TermVariable:
		if e.Var != nil {
			q := e.Var.Qfer
			return &q
		}
	case TermAssignment:
		if e.Assign != nil && e.Assign.LhsVar != nil {
			q := e.Assign.LhsVar.Qfer
			return &q
		}
	}
	return nil
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
		// invocation facts deferred — accept for now
		return true
	default:
		return true
	}
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
	}
	return true
}
