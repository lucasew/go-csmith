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
	// StatementAssign.cpp:92–95 — non-simple or base float → simple assign
	if typ != nil && (!typ.IsSimple() || typ.IsFloat()) {
		return AssignSimple
	}
	// C++ always has RNG + assignOpsTable_; no soft invent table or simple without pick
	if r == nil {
		return AssignOp(-1)
	}
	if table == nil {
		// StatementAssign::InitProbabilityTable always live; fail closed invalid op
		return AssignOp(-1)
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
// MakeRandomAssign mirrors StatementAssign::make_random(cg, type, qf=nullptr).
// cg is *CGContext (C++ CGContext&) so merge_param_context RHS/LHS stick.
func MakeRandomAssign(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	typ *Type,
) Stmt {
	return MakeRandomAssignQfer(r, opts, probs, vs, tables, cg, typ, nil)
}

// MakeRandomAssignQfer mirrors StatementAssign::make_random with optional qf.
// StatementAssign.cpp:111–231 — when qf non-nil, force match_exact_qualifiers for Lhs.
// cg is *CGContext (C++ CGContext&).
func MakeRandomAssignQfer(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	typ *Type,
	qf *CVQualifiers,
) Stmt {
	// StatementAssign.cpp always has RNG + CGContext; no invent assign shell without them
	if r == nil || cg == nil {
		return Stmt{}
	}
	// StatementAssign.cpp:127 — assert(fm); nullptr empty Stmt (no Kind shell)
	if cg.FM == nil {
		return Stmt{}
	}
	// do not ClearError here — sticky Error::r_error_ is checked by ERROR_GUARD
	// after Statement::make_random (Statement.cpp:309)
	// StatementAssign::assignOpsTable_ from InitProbabilityTable (no invent per assign)
	assignTab := ProcessAssignOpsTable()
	if assignTab == nil {
		return Stmt{}
	}
	op := AssignOpsProbability(r, opts, assignTab, typ)
	if op < 0 {
		return Stmt{}
	}
	if typ == nil {
		// Type::SelectLType(!SE-free, op)
		noVol := !cg.EffectContext().IsSideEffectFree()
		typ = SelectLType(r, opts, probs, cg.Types, noVol, op)
		// ERROR_GUARD after SelectLType RNG paths
		if HasError() || typ == nil {
			return Stmt{}
		}
		op = AssignOpsProbability(r, opts, assignTab, typ)
	}
	// StatementAssign.cpp:124 — assert(!type->is_const_struct_union()); fail closed
	if typ != nil && typ.IsConstStructUnion() {
		return Stmt{}
	}
	// StatementAssign.cpp:211–216 — float LHS forces simple if op doesn't work
	if typ != nil && typ.IsFloat() && !AssignOpWorksForFloat(op) {
		op = AssignSimple
	}

	// StatementAssign.cpp:131–140 — running effect + separate RHS/LHS accum
	runningEff := cg.EffectContext()
	rhsAccum := EmptyEffect()
	rhsCG := *cg
	rhsCG.effectContext = runningEff
	rhsCG.EffectAccum = &rhsAccum
	rhsCG.EffectStm = EmptyEffect()

	var rhs *Expression
	// StatementAssign.cpp:147–148 — qfer from caller or derived from RHS
	qfer := NewCVQualifiers([]bool{false}, []bool{false})
	qfer.Wildcard = true
	callerQf := qf != nil
	if callerQf {
		qfer = *qf
	}

	// StatementAssign.cpp:145/168 — Expression::make_random(..., type, qf)
	// pass caller's qf into RHS when set (ExpressionAssign path); else nullptr.
	var rhsQf *CVQualifiers
	if callerQf {
		rhsQf = qf
	}
	if op.NeedNoRHS() {
		// StatementAssign.cpp:138–144 — Constant::make_int(1); wildcard when no qf
		rhs = &Expression{Term: TermConstant, Con: MakeInt(1)}
		if !callerQf {
			qfer.Wildcard = true
		}
	} else if opts.StrictVolatileRule {
		// StatementAssign.cpp:145–167
		if typ != nil && typ.IsVolatileStructUnion() {
			// StatementAssign.cpp:145–146 — return nullptr (no set_error)
			return Stmt{}
		}
		// StatementAssign.cpp:148 — Expression::make_random; ERROR_GUARD (no const soft-fallback)
		rhs = MakeRandomExpression(r, opts, tables, vs, &rhsCG, typ, rhsQf, false, false, MaxTermTypes, rhsCG.ExprDepth)
		if rhs == nil || HasError() {
			return Stmt{}
		}
		if !callerQf {
			if q := expressionQualifiers(rhs); q != nil {
				qfer = *q
				// StatementAssign.cpp:154–155 — accept_stricter; LHS not const
				qfer.AcceptStricter = true
				qfer.SetConst(false, 0)
			}
		}
		if op != AssignSimple {
			runningEff = runningEff.AddEffect(rhsAccum)
			if !callerQf {
				qfer.SetVolatile(false, 0)
			}
		}
		// StatementAssign.cpp:163 — always fold RHS into running under strict_volatile
		runningEff = runningEff.AddEffect(rhsAccum)
		if !callerQf && qfer.IsVolatile() {
			qfer.SetVolatile(false, 0)
		}
	} else {
		// StatementAssign.cpp:168–181
		rhs = MakeRandomExpression(r, opts, tables, vs, &rhsCG, typ, rhsQf, false, false, MaxTermTypes, rhsCG.ExprDepth)
		if rhs == nil || HasError() {
			return Stmt{}
		}
		if !callerQf {
			if q := expressionQualifiers(rhs); q != nil {
				qfer = *q
				// StatementAssign.cpp:172–174
				qfer.AcceptStricter = true
				qfer.SetConst(false, 0)
			}
		}
		if op != AssignSimple {
			runningEff = runningEff.AddEffect(rhsAccum)
			if !callerQf {
				qfer.SetVolatile(false, 0)
			}
		}
	}
	// StatementAssign.cpp:181 — merge_param_context(rhs_cg_context, true)
	cg.MergeParamContext(rhsCG, true)

	// StatementAssign.cpp:183 — write_var_set(rhs_accum.get_lhs_write_vars())
	if lw := rhsAccum.LhsWriteVars(); len(lw) > 0 {
		runningEff = runningEff.WriteVarSet(lw)
	}

	// LHS context after RHS (StatementAssign.cpp:185–199)
	lhsAccum := EmptyEffect()
	lhsCG := *cg
	lhsCG.effectContext = runningEff
	lhsCG.EffectAccum = &lhsAccum
	lhsCG.EffectStm = rhsCG.EffectStm
	lhsCG.CurrRHS = rhs

	// StatementAssign.cpp:190–203 — CGOptions::match_exact_qualifiers(true) when qf
	// process-wide for CVQualifiers::match / choose_var; restore after Lhs
	prevExact := opts.MatchExactQualifiers
	prevProc := ProcessOptions()
	if callerQf {
		opts.MatchExactQualifiers = true
		po := prevProc
		po.MatchExactQualifiers = true
		SetProcessOptions(po)
	}
	// StatementAssign.cpp:195–200 — strict_float uses RHS type for Lhs
	lhsType := typ
	if opts.StrictFloat && rhs != nil {
		if rt := rhs.GetType(); rt != nil {
			lhsType = rt
		}
	}
	compound := op != AssignSimple
	lhs := MakeRandomLhs(r, opts, probs, vs, &lhsCG, lhsType, compound, op.NeedNoRHS(), &qfer)
	if callerQf {
		opts.MatchExactQualifiers = prevExact
		SetProcessOptions(prevProc)
	}
	var lhsVar *Variable
	if lhs != nil {
		lhsVar = lhs.Var
	}
	if lhs == nil {
		// Lhs::make_random null — re-pick unless sticky error already set
		return Stmt{}
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
	// StatementAssign.cpp:211–216 — float base forces simple op
	if lhsVar != nil && lhsVar.Type != nil {
		if bt := lhsVar.Type.BaseType(); bt != nil && bt.IsFloat() && !AssignOpWorksForFloat(op) {
			op = AssignSimple
		}
	}
	if rhs != nil {
		if rt := rhs.GetType(); rt != nil {
			if bt := rt.BaseType(); bt != nil && bt.IsFloat() && !AssignOpWorksForFloat(op) {
				op = AssignSimple
			}
		}
	}

	// StatementAssign.cpp:218–223 — CompatibleChecker → nullptr
	if CompatibleCheckExprs(opts, rhs, LhsAsExpression(lhs)) {
		SetError(ErrCompatibleCheck)
		return Stmt{}
	}

	// StatementAssign.cpp:225 — merge_param_context(lhs_cg_context, true)
	cg.MergeParamContext(lhsCG, true)

	// StatementAssign.cpp:228 — make_possible_compound_assign (safe math flags/tmps)
	st := makePossibleCompoundAssign(*cg, opts, probs, r, typ, lhs, op, rhs, gensymFromVS(vs))
	lhsIndir := 0
	if st.Lhs != nil {
		lhsIndir = st.Lhs.IndirectLevel()
		if lhsIndir > 0 || (opts.WrapVolatiles && st.Lhs.IsVolatile()) {
			st.ArrayAccess = st.Lhs.Output(opts.WrapVolatiles)
		}
	}
	// FactMgr::update_fact_for_assign(sa) — get_rhs() (canonized ExpressionFuncall)
	if cg.FM != nil && st.LhsVar != nil {
		_ = cg.FM.UpdateFactForAssign(st.LhsVar, lhsIndir, st.GetAssignRhs())
		// incomplete assign must not invent assign stmt with wiped GlobalFacts
		if !FactsComplete(cg.FM.GlobalFacts) {
			return Stmt{}
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
// StatementAssign.cpp:244–301 — for compound ops: SafeOpFlags + FunctionInvocationBinary
// operands (ExpressionVariable(lhs), e.clone()) wrapped as ExpressionFuncall rhs;
// CreateFunctionInvocationBinary always allocates t_ temps for safe_ops.
// Not gated on avoid_signed_overflow (SafeMath only affects OutputAsExpr).
// sym is the session GenSym (VS.Sym); nil uses package gensym like util.cpp.
func makePossibleCompoundAssign(
	cg CGContext,
	opts Options,
	probs *Probabilities,
	r *Rng,
	typ *Type,
	lhs *Lhs,
	op AssignOp,
	rhs *Expression,
	sym *GenSym,
) Stmt {
	// Statement base ctor always assigns stm_id (Statement.cpp:364–367)
	st := Stmt{Kind: StmtAssign, AssignOp: op, Expr: rhs, Lhs: lhs, Rhs: rhs, StmID: AllocStmID()}
	if lhs != nil {
		st.LhsVar = lhs.Var
	}
	bop, ok := op.CompoundToBinaryOps()
	if !ok {
		// simple assign — StatementAssign ctor rhs(&expr)
		return st
	}
	// compound always maps to a live binary token; no invent empty Binary shell
	opStr := bop.BinaryOpC()
	if int(bop) < 0 || int(bop) >= MaxBinaryOp || opStr == "" {
		return Stmt{}
	}
	lt := typ
	if lhs != nil {
		if t := lhs.GetType(); t != nil {
			lt = t
		}
	}
	var flags *SafeOpFlags
	var inv *Invocation
	if SafeAssign(op) {
		// StatementAssign.cpp:256–259 — dummy flags + FunctionInvocationBinary(bop, local_fs)
		flags = MakeDummyFlags()
		inv = &Invocation{IsStd: true, Binary: opStr, Safe: flags}
		inv.setOutOpts(opts)
	} else {
		// StatementAssign.cpp:260–266 — make_random_binary + CreateFunctionInvocationBinary
		// SafeOpFlags.cpp:169–215 via make_random_binary(..., sOpAssign, bop)
		// always has RNG for non-safe compounds; no invent nil-flags shell
		if r == nil {
			return Stmt{}
		}
		flags = MakeRandomBinaryKind(r, opts, probs, lt, lt, lt, SafeOpAssign, bop)
		// StatementAssign.cpp:260–262 — ERROR_GUARD(nullptr); no soft invent nil-flags compound
		if flags == nil || HasError() {
			return Stmt{}
		}
		inv = &Invocation{IsStd: true, Binary: opStr, Safe: flags}
		inv.setOutOpts(opts)
		// FunctionInvocationBinary.cpp:59–75 — always create tmps for safe_ops
		// assert(blk) when safe_ops — no soft invent compound without temps
		if SafeOpsBinary(opStr) {
			blk := cg.CurrentBlock()
			if blk == nil {
				// FunctionInvocationBinary.cpp:68 assert(blk)
				return Stmt{}
			}
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
			st.Tmp1 = blk.CreateNewTmpVar(sym, st1)
			st.Tmp2 = blk.CreateNewTmpVar(sym, st2)
			inv.Tmp1, inv.Tmp2 = st.Tmp1, st.Tmp2
		}
	}
	st.SafeFlags = flags
	// StatementAssign.cpp:269–271 — add_operand ExpressionVariable(lhs); e.clone(); ExpressionFuncall
	lhsExpr := LhsAsExpression(lhs)
	if lhsExpr == nil {
		// C++ always has live Lhs; incomplete IR → empty assign (ERROR path)
		return Stmt{}
	}
	// e.clone() — Expression is value-like; shallow copy of the root is enough
	// (operands of the original expr are shared by pointer, as clone shares subtrees).
	var rhsClone *Expression
	if rhs != nil {
		cp := *rhs
		rhsClone = &cp
	}
	inv.Args = []*Expression{lhsExpr, rhsClone}
	st.Rhs = &Expression{Term: TermFunction, Invoke: inv, ExprType: lt}
	return st
}

// GetAssignRhs mirrors StatementAssign::get_rhs — canonized compound form when set.
// StatementAssign.h:109; FactMgr::update_fact_for_assign(sa) uses get_rhs().
func (st *Stmt) GetAssignRhs() *Expression {
	if st == nil {
		return nil
	}
	if st.Rhs != nil {
		return st.Rhs
	}
	return st.Expr
}

// gensymFromVS returns &vs.Sym for create_new_tmp_var / gensym share with g_/l_.
func gensymFromVS(vs *VariableSelector) *GenSym {
	if vs == nil {
		return nil
	}
	return &vs.Sym
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
	// StatementAssign.cpp:515–537 — expr.Output always for ops that need RHS
	// no soft invent "0" or "lhs = " empty RHS for incomplete IR
	if st.AssignOp.NeedNoRHS() {
		return st.AssignOp.AssignOpC(lhs, "")
	}
	if st.Expr == nil {
		return ""
	}
	rhs := st.Expr.Output()
	if rhs == "" {
		return ""
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
// Uses process CGOptions (identify_wrappers); no soft invent Defaults().
func OutputAssignAsExpr(st *Stmt, wrapVol bool) string {
	return OutputAssignAsExprOpts(st, wrapVol, ProcessOptions())
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
	// StatementAssign.cpp: expr.Output always for ops that need RHS
	// no soft invent "0" / bare lhs when Expr missing or Output empty
	var rhs string
	if st.Expr != nil {
		rhs = st.Expr.Output()
	}
	if !st.AssignOp.NeedNoRHS() && (st.Expr == nil || rhs == "") {
		return ""
	}
	// StatementAssign.cpp:543 — if (avoid_signed_overflow() && op_flags)
	// no soft invent safe rewrite when SafeMath off or flags missing
	if opts.SafeMath && st.SafeFlags != nil {
		switch st.AssignOp {
		case AssignSimple, AssignBitAnd, AssignBitXor, AssignBitOr:
			// StatementAssign.cpp:546–565 — simple/bit compounds
			// ccomp + volatile + real compound → "lhs = lhs binop rhs"
			// no invent "lhs = lhs + " with empty rhs Output
			if bop, ok := st.AssignOp.CompoundToBinaryOps(); ok && opts.CComp {
				if assignLhsIsVolatile(st) {
					return lhs + " = " + lhs + " " + bop.BinaryOpC() + " " + rhs
				}
			}
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
				// incomplete IR — no invent OutputSimple for broken compound map
				return ""
			}
			fname := st.SafeFlags.BinaryFuncName(bop.BinaryOpC())
			if fname == "" {
				// SafeOpFlags.cpp assert empty name — fail closed no invent bare +=
				return ""
			}
			id := SafeOpFlagsToID(fname)
			// don't use wrapper if filtered out by --safe-math-wrapper
			if !SafeMathWrapperAllowed(opts, id) {
				return OutputAssignSimple(st, wrapVol)
			}
			// StatementAssign.cpp:595–598 — expr.Output always (live Expression*)
			if rhs == "" && !st.AssignOp.NeedNoRHS() {
				return ""
			}
			var b strings.Builder
			b.WriteString(lhs)
			b.WriteString(" = ")
			b.WriteString(fname)
			b.WriteString("(")
			// StatementAssign.cpp:584–591 — math_notmp gates emit, not tmp existence
			if opts.MathNoTmp && st.Tmp1 != "" {
				b.WriteString(st.Tmp1)
				b.WriteString(", ")
			}
			b.WriteString(lhs)
			b.WriteString(", ")
			if opts.MathNoTmp && st.Tmp2 != "" {
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
			// StatementAssign.cpp:618–619 — assert(false); no soft invent OutputSimple
			return ""
		}
	}
	return OutputAssignSimple(st, wrapVol)
}

// assignLhsIsVolatile reports LHS volatile for OutputAsExpr ccomp rewrite.
// StatementAssign.cpp:552 — lhs.is_volatile().
func assignLhsIsVolatile(st *Stmt) bool {
	if st == nil {
		return false
	}
	if st.Lhs != nil {
		return st.Lhs.IsVolatile()
	}
	return st.LhsVar != nil && st.LhsVar.IsVolatile()
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
// Incomplete IR (nil/missing sides) fails — no soft invent true (C++ always has live Expression*).
func VisitFactsExpression(e *Expression, cg *CGContext, opts Options) bool {
	if e == nil || cg == nil {
		return false
	}
	switch e.Term {
	case TermConstant:
		// Constant.cpp always has live value string; incomplete Con fails visit
		// no soft invent visit success for TermConstant shell without Value
		if e.Con == nil || e.Con.Value == "" {
			return false
		}
		return true
	case TermVariable:
		return cg.VisitFactsExpressionVariable(e, opts)
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			return false
		}
		if !VisitFactsExpression(e.CommaLHS, cg, opts) {
			return false
		}
		return VisitFactsExpression(e.CommaRHS, cg, opts)
	case TermAssignment:
		if e.Assign == nil {
			return false
		}
		return VisitFactsStatementAssign(e.Assign, cg, opts)
	case TermFunction:
		if e.Invoke == nil {
			return false
		}
		return VisitFactsInvocation(e.Invoke, cg, opts)
	default:
		// unknown term; no soft invent success
		return false
	}
}

// VisitFactsInvocation mirrors FunctionInvocation::visit_facts.
// FunctionInvocation.cpp:502–555 — ordered params (unordered path available but
// upstream sets unordered=false); then user revisit when NeedsRevisit.
// Binary &&/|| use FunctionInvocationBinary::visit_facts short-circuit merge.
func VisitFactsInvocation(fi *Invocation, cg *CGContext, opts Options) bool {
	// C++ always has live FunctionInvocation*; nil / failed → visit fail (no soft invent true)
	if fi == nil || cg == nil {
		return false
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
			// incomplete GlobalFacts fail closed (no invent cleaned visit)
			if !FactsComplete(cg.FM.GlobalFacts) {
				return false
			}
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
			// FunctionInvocation.cpp: param_value[i] always non-null after ERROR_GUARD
			if arg == nil {
				_ = i
				return false
			}
			paramAccum := EmptyEffect()
			paramCG := *cg
			paramCG.effectContext = running
			paramCG.EffectAccum = &paramAccum
			if !VisitFactsExpression(arg, &paramCG, opts) {
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
			// incomplete GlobalFacts fail closed (no invent cleaned revisit)
			if !FactsComplete(cg.FM.GlobalFacts) {
				return false
			}
			facts := CloneFactSlice(cg.FM.GlobalFacts)
			if !RevisitUserInvocation(fi, &facts, cg, opts) {
				return false
			}
			// incomplete post-revisit must not invent GlobalFacts assignment
			if !FactsComplete(facts) {
				return false
			}
			cg.FM.GlobalFacts = facts
			// fold feffect from accum during revisit
			if cg.InConflict(fi.User.FEffect) {
				return false
			}
			// FunctionInvocation.cpp:543 — assert(cg_context.curr_blk)
			blk := cg.CurrentBlock()
			if blk == nil {
				return false
			}
			cg.AddVisibleEffectAt(fi.User.FEffect, blk)
		} else if fi.User.IsEffectKnown() {
			// static effect path (no fact/pointer change)
			if cg.InConflict(fi.User.FEffect) {
				return false
			}
			// same curr_blk for visible effect (visit_facts path uses curr_blk)
			blk := cg.CurrentBlock()
			if blk == nil {
				return false
			}
			cg.AddVisibleEffectAt(fi.User.FEffect, blk)
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
// StatementAssign.cpp:358–390 — RHS first; compound folds RHS effect into LHS
// context; write_var_set of RHS lhs_write_vars; update_fact_for_assign; map_stm_effect.
func VisitFactsStatementAssign(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil || st.Kind != StmtAssign {
		return false
	}
	// StatementAssign.cpp always has live Lhs and Expression* (make_int(1) for ++/--)
	if st.Expr == nil {
		return false
	}
	// StatementAssign.cpp:362–367 — RHS in its own accum context
	runningEff := cg.EffectContext()
	rhsAccum := EmptyEffect()
	rhsCG := *cg
	rhsCG.effectContext = runningEff
	rhsCG.EffectAccum = &rhsAccum
	rhsCG.EffectStm = cg.EffectStm

	if !VisitFactsExpression(st.Expr, &rhsCG, opts) {
		return false
	}
	// StatementAssign.cpp:372–375 — compound: LHS sees RHS effect
	if st.AssignOp != AssignSimple {
		runningEff = runningEff.AddEffect(rhsAccum)
	}
	cg.MergeParamContext(rhsCG, true)
	// StatementAssign.cpp:377 — write_var_set(rhs_accum.get_lhs_write_vars())
	if lw := rhsAccum.LhsWriteVars(); len(lw) > 0 {
		runningEff = runningEff.WriteVarSet(lw)
	}

	// StatementAssign.cpp:379–384 — LHS context
	lhsAccum := EmptyEffect()
	lhsCG := *cg
	lhsCG.effectContext = runningEff
	lhsCG.EffectAccum = &lhsAccum
	lhsCG.EffectStm = rhsCG.EffectStm
	lhsCG.CurrRHS = st.Expr

	var lhsVar *Variable
	indir := 0
	if st.Lhs != nil {
		if !lhsCG.VisitFactsLhs(st.Lhs, opts) {
			return false
		}
		lhsVar = st.Lhs.Var
		indir = st.Lhs.IndirectLevel()
	} else if st.LhsVar != nil {
		tmp := &Lhs{Var: st.LhsVar, Type: st.LhsVar.Type}
		if !lhsCG.VisitFactsLhs(tmp, opts) {
			return false
		}
		lhsVar = st.LhsVar
	} else {
		// incomplete assign IR — no soft invent visit success without LHS
		return false
	}
	cg.MergeParamContext(lhsCG, true)

	// StatementAssign.cpp:386 — FactMgr::update_fact_for_assign(this, inputs)
	// uses get_rhs() (canonized ExpressionFuncall for compounds)
	if cg.FM != nil && lhsVar != nil {
		_ = cg.FM.UpdateFactForAssign(lhsVar, indir, st.GetAssignRhs())
		// incomplete assign must not invent visit success / SetMapFactsOut
		if !FactsComplete(cg.FM.GlobalFacts) {
			return false
		}
		// StatementAssign.cpp:388–389 — map_stm_effect[this] = effect_stm
		if st.StmID > 0 {
			cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
			cg.FM.SetMapFactsOut(st.StmID, cg.FM.GlobalFacts)
		}
	}
	return true
}
