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

// MakeRandomAssign mirrors StatementAssign::make_random (simplified Lhs = SelectGlobal/Local WRITE).
// StatementAssign.cpp:111+.
func MakeRandomAssign(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	typ *Type,
) Stmt {
	assignTab := NewAssignOpsTable(opts)
	// When type is nil, SelectLType after op — but op needs type for filters.
	// Upstream: op = AssignOpsProbability(type) then if type nil SelectLType(..., op).
	// First call with nil type uses simple/int path in AssignOpsProbability.
	op := AssignOpsProbability(r, opts, assignTab, typ)
	if typ == nil {
		// Type::SelectLType(!SE-free, op)
		noVol := !cg.EffectContext().IsSideEffectFree()
		typ = SelectLType(r, opts, probs, cg.Types, noVol, op)
		// Re-roll op for actual type (signed filter, float)
		op = AssignOpsProbability(r, opts, assignTab, typ)
	}
	// StatementAssign.cpp:211–216 — float LHS forces simple if op doesn't work
	if typ != nil && typ.IsFloat() && !AssignOpWorksForFloat(op) {
		op = AssignSimple
	}

	var rhs *Expression
	if op.NeedNoRHS() {
		rhs = &Expression{Term: TermConstant, Con: MakeInt(1)}
	} else {
		rhs = MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, false, false, MaxTermTypes, cg.ExprDepth)
		if rhs == nil {
			rhs = MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, TermConstant, cg.ExprDepth)
		}
	}

	// Lhs::make_random — SelectDerefPointerProb / local+global WRITE
	compound := op != AssignSimple
	lhs := MakeRandomLhs(r, opts, probs, vs, cg, typ, compound)
	var lhsVar *Variable
	if lhs != nil {
		lhsVar = lhs.Var
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
	// StatementAssign.cpp:218–223 — CompatibleChecker rejects self-compatible assign
	if CompatibleCheckExprVar(opts, lhsVar, rhs) {
		// regenerate RHS once as constant to avoid COMPATIBLE_CHECK_ERROR path
		rhs = &Expression{Term: TermConstant, Con: MakeRandom(typ, opts, r)}
		if rhs.Con != nil {
			rhs.CheckAndSetCast(typ)
		}
	}
	st := Stmt{Kind: StmtAssign, LhsVar: lhsVar, Expr: rhs, AssignOp: op, Lhs: lhs}
	// if LHS is a pointer to be dereferenced, emit via Lhs.Output shape
	lhsIndir := 0
	if lhs != nil {
		lhsIndir = lhs.IndirectLevel()
		if lhsIndir > 0 || (opts.WrapVolatiles && lhs.IsVolatile()) {
			st.ArrayAccess = lhs.Output(opts.WrapVolatiles)
		}
	}
	// FactMgr::update_fact_for_assign when points-to env present
	if cg.FM != nil && lhsVar != nil {
		// store into pointer var itself when exprTy is the pointer type (indir 0)
		if lhsIndir == 0 {
			cg.FM.UpdateFactForAssign(lhsVar, 0, rhs)
		}
		cg.NoteWrite(lhsVar)
	} else if lhsVar != nil {
		cg.NoteWrite(lhsVar)
	}
	return st
}
