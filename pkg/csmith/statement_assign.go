// Upstream: StatementAssign.cpp (InitProbabilityTable, AssignOpsProbability, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// NewAssignOpsTable mirrors StatementAssign::InitProbabilityTable.
// StatementAssign.cpp:68–81.
func NewAssignOpsTable(opts Options) *DistributionTable {
	// Construction path: residual bag unused (table always live).
	return NewAssignOpsTableSess(testAmbientSession, opts)
}

// NewAssignOpsTableSess builds assign-op distribution with explicit residual bag.
func NewAssignOpsTableSess(s *Session, opts Options) *DistributionTable {
	var t DistributionTable
	t.AddEntrySess(s, int(AssignSimple), 70)
	t.AddEntrySess(s, int(AssignBitAnd), 10)
	t.AddEntrySess(s, int(AssignBitXor), 10)
	t.AddEntrySess(s, int(AssignBitOr), 10)
	if opts.PreIncrOperator {
		t.AddEntrySess(s, int(AssignPreIncr), 5)
	}
	if opts.PreDecrOperator {
		t.AddEntrySess(s, int(AssignPreDecr), 5)
	}
	if opts.PostIncrOperator {
		t.AddEntrySess(s, int(AssignPostIncr), 5)
	}
	if opts.PostDecrOperator {
		t.AddEntrySess(s, int(AssignPostDecr), 5)
	}
	return &t
}

// AssignOpsProbability mirrors StatementAssign::AssignOpsProbability.
// StatementAssign.cpp:84–106.
func AssignOpsProbability(r *Rng, opts Options, table *DistributionTable, typ *Type) AssignOp {
	return AssignOpsProbabilitySess(testAmbientSession, r, opts, table, typ)
}

func AssignOpsProbabilitySess(s *Session, r *Rng, opts Options, table *DistributionTable, typ *Type) AssignOp {
	if !opts.CompoundAssignment {
		return AssignSimple
	}
	// StatementAssign.cpp:92–95 — non-simple or base float → simple assign
	if typ != nil {
		simple := typ.IsSimpleSess(s)
		// residual ERROR sticky — no invent soft-simple past IsSimple residual
		if sessHasError(s) {
			return AssignOp(-1)
		}
		if !simple {
			return AssignSimple
		}
		isF := typ.IsFloatSess(s)
		// residual ERROR sticky — no invent soft-simple past IsFloat residual
		if sessHasError(s) {
			return AssignOp(-1)
		}
		if isF {
			return AssignSimple
		}
	}
	// C++ always has RNG + assignOpsTable_ sticky; no invent table or simple without pick
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return AssignOp(-1)
	}
	if table == nil {
		// StatementAssign::InitProbabilityTable always live; sticky fail closed invalid op
		sessNoteError(s, ErrGeneric)
		return AssignOp(-1)
	}
	f := NewVectorFilterSess(s, table)
	// signed ints: filter out ++/-- (upstream avoids for signed)
	if typ != nil {
		signed := typ.IsSignedSess(s)
		// residual ERROR sticky — no invent soft-filter past IsSigned residual
		if sessHasError(s) {
			return AssignOp(-1)
		}
		if signed {
			f.AddSess(s, int(AssignPreIncr)).AddSess(s, int(AssignPreDecr)).AddSess(s, int(AssignPostIncr)).AddSess(s, int(AssignPostDecr))
		}
	}
	v := r.RndUptoFilterSess(s, uint32(f.MaxProb()), f)
	return AssignOp(f.Lookup(int(v)))
}

// SafeAssign mirrors StatementAssign::safe_assign — bit ops need no overflow wrap.
// StatementAssign.cpp:233–242.}

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
	// StatementAssign.cpp always has RNG + CGContext; sticky no invent assign shell without them
	if r == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	// StatementAssign.cpp:127 — assert(fm); nullptr empty Stmt (no Kind shell)
	if cg.FM == nil {
		return Stmt{}
	}
	// Incomplete ambient/facts fail closed sticky (no invent assign under hole shells)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	if !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	// do not ClearError here — sticky Error::r_error_ is checked by ERROR_GUARD
	// after Statement::make_random (Statement.cpp:309)
	// StatementAssign::assignOpsTable_ from InitProbabilityTable sticky (no invent per assign)
	assignTab := sessAssignOpsTab(sessFromCG(cg))
	if assignTab == nil {
		noteErrCG(cg, ErrGeneric)
		return Stmt{}
	}
	// StatementAssign.cpp:115–123 — AssignOpsProbabilitySess(sessFromCG(cg), type) once; if type null
	// SelectLType(no_vol, op) using that op. Do NOT re-pick op after SelectLType
	// (seed-2 event 45 was an invented second AssignOpsProbability draw).
	op := AssignOpsProbabilitySess(sessFromCG(cg), r, opts, assignTab, typ)
	if op < 0 {
		// AssignOpsProbability already stickies on nil r/table; other invalid op soft
		return Stmt{}
	}
	if typ == nil {
		// Type::SelectLType(!SE-free, op)
		seFree := cg.EffectContext().IsSideEffectFreeSess(sessFromCG(cg))
		// residual ERROR sticky — no invent soft-no-vol SelectLType past IsSideEffectFree residual
		if hasErrCG(cg) {
			return Stmt{}
		}
		typ = SelectLType(r, opts, probs, cg.Types, !seFree, op)
		// ERROR_GUARD after SelectLType RNG paths
		if hasErrCG(cg) || typ == nil {
			return Stmt{}
		}
	}
	// StatementAssign.cpp:124 — assert(!type->is_const_struct_union()) sticky
	if typ != nil {
		isCSU := typ.IsConstStructUnionSess(sessFromCG(cg))
		// residual ERROR sticky — no invent soft-continue assign past IsConstStructUnion residual
		if hasErrCG(cg) {
			return Stmt{}
		}
		if isCSU {
			noteErrCG(cg, ErrGeneric)
			return Stmt{}
		}
	}
	// StatementAssign.cpp:211–216 — float LHS forces simple if op doesn't work
	if typ != nil {
		isF := typ.IsFloatSess(sessFromCG(cg))
		// residual ERROR sticky — no invent soft-continue float op past IsFloat residual
		if hasErrCG(cg) {
			return Stmt{}
		}
		if isF && !AssignOpWorksForFloat(op) {
			op = AssignSimple
		}
	}

	// StatementAssign.cpp:131–140 — running effect + separate RHS/LHS accum
	// CGContext.cpp:74–82 — (cgc, running_eff, &rhs_accum): curr_rhs(nullptr)
	runningEff := cg.EffectContext().detachMaps()
	rhsAccum := EmptyEffect()
	rhsCG := cg.CloneSubcontext()
	rhsCG.effectContext = runningEff
	rhsCG.EffectAccum = &rhsAccum
	rhsCG.EffectStm = EmptyEffect()
	rhsCG.CurrRHS = nil

	var rhs *Expression
	// StatementAssign.cpp:147–148 — qfer from caller or derived from RHS
	qfer := NewCVQualifiers([]bool{false}, []bool{false})
	qfer.Wildcard = true
	callerQf := qf != nil
	if callerQf {
		// C++ *qf value-copy; Clone so later SetVolatile cannot mutate caller qfer
		qfer = qf.Clone()
	}

	// StatementAssign.cpp:145/168 — Expression::make_random(..., type, qf)
	// pass caller's qf into RHS when set (ExpressionAssign path); else nullptr.
	var rhsQf *CVQualifiers
	if callerQf {
		rhsQf = qf
	}
	if op.NeedNoRHS() {
		// StatementAssign.cpp:138–144 — Constant::make_int(1); wildcard when no qf
		rhs = &Expression{Term: TermConstant, Con: MakeIntSess(sessFromCG(cg), 1)}
		if !callerQf {
			qfer.Wildcard = true
		}
	} else if opts.StrictVolatileRule {
		// StatementAssign.cpp:145–167
		if typ != nil {
			isVSU := typ.IsVolatileStructUnionSess(sessFromCG(cg))
			// residual ERROR sticky — no invent soft-continue RHS past IsVolatileStructUnion residual
			if hasErrCG(cg) {
				return Stmt{}
			}
			if isVSU {
				// StatementAssign.cpp:145–146 — return nullptr (no set_error)
				return Stmt{}
			}
		}
		// StatementAssign.cpp:148 — Expression::make_random; ERROR_GUARD (no const soft-fallback)
		rhs = MakeRandomExpression(r, opts, tables, vs, &rhsCG, typ, rhsQf, false, false, MaxTermTypes, rhsCG.ExprDepth)
		if rhs == nil || hasErrCG(cg) {
			return Stmt{}
		}
		if !callerQf {
			if q := expressionQualifiersSess(sessFromCG(cg), rhs); q != nil {
				// Clone: expressionQualifiers may alias Variable.qfer slice backing
				qfer = q.Clone()
				// StatementAssign.cpp:151–152 — accept_stricter only.
				// Do not SetConst(false) here: C++ leaves const bits from RHS quals;
				// Lhs::make_random Select path calls restrict(WRITE) → set_const(false).
				// Early SetConst desyncs AcceptStricter matching for pointer Lhs.
				qfer.AcceptStricter = true
			}
		}
		if op != AssignSimple {
			runningEff = runningEff.AddEffectSess(sessFromCG(cg), rhsAccum)
			// residual ERROR sticky — no invent soft-continue compound past AddEffect residual
			if hasErrCG(cg) {
				return Stmt{}
			}
			if !EffectComplete(runningEff) {
				noteErrCG(cg, ErrGeneric)
				return Stmt{}
			}
			// StatementAssign.cpp:156–159 — compound always set_volatile(false),
			// even when caller qf non-nil (ExpressionAssign / MatchExact path).
			qfer.SetVolatileSess(sessFromCG(cg), false, 0)
		}
		// StatementAssign.cpp:161 — always fold RHS into running under strict_volatile
		runningEff = runningEff.AddEffectSess(sessFromCG(cg), rhsAccum)
		// residual ERROR sticky — no invent soft-continue strict-vol past AddEffect residual
		if hasErrCG(cg) {
			return Stmt{}
		}
		if !EffectComplete(runningEff) {
			noteErrCG(cg, ErrGeneric)
			return Stmt{}
		}
		// StatementAssign.cpp:163–165 — not gated on qf:
		// if (qfer.get_volatiles().size() && qfer.is_volatile()) set_volatile(false)
		if qfer.IsVolatileSess(sessFromCG(cg)) {
			// residual ERROR sticky — no invent soft-clear vol past IsVolatile residual
			if hasErrCG(cg) {
				return Stmt{}
			}
			qfer.SetVolatileSess(sessFromCG(cg), false, 0)
		} else if hasErrCG(cg) {
			// residual ERROR sticky — no invent soft-continue past IsVolatile residual false
			return Stmt{}
		}
	} else {
		// StatementAssign.cpp:168–181
		rhs = MakeRandomExpression(r, opts, tables, vs, &rhsCG, typ, rhsQf, false, false, MaxTermTypes, rhsCG.ExprDepth)
		if rhs == nil || hasErrCG(cg) {
			return Stmt{}
		}
		if !callerQf {
			if q := expressionQualifiersSess(sessFromCG(cg), rhs); q != nil {
				// Clone: do not share Variable.qfer slices with later SetVolatile
				qfer = q.Clone()
				// StatementAssign.cpp:172–174 — accept_stricter only (no set_const).
				// Lhs Select restrict(WRITE) clears const; early SetConst unfairly
				// changes AcceptStricter matching for multi-level pointer Lhs.
				qfer.AcceptStricter = true
			}
		}
		if op != AssignSimple {
			runningEff = runningEff.AddEffectSess(sessFromCG(cg), rhsAccum)
			// residual ERROR sticky — no invent soft-continue compound past AddEffect residual
			if hasErrCG(cg) {
				return Stmt{}
			}
			if !EffectComplete(runningEff) {
				noteErrCG(cg, ErrGeneric)
				return Stmt{}
			}
			// StatementAssign.cpp:176–179 — compound always set_volatile(false)
			// regardless of caller qf (func-param ExpressionAssign MatchExact).
			qfer.SetVolatileSess(sessFromCG(cg), false, 0)
		}
	}
	// StatementAssign.cpp:181 — merge_param_context(rhs_cg_context, true)
	cg.MergeParamContext(rhsCG, true)
	// incomplete effect after RHS merge fails closed sticky (no invent LHS / soft re-pick)
	if hasErrCG(cg) || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		return Stmt{}
	}

	// StatementAssign.cpp:183 — write_var_set(rhs_accum.get_lhs_write_vars())
	// IncompleteVariables → WriteVarSet IncompleteEffect (no invent skip empty merge
	// when LhsWriteVars used bare nil on incomplete rhs_accum).
	if lw := rhsAccum.LhsWriteVarsSess(sessFromCG(cg)); !VariablesComplete(lw) || len(lw) > 0 {
		// residual ERROR sticky — no invent soft-skip WriteVarSet past LhsWriteVars residual
		if hasErrCG(cg) {
			return Stmt{}
		}
		runningEff = runningEff.WriteVarSetSess(sessFromCG(cg), lw)
		// residual ERROR sticky — no invent soft-continue LHS past WriteVarSet residual
		if hasErrCG(cg) || !EffectComplete(runningEff) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return Stmt{}
		}
	} else if hasErrCG(cg) {
		// residual ERROR sticky — no invent soft-empty LhsWriteVars past residual hole
		return Stmt{}
	}

	// LHS context after RHS (StatementAssign.cpp:185–199)
	lhsAccum := EmptyEffect()
	lhsCG := cg.CloneSubcontext()
	lhsCG.effectContext = runningEff
	lhsCG.EffectAccum = &lhsAccum
	// Effect.cpp:84–89 assignment deep-copies vectors; do not share EffectStm maps
	// with rhsCG (Lhs visit WriteVar COW would still leave dual refs on no-new-write paths).
	lhsCG.EffectStm = rhsCG.EffectStm.detachMaps()
	lhsCG.CurrRHS = rhs

	// StatementAssign.cpp:190–203 — CGOptions::match_exact_qualifiers(true) when qf
	// for CVQualifiers::match / choose_var; restore after Lhs.
	// Must restore on every early return (no invent sticky MatchExactQualifiers
	// that over-restricts later choose_var qfer match — seed-2 OK-list shrink).
	// Bag-local: opts + sessFromCG(cg).Opts (ChooseVarFull → sessOpts). No ProcessOptions dual-path.
	prevExact := opts.MatchExactQualifiers
	if callerQf {
		opts.MatchExactQualifiers = true
		bag := sessFromCG(cg)
		prevBagExact := bag.Opts.MatchExactQualifiers
		bag.Opts.MatchExactQualifiers = true
		defer func() {
			opts.MatchExactQualifiers = prevExact
			bag.Opts.MatchExactQualifiers = prevBagExact
		}()
	}
	// StatementAssign.cpp:195–200 — strict_float uses RHS type for Lhs
	lhsType := typ
	if opts.StrictFloat && rhs != nil {
		if rt := rhs.GetTypeSess(sessFromCG(cg)); rt != nil {
			// residual ERROR sticky — no invent Lhs type soft-fallback past GetType residual
			if hasErrCG(cg) {
				return Stmt{}
			}
			lhsType = rt
		} else if hasErrCG(cg) {
			// residual ERROR sticky — no invent Lhs past GetType residual nil
			return Stmt{}
		}
	}
	compound := op != AssignSimple
	lhs := MakeRandomLhs(r, opts, probs, vs, &lhsCG, lhsType, compound, op.NeedNoRHS(), &qfer)
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
		rhs.CheckAndSetCastOptsSess(sessFromCG(cg), typ, opts)
		// residual ERROR sticky — no invent Assign past CheckAndSetCast residual hole
		if hasErrCG(cg) {
			return Stmt{}
		}
	}
	if opts.CComp && lhsVar != nil && lhsVar.IsBitfield {
		if rhs != nil {
			rhs.CastType = typ
		}
	}
	// StatementAssign.cpp:211–216 — float base forces simple op
	if lhsVar != nil && lhsVar.Type != nil {
		if bt := lhsVar.Type.BaseTypeSess(sessFromCG(cg)); bt != nil && bt.IsFloatSess(sessFromCG(cg)) && !AssignOpWorksForFloat(op) {
			// residual ERROR sticky — no invent float-op soft-continue past BaseType residual
			if hasErrCG(cg) {
				return Stmt{}
			}
			op = AssignSimple
		} else if hasErrCG(cg) {
			// residual ERROR sticky — no invent soft-continue op past BaseType residual false
			return Stmt{}
		}
	}
	if rhs != nil {
		if rt := rhs.GetTypeSess(sessFromCG(cg)); rt != nil {
			// residual ERROR sticky — no invent float-op soft-continue past GetType residual
			if hasErrCG(cg) {
				return Stmt{}
			}
			if bt := rt.BaseTypeSess(sessFromCG(cg)); bt != nil && bt.IsFloatSess(sessFromCG(cg)) && !AssignOpWorksForFloat(op) {
				// residual ERROR sticky — no invent float-op soft-continue past BaseType residual
				if hasErrCG(cg) {
					return Stmt{}
				}
				op = AssignSimple
			} else if hasErrCG(cg) {
				// residual ERROR sticky — no invent soft-continue op past BaseType residual false
				return Stmt{}
			}
		} else if hasErrCG(cg) {
			// residual ERROR sticky — no invent soft-continue op past GetType residual nil
			return Stmt{}
		}
	}

	// StatementAssign.cpp:218–223 — CompatibleChecker → nullptr
	if CompatibleCheckExprsSess(sessFromCG(cg), opts, rhs, LhsAsExpressionSess(sessFromCG(cg), lhs)) {
		// residual ERROR sticky — no invent soft-assign past CompatibleCheck residual true
		if hasErrCG(cg) {
			return Stmt{}
		}
		noteErrCG(cg, ErrCompatibleCheck)
		return Stmt{}
	}
	// residual ERROR sticky — no invent soft-assign past CompatibleCheck residual false
	if hasErrCG(cg) {
		return Stmt{}
	}

	// StatementAssign.cpp:225 — merge_param_context(lhs_cg_context, true)
	cg.MergeParamContext(lhsCG, true)
	if hasErrCG(cg) || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		return Stmt{}
	}

	// StatementAssign.cpp:228 — make_possible_compound_assign (safe math flags/tmps)
	st := makePossibleCompoundAssign(*cg, opts, probs, r, typ, lhs, op, rhs, gensymFromVS(vs))
	// residual ERROR sticky — no invent ArrayAccess/complete assign past compound residual
	if hasErrCG(cg) {
		return Stmt{}
	}
	lhsIndir := 0
	if st.Lhs != nil {
		lhsIndir = st.Lhs.IndirectLevelSess(sessFromCG(cg))
		// residual ERROR sticky — no invent ArrayAccess past IndirectLevel residual hole
		if hasErrCG(cg) {
			return Stmt{}
		}
		isVol := opts.WrapVolatiles && st.Lhs.IsVolatileSess(sessFromCG(cg))
		// residual ERROR sticky — no invent ArrayAccess past IsVolatile residual hole
		if hasErrCG(cg) {
			return Stmt{}
		}
		if lhsIndir > 0 || isVol {
			st.ArrayAccess = st.Lhs.OutputSess(sessFromCG(cg), opts.WrapVolatiles)
			// residual ERROR sticky — no invent soft-empty ArrayAccess past Output residual
			if hasErrCG(cg) {
				return Stmt{}
			}
		}
	}
	// StatementAssign.cpp:make_random — does NOT update_fact_for_assign here.
	// Fact updates: ExpressionAssign.cpp after make_random (nested assigns), and
	// Statement::post_creation_analysis for eAssign (top-level). Early update
	// here double-applied lattice merges and diverged may-null (seed-2 e10107).
	// Write effects still come from Lhs visit + merge_param_context only —
	// do NOT NoteWrite(LhsVar) (seed2 e9238).
	return st
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
	st := Stmt{Kind: StmtAssign, AssignOp: op, Expr: rhs, Lhs: lhs, Rhs: rhs, StmID: AllocStmIDSess(sessFromCG(&cg))}
	if lhs != nil {
		st.LhsVar = lhs.Var
	}
	bop, ok := op.CompoundToBinaryOps()
	if !ok {
		// simple assign — StatementAssign ctor rhs(&expr)
		return st
	}
	// compound always maps to a live binary token; sticky no invent empty Binary shell
	opStr := bop.BinaryOpCSess(sessFromCG(&cg))
	if int(bop) < 0 || int(bop) >= MaxBinaryOp || opStr == "" {
		noteErrCG(&cg, ErrGeneric)
		return Stmt{}
	}
	lt := typ
	if lhs != nil {
		if t := lhs.GetTypeSess(sessFromCG(&cg)); t != nil {
			// residual ERROR sticky — no invent compound binary past GetType residual
			if hasErrCG(&cg) {
				return Stmt{}
			}
			lt = t
		} else if hasErrCG(&cg) {
			// residual ERROR sticky — no invent compound binary past GetType residual nil
			return Stmt{}
		}
	}
	var flags *SafeOpFlags
	var inv *Invocation
	if SafeAssign(op) {
		// StatementAssign.cpp:256–259 — dummy flags + FunctionInvocationBinary(bop, local_fs)
		flags = MakeDummyFlags()
		inv = &Invocation{IsStd: true, Binary: opStr, Safe: flags}
		inv.setOutOptsSess(sessFromCG(&cg), opts)
	} else {
		// StatementAssign.cpp:260–266 — make_random_binary + CreateFunctionInvocationBinary
		// SafeOpFlags.cpp:169–215 via make_random_binary(..., sOpAssign, bop)
		// always has RNG for non-safe compounds; sticky no invent nil-flags shell
		if r == nil {
			noteErrCG(&cg, ErrGeneric)
			return Stmt{}
		}
		flags = MakeRandomBinaryKindSess(sessFromCG(&cg), r, opts, probs, lt, lt, lt, SafeOpAssign, bop)
		// StatementAssign.cpp:260–262 — ERROR_GUARD(nullptr); no soft invent nil-flags compound
		if flags == nil || hasErrCG(&cg) {
			return Stmt{}
		}
		inv = &Invocation{IsStd: true, Binary: opStr, Safe: flags}
		inv.setOutOptsSess(sessFromCG(&cg), opts)
		// FunctionInvocationBinary.cpp:59–75 — always create tmps for safe_ops
		// assert(blk) when safe_ops — no soft invent compound without temps
		if SafeOpsBinary(opStr) {
			blk := cg.CurrentBlock()
			if blk == nil {
				// FunctionInvocationBinary.cpp:68 assert(blk)
				return Stmt{}
			}
			st1 := EInt
			if t := flags.LHSTypeSess(sessFromCG(&cg)); t != nil {
				if t.IsSimpleSess(sessFromCG(&cg)) {
					// residual ERROR sticky — no invent soft-tmp past IsSimple residual true
					if hasErrCG(&cg) {
						return Stmt{}
					}
					st1 = t.SimpleSess(sessFromCG(&cg))
				} else if hasErrCG(&cg) {
					// residual ERROR sticky — no invent soft-tmp past IsSimple residual false
					return Stmt{}
				}
			}
			st2 := st1
			if bop == BinLShift || bop == BinRShift {
				if t := flags.RHSTypeSess(sessFromCG(&cg)); t != nil {
					if t.IsSimpleSess(sessFromCG(&cg)) {
						if hasErrCG(&cg) {
							return Stmt{}
						}
						st2 = t.SimpleSess(sessFromCG(&cg))
					} else if hasErrCG(&cg) {
						return Stmt{}
					}
				}
			}
			st.Tmp1 = blk.CreateNewTmpVarSess(sessFromCG(&cg), st1)
			st.Tmp2 = blk.CreateNewTmpVarSess(sessFromCG(&cg), st2)
			inv.Tmp1, inv.Tmp2 = st.Tmp1, st.Tmp2
		}
	}
	st.SafeFlags = flags
	// StatementAssign.cpp:269–271 — add_operand ExpressionVariable(lhs); e.clone(); ExpressionFuncall
	lhsExpr := LhsAsExpressionSess(sessFromCG(&cg), lhs)
	if lhsExpr == nil {
		// C++ always has live Lhs; incomplete IR sticky empty assign
		noteErrCG(&cg, ErrGeneric)
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
// Incomplete Statement sticky nil (no invent soft-skip assign without RHS past hole).
func (st *Stmt) GetAssignRhs() *Expression {
	return st.GetAssignRhsSess(testAmbientSession)
}

// GetAssignRhsSess is GetAssignRhs with explicit session residual sticky.
func (st *Stmt) GetAssignRhsSess(s *Session) *Expression {
	// Statement always live for get_rhs; sticky incomplete no invent nil soft-skip
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if st.Rhs != nil {
		return st.Rhs
	}
	return st.Expr
}

// gensymFromVS returns &vs.Sym for create_new_tmp_var / gensym share with g_/l_.
// Nil VS is complete soft miss (callers use package Gensym) — not incomplete IR.
func gensymFromVS(vs *VariableSelector) *GenSym {
	if vs == nil {
		return nil
	}
	return &vs.Sym
}

// OutputAssignSimple mirrors StatementAssign::OutputSimple.
// StatementAssign.cpp:515–537 — lhs op rhs or pre/post incr forms.
// Incomplete Statement sticky empty (no invent empty assign shell past hole).
func OutputAssignSimple(st *Stmt, wrapVol bool) string {
	return OutputAssignSimpleSess(testAmbientSession, st, wrapVol)
}

func OutputAssignSimpleSess(s *Session, st *Stmt, wrapVol bool) string {
	// Statement always live at assign emit; sticky incomplete no invent empty token
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	lhs := assignLhsTextSess(s, st, wrapVol)
	if lhs == "" {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	// StatementAssign.cpp:515–537 — expr.Output always for ops that need RHS
	// sticky no soft invent "0" or "lhs = " empty RHS for incomplete IR
	if st.AssignOp.NeedNoRHS() {
		return st.AssignOp.AssignOpCSess(s, lhs, "")
	}
	if st.Expr == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	rhs := st.Expr.OutputSess(s)
	if rhs == "" {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	return st.AssignOp.AssignOpCSess(s, lhs, rhs)
}

// assignLhsText resolves LHS text for assign emit.
// Statement always live at assign emit; sticky empty (no invent bare RHS past hole).}

func assignLhsText(st *Stmt, wrapVol bool) string {
	return assignLhsTextSess(testAmbientSession, st, wrapVol)
}

func assignLhsTextSess(s *Session, st *Stmt, wrapVol bool) string {
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if st.ArrayAccess != "" {
		return st.ArrayAccess
	}
	if st.Lhs != nil {
		out := st.Lhs.OutputSess(s, wrapVol)
		// residual ERROR sticky — no invent soft-empty LHS past Lhs.Output residual
		if sessHasError(s) {
			return ""
		}
		return out
	}
	if st.LhsVar != nil {
		out := st.LhsVar.OutputLhsCOptsSess(s, false)
		// residual ERROR sticky — no invent soft-empty LHS past OutputLhsC residual
		if sessHasError(s) {
			return ""
		}
		return out
	}
	return ""
}

// OutputAssignAsExpr mirrors StatementAssign::OutputAsExpr.
// StatementAssign.cpp:542–625 — safe math rewrite for +=/-= when SafeFlags set.
// Uses process CGOptions (identify_wrappers); no soft invent Defaults().
// Incomplete Statement sticky empty (no invent empty assign-as-expr shell past hole).}

func OutputAssignAsExpr(st *Stmt, wrapVol bool) string {
	return OutputAssignAsExprSess(testAmbientSession, st, wrapVol)
}

// OutputAssignAsExprSess is OutputAssignAsExpr with Options/sticky from an explicit bag.
func OutputAssignAsExprSess(s *Session, st *Stmt, wrapVol bool) string {
	return OutputAssignAsExprOptsSess(s, st, wrapVol, sessOpts(s))
}

// OutputAssignAsExprOpts is OutputAsExpr with options for wrapper id filtering.
func OutputAssignAsExprOpts(st *Stmt, wrapVol bool, opts Options) string {
	return OutputAssignAsExprOptsSess(testAmbientSession, st, wrapVol, opts)
}

func OutputAssignAsExprOptsSess(s *Session, st *Stmt, wrapVol bool, opts Options) string {
	// Statement always live at OutputAsExpr; sticky incomplete no invent empty token
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	lhs := assignLhsTextSess(s, st, wrapVol)
	if lhs == "" {
		// incomplete LHS IR sticky — no invent bare RHS / safe rewrite
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	// StatementAssign.cpp: expr.Output always for ops that need RHS
	// sticky no invent "0" / bare lhs when Expr missing or Output empty
	var rhs string
	if st.Expr != nil {
		rhs = st.Expr.OutputOptsSess(s, opts)
		// residual ERROR sticky — no invent soft-empty RHS past Output residual hole
		if sessHasError(s) {
			return ""
		}
	}
	if !st.AssignOp.NeedNoRHS() && (st.Expr == nil || rhs == "") {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
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
				if assignLhsIsVolatileSess(s, st) {
					// residual ERROR sticky — no invent ccomp rewrite past IsVolatile residual
					if sessHasError(s) {
						return ""
					}
					return lhs + " = " + lhs + " " + bop.BinaryOpCSess(s) + " " + rhs
				}
				// residual ERROR sticky — no invent non-vol soft path past IsVolatile residual false
				if sessHasError(s) {
					return ""
				}
			}
			return OutputAssignSimpleSess(s, st, wrapVol)
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
				// incomplete IR sticky — no invent OutputSimple for broken compound map
				sessNoteError(s, ErrGeneric)
				return ""
			}
			fname := st.SafeFlags.BinaryFuncNameSess(s, bop.BinaryOpCSess(s))
			if fname == "" {
				// SafeOpFlags.cpp assert empty name sticky; no invent bare +=
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return ""
			}
			id := SafeOpFlagsToIDSess(s, fname)
			// don't use wrapper if filtered out by --safe-math-wrapper
			if !SafeMathWrapperAllowed(opts, id) {
				return OutputAssignSimpleSess(s, st, wrapVol)
			}
			// StatementAssign.cpp:595–598 — expr.Output always (live Expression*)
			if rhs == "" && !st.AssignOp.NeedNoRHS() {
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
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
			// StatementAssign.cpp:618–619 — assert(false) sticky; no soft invent OutputSimple
			sessNoteError(s, ErrGeneric)
			return ""
		}
	}
	return OutputAssignSimpleSess(s, st, wrapVol)
}

// assignLhsIsVolatile reports LHS volatile for OutputAsExpr ccomp rewrite.
// StatementAssign.cpp:552 — lhs.is_volatile().
// Statement always live; sticky true (no invent non-vol soft-skip ccomp path past hole).
func assignLhsIsVolatile(st *Stmt) bool {
	return assignLhsIsVolatileSess(testAmbientSession, st)
}

func assignLhsIsVolatileSess(s *Session, st *Stmt) bool {
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if st.Lhs != nil {
		vol := st.Lhs.IsVolatileSess(s)
		// residual ERROR sticky — no invent non-vol soft-skip past Lhs IsVolatile residual
		if sessHasError(s) {
			return true
		}
		return vol
	}
	if st.LhsVar == nil {
		return false
	}
	vol := st.LhsVar.IsVolatileSess(s)
	// residual ERROR sticky — no invent non-vol soft-skip past LhsVar IsVolatile residual
	if sessHasError(s) {
		return true
	}
	return vol
}

// expressionQualifiers mirrors Expression::get_qualifiers for qfer seed.
// Uses Expression.GetQualifiers (ExpressionVariable/Assign/Funcall/Comma).
// Expression always live at qfer seed; sticky nil (no invent empty seed past hole).

func expressionQualifiers(e *Expression) *CVQualifiers {
	return expressionQualifiersSess(testAmbientSession, e)
}

// expressionQualifiersSess is expressionQualifiers with explicit session residual sticky.
func expressionQualifiersSess(s *Session, e *Expression) *CVQualifiers {
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	q := e.GetQualifiersSess(s)
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
	// incomplete call / shells sticky (no soft invent visit success / soft re-pick)
	if e == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	switch e.Term {
	case TermConstant:
		// Constant.cpp always has live value string; incomplete Con sticky
		if e.Con == nil || e.Con.Value == "" {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		return true
	case TermVariable:
		return cg.VisitFactsExpressionVariable(e, opts)
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		if !VisitFactsExpression(e.CommaLHS, cg, opts) {
			return false
		}
		// residual ERROR sticky — no invent soft-continue RHS past LHS visit residual
		if hasErrCG(cg) {
			return false
		}
		ok := VisitFactsExpression(e.CommaRHS, cg, opts)
		// residual ERROR sticky — no invent visit success past RHS visit residual
		if hasErrCG(cg) {
			return false
		}
		return ok
	case TermAssignment:
		if e.Assign == nil {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		return VisitFactsStatementAssign(e.Assign, cg, opts)
	case TermFunction:
		if e.Invoke == nil {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		return VisitFactsInvocation(e.Invoke, cg, opts)
	default:
		// unknown term hard IR sticky
		noteErrCG(cg, ErrGeneric)
		return false
	}
}

// VisitFactsInvocation mirrors FunctionInvocation::visit_facts.
// FunctionInvocation.cpp:502–555 — ordered params (unordered path available but
// upstream sets unordered=false); then ALWAYS revisit user callees.
// NeedsRevisit / static feffect is build_invocation only
// (FunctionInvocationUser.cpp:272–297). Gating visit_facts on NeedsRevisit
// invents soft skip of body re-analysis (seed-2 e10107 lattice path).
// Binary &&/|| use FunctionInvocationBinary::visit_facts short-circuit merge.
func VisitFactsInvocation(fi *Invocation, cg *CGContext, opts Options) bool {
	// C++ always has live FunctionInvocation*; nil sticky.
	// FunctionInvocation.cpp:502–555 — visit_facts does NOT consult `failed`.
	// Generation-time Failed must not invent re-analysis failure (would strip
	// compound containers and drop mid-gen may-null — seed-2 e10107 path).
	if fi == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
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
			// incomplete GlobalFacts sticky (no invent cleaned visit / soft re-pick)
			if !FactsComplete(cg.FM.GlobalFacts) {
				noteErrCG(cg, ErrGeneric)
				return false
			}
			facts = CloneFactSliceSess(sessFromCG(cg), cg.FM.GlobalFacts)
			// residual ERROR sticky — no invent soft-visit past CloneFactSlice residual
			if hasErrCG(cg) {
				return false
			}
		}
		if !fi.VisitUnorderedParams(&facts, cg, opts) {
			return false
		}
		if cg.FM != nil {
			cg.FM.SetGlobalFacts(facts, "auto_statement_assign_978")
		}
	} else {
		running := cg.EffectContext().detachMaps()
		for i, arg := range fi.Args {
			// FunctionInvocation.cpp: param_value[i] always non-null after ERROR_GUARD sticky
			if arg == nil {
				_ = i
				noteErrCG(cg, ErrGeneric)
				return false
			}
			paramAccum := EmptyEffect()
			// CGContext.cpp:74–82 — (cgc, running, &param_accum): effect_stm default, curr_rhs null
			paramCG := cg.CloneSubcontext()
			paramCG.effectContext = running
			paramCG.EffectAccum = &paramAccum
			paramCG.EffectStm = EmptyEffect()
			paramCG.CurrRHS = nil
			if !VisitFactsExpression(arg, &paramCG, opts) {
				return false
			}
			// residual ERROR sticky — no invent soft-continue later args past visit residual
			if hasErrCG(cg) {
				return false
			}
			// Incomplete param accum sticky (no invent visit more args under incomplete)
			running = running.AddEffectSess(sessFromCG(cg), paramAccum)
			// residual ERROR sticky — no invent soft-continue later args past AddEffect residual
			if hasErrCG(cg) {
				return false
			}
			if !EffectComplete(running) {
				noteErrCG(cg, ErrGeneric)
				return false
			}
			// merge_param_context; include_lhs for std ops only
			cg.MergeParamContext(paramCG, !isFuncCall)
			// residual ERROR sticky — no invent soft-continue later args past MergeParam residual
			if hasErrCG(cg) {
				return false
			}
			if !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
		}
	}
	if isFuncCall {
		// FunctionInvocation.cpp:530–551 — visit_facts ALWAYS revisits user callees.
		// NeedsRevisit / static feffect is build_invocation only
		// (FunctionInvocationUser.cpp:272–297). Do not invent soft-skip of body
		// re-analysis during fixed-point (seed-2 e10107 may-null path).
		// Body nil: soft analysis fail (no sticky) — C++ always has a Block* body;
		// incomplete IR here is an analysis miss, not a hard generation ERROR.
		if fi.User.Body == nil {
			return false
		}
		if cg.FM == nil {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		if !FactsComplete(cg.FM.GlobalFacts) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		// FunctionInvocation.cpp:536–541 —
		//   Effect effect_accum;
		//   CGContext new_context(cg_context, func_call->func,
		//                         cg_context.get_effect_context(), &effect_accum);
		//   ok = func_call->revisit(inputs, new_context);
		// Must not pass the parent cg: parent EffectAccum/CurrRHS/CurrentFunc would
		// pollute nested body visit (CheckRead/Write, call_chain) and falsely fail
		// revisit under fresh param lattices (seed-2 func_49 visited=7 / e37241).
		effectAccum := EmptyEffect()
		newCG := cg.CloneSubcontext()
		newCG.CurrentFunc = fi.User
		newCG.EffectAccum = &effectAccum
		newCG.EffectStm = EmptyEffect()
		newCG.CurrRHS = nil
		newCG.ExprDepth = 0
		newCG.BlkDepth = 0
		newCG.ExtendCallChain(*cg)
		// residual ERROR sticky — no invent soft-revisit past ExtendCallChain residual
		if hasErrCG(cg) {
			return false
		}
		// FunctionInvocation.cpp:539–540 — revisit(inputs, new_context)
		if !RevisitUserInvocation(fi, &cg.FM.GlobalFacts, &newCG, opts) {
			return false
		}
		if !FactsComplete(cg.FM.GlobalFacts) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		// FunctionInvocation.cpp:542–550 — assert(curr_blk);
		// add_visible_effect(*new_context.get_effect_accum(), curr_blk);
		// feffect.add_external_effect(*new_context.get_effect_accum(), call_chain).
		// Revisit already folded body map_stm_effect into newCG.EffectAccum.
		// curr_blk is set in stm_visit_facts (Statement.cpp:612), not stack-top alone.
		blk := cg.AnalysisBlock()
		if blk == nil {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		if !EffectComplete(effectAccum) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		cg.AddVisibleEffectAt(effectAccum, blk)
		if hasErrCG(cg) {
			return false
		}
		if !EffectComplete(fi.User.FEffect) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		fi.User.FEffect = fi.User.FEffect.AddExternalEffectWithCallersSess(sessFromCG(cg), effectAccum, cg.CallChain)
		if !EffectComplete(fi.User.FEffect) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// StatementAssign.cpp always has live Lhs and Expression* sticky
	if st.Expr == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// StatementAssign.cpp:362–367 — RHS in its own accum context
	// Incomplete ambient/stm/accum sticky (no invent visit under incomplete shell)
	runningEff := cg.EffectContext().detachMaps()
	if !EffectComplete(runningEff) {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	if !EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	if cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum) {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	rhsAccum := EmptyEffect()
	rhsCG := cg.CloneSubcontext()
	rhsCG.effectContext = runningEff
	rhsCG.EffectAccum = &rhsAccum
	// CGContext.cpp:74–82 — (cgc, running_eff, &rhs_accum): effect_stm() default empty,
	// curr_rhs(nullptr). Do not inherit parent effect_stm: Lhs::ptr_modified_in_rhs
	// (Lhs.cpp:240–261) must see only this assign's RHS writes, not sibling effects
	// already on the parent (e.g. left of && before a nested ExpressionAssign).
	// Generation path (MakeRandomAssign) already uses EmptyEffect here.
	rhsCG.EffectStm = EmptyEffect()
	rhsCG.CurrRHS = nil

	if !VisitFactsExpression(st.Expr, &rhsCG, opts) {
		return false
	}
	// StatementAssign.cpp:372–375 — compound: LHS sees RHS effect
	// Incomplete folds sticky (no invent LHS visit under incomplete running)
	if st.AssignOp != AssignSimple {
		runningEff = runningEff.AddEffectSess(sessFromCG(cg), rhsAccum)
		// residual ERROR sticky — no invent soft-continue LHS visit past AddEffect residual
		if hasErrCG(cg) {
			return false
		}
		if !EffectComplete(runningEff) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
	}
	cg.MergeParamContext(rhsCG, true)
	if !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		return false
	}
	// StatementAssign.cpp:377 — write_var_set(rhs_accum.get_lhs_write_vars())
	// IncompleteVariables → WriteVarSet IncompleteEffect sticky
	if lw := rhsAccum.LhsWriteVarsSess(sessFromCG(cg)); !VariablesComplete(lw) || len(lw) > 0 {
		// residual ERROR sticky — no invent soft-skip WriteVarSet past LhsWriteVars residual
		if hasErrCG(cg) {
			return false
		}
		runningEff = runningEff.WriteVarSetSess(sessFromCG(cg), lw)
		// residual ERROR sticky — no invent soft-continue LHS past WriteVarSet residual
		if hasErrCG(cg) || !EffectComplete(runningEff) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
	} else if hasErrCG(cg) {
		// residual ERROR sticky — no invent soft-empty LhsWriteVars past residual hole
		return false
	}

	// StatementAssign.cpp:379–384 — LHS context
	lhsAccum := EmptyEffect()
	lhsCG := cg.CloneSubcontext()
	lhsCG.effectContext = runningEff
	lhsCG.EffectAccum = &lhsAccum
	// Effect.cpp:84–89 deep copy (same as generation path).
	lhsCG.EffectStm = rhsCG.EffectStm.detachMaps()
	lhsCG.CurrRHS = st.Expr

	var lhsVar *Variable
	indir := 0
	if st.Lhs != nil {
		if !lhsCG.VisitFactsLhs(st.Lhs, opts) {
			return false
		}
		lhsVar = st.Lhs.Var
		indir = st.Lhs.IndirectLevelSess(sessFromCG(cg))
		// residual ERROR sticky — no invent visit success past IndirectLevel residual
		if hasErrCG(cg) {
			return false
		}
	} else if st.LhsVar != nil {
		tmp := &Lhs{Var: st.LhsVar, Type: st.LhsVar.Type}
		if !lhsCG.VisitFactsLhs(tmp, opts) {
			return false
		}
		lhsVar = st.LhsVar
	} else {
		// incomplete assign IR sticky (no invent visit success without LHS)
		noteErrCG(cg, ErrGeneric)
		return false
	}
	cg.MergeParamContext(lhsCG, true)
	if !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		return false
	}

	// StatementAssign.cpp:386 — FactMgr::update_fact_for_assign(this, inputs)
	// uses get_rhs() (canonized ExpressionFuncall for compounds)
	if cg.FM != nil && lhsVar != nil {
		// Statement::stm_id always live; StmID 0 sticky
		if StmIDUnset(st.StmID) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		// StatementAssign.cpp:386 — FactMgr::update_fact_for_assign(this, inputs)
		// Go: GlobalFacts is the visit working set (cloned into by StmVisitFacts).
		// FactUnion.cpp:133 — pass Lhs::get_type() (desired), not Variable.Type.
		// Soft invent UpdateFactForAssign(var,…) missed (*union*) eUnionWrite transfer.
		var lhsWant *Type
		if st.Lhs != nil {
			lhsWant = st.Lhs.GetTypeSess(sessFromCG(cg))
			if hasErrCG(cg) {
				return false
			}
		}
		_ = cg.FM.UpdateFactForAssignWant(lhsVar, indir, lhsWant, st.GetAssignRhsSess(sessFromCG(cg)))
		// incomplete assign sticky (no invent visit success)
		if !FactsComplete(cg.FM.GlobalFacts) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		// Incomplete EffectStm sticky
		if !EffectComplete(cg.EffectStm) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		// StatementAssign.cpp:388–389 — map_stm_effect only; set_fact_out is
		// validate_and_update_facts / stm_visit_facts callers (not here).
		cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	return true
}
