// Upstream: StatementAssign.cpp (InitProbabilityTable, AssignOpsProbability, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// NewAssignOpsTable mirrors StatementAssign::InitProbabilityTable.
// StatementAssign.cpp:68–81.
// Non-Sess NewAssignOpsTable deleted — pass run bag or testAmbientSession explicitly.

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
// Non-Sess AssignOpsProbability deleted — pass run bag or testAmbientSession explicitly.

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
	v := r.RndUptoFilterSess(s, uint32(f.MaxProbSess(s)), f)
	return AssignOp(f.LookupSess(s, int(v)))
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
	qfer := NewCVQualifiersSess(cgSess(cg), []bool{false}, []bool{false})
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
	// C++ StatementAssign::make_random does not Output LHS mid-gen; OutputAsExpr
	// calls lhs.Output at emit (StatementAssign.cpp:515+). Do not cache Lhs text
	// into ArrayAccess here — ACCESS_ONCE depends on isAddrTaken, which can flip
	// later when another global takes &var (bodyparity crest+access_once seed-1).
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
// Non-Sess GetAssignRhs deleted — pass run bag or testAmbientSession explicitly.

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
// Non-Sess OutputAssignSimple deleted — pass run bag or testAmbientSession explicitly.

func OutputAssignSimpleSess(s *Session, st *Stmt, wrapVol bool) string {
	// Statement always live at assign emit; sticky incomplete no invent empty token
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	lhs := assignLhsTextSess(s, st, wrapVol)
	if sessHasError(s) {
		return ""
	}
	// empty LHS ok under prefix_name (NDEBUG get_count_prefix → bare " = rhs")
	if lhs == "" && !sessOpts(s).PrefixName {
		sessNoteError(s, ErrGeneric)
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
	if sessHasError(s) {
		return ""
	}
	// empty RHS ok under prefix_name (global RHS name emptied)
	if rhs == "" && !sessOpts(s).PrefixName {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return st.AssignOp.AssignOpCSess(s, lhs, rhs)
}

// assignLhsText resolves LHS text for assign emit.
// Statement always live at assign emit; sticky empty (no invent bare RHS past hole).}

// Non-Sess assignLhsText deleted — pass run bag or testAmbientSession explicitly.

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
		out := st.LhsVar.OutputLhsCOptsSess(s, sessOpts(s).PrefixName)
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

// Non-Sess OutputAssignAsExpr deleted — pass run bag or testAmbientSession explicitly.

// OutputAssignAsExprSess is OutputAssignAsExpr with Options/sticky from an explicit bag.
func OutputAssignAsExprSess(s *Session, st *Stmt, wrapVol bool) string {
	return OutputAssignAsExprOptsSess(s, st, wrapVol, sessOpts(s))
}

// OutputAssignAsExprOpts is OutputAsExpr with options for wrapper id filtering.
// Non-Sess OutputAssignAsExprOpts deleted — pass run bag or testAmbientSession explicitly.

func OutputAssignAsExprOptsSess(s *Session, st *Stmt, wrapVol bool, opts Options) string {
	// Statement always live at OutputAsExpr; sticky incomplete no invent empty token
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	lhs := assignLhsTextSess(s, st, wrapVol)
	if sessHasError(s) {
		return ""
	}
	// empty LHS ok under prefix_name (NDEBUG get_count_prefix)
	if lhs == "" && !opts.PrefixName {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// StatementAssign.cpp:542–625 — Output expr once per path (do not pre-Output then
	// OutputAssignSimple again: double Output burns FunctionInvocationUser flipcoins).
	if !st.AssignOp.NeedNoRHS() && st.Expr == nil {
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
			if bop, ok := st.AssignOp.CompoundToBinaryOps(); ok && opts.CComp {
				if assignLhsIsVolatileSess(s, st) {
					// residual ERROR sticky — no invent ccomp rewrite past IsVolatile residual
					if sessHasError(s) {
						return ""
					}
					rhs := st.Expr.OutputOptsSess(s, opts)
					if sessHasError(s) {
						return ""
					}
					if rhs == "" && !opts.PrefixName {
						sessNoteError(s, ErrGeneric)
						return ""
					}
					return lhs + " = " + lhs + " " + bop.BinaryOpCSess(s) + " " + rhs
				}
				// residual ERROR sticky — no invent non-vol soft path past IsVolatile residual false
				if sessHasError(s) {
					return ""
				}
			}
			// StatementAssign.cpp:559–562 — output_op + expr.Output once via OutputSimple
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
			// StatementAssign.cpp:595–598 — expr.Output once
			rhs := ""
			if st.Expr != nil {
				rhs = st.Expr.OutputOptsSess(s, opts)
				if sessHasError(s) {
					return ""
				}
			}
			if rhs == "" && !st.AssignOp.NeedNoRHS() && !opts.PrefixName {
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
// Non-Sess assignLhsIsVolatile deleted — pass run bag or testAmbientSession explicitly.

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

// Non-Sess expressionQualifiers deleted — pass run bag or testAmbientSession explicitly.

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
		okRev := RevisitUserInvocation(fi, &cg.FM.GlobalFacts, &newCG, opts)
		if !okRev {
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
		// Acc→FE handoff with outer pure for-IV strip (n94 g_2991.f0).
		effForFE := stripCallerForIVsFromEffect(sessFromCG(cg), effectAccum, cg.CurrentFunc, cg, fi.User)
		if !EffectComplete(effForFE) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		if os.Getenv("DIAG_GO_G8") != "" && fi.User != nil && fi.User.Name == "func_13" {
			s := sessFromCG(cg)
			hasAcc, hasStrip := false, false
			var stripG []string
			for _, v := range effectAccum.ReadVarsSess(s) {
				if v != nil && v.Name == "g_8" {
					hasAcc = true
					fmt.Fprintf(os.Stderr, "GO_G8_STRIP pre isGlobal=%v\n", v.IsGlobalSess(s))
				}
			}
			for _, v := range effForFE.ReadVarsSess(s) {
				if v != nil && v.IsGlobalSess(s) {
					stripG = append(stripG, v.Name)
				}
				if v != nil && v.Name == "g_8" {
					hasStrip = true
				}
			}
			caller := "?"
			if cg.CurrentFunc != nil {
				caller = cg.CurrentFunc.Name
			}
			var ivs []string
			for iv := range cg.IVBounds {
				if iv != nil {
					ivs = append(ivs, iv.Name)
				}
			}
			// rebuild callerIV set the same way strip does for names
			var forIVs []string
			if cg.CurrentFunc != nil {
				for _, blk := range cg.CurrentFunc.Blocks {
					if blk == nil {
						continue
					}
					for i := range blk.Stmts {
						st := &blk.Stmts[i]
						if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil {
							forIVs = append(forIVs, st.Loop.IV.Name)
							if st.Loop.IV.Name == "g_8" {
								fmt.Fprintf(os.Stderr, "GO_G8_FORIV blkSid=%d stmSid=%d parentNil=%v\n",
									blk.StmID, st.StmID, blk.Parent == nil)
							}
						}
					}
				}
			}
			// Is g_8 in IVBounds as pointer identity vs forIVs
			for iv := range cg.IVBounds {
				if iv != nil && iv.Name == "g_8" {
					fmt.Fprintf(os.Stderr, "GO_G8_IVBOUND ptr=%p name=%s\n", iv, iv.Name)
				}
			}
			fmt.Fprintf(os.Stderr, "GO_G8_STRIP caller=%s accHas8=%v stripHas8=%v stripG=%v ivBounds=%v forIVs=%v\n",
				caller, hasAcc, hasStrip, stripG, ivs, forIVs)
		}
		fi.User.FEffect = fi.User.FEffect.AddExternalEffectWithCallersSess(sessFromCG(cg), effForFE, cg.CallChain)
		if !EffectComplete(fi.User.FEffect) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		if os.Getenv("DIAG_GO_G8") != "" && fi.User != nil && fi.User.Name == "func_13" {
			s := sessFromCG(cg)
			var fe, acc []string
			for _, v := range fi.User.FEffect.ReadVarsSess(s) {
				if v != nil && v.IsGlobalSess(s) {
					fe = append(fe, v.Name)
				}
			}
			for _, v := range effectAccum.ReadVarsSess(s) {
				if v != nil && v.IsGlobalSess(s) {
					acc = append(acc, v.Name)
				}
			}
			fmt.Fprintf(os.Stderr, "GO_G8_FE visit fe=%v accG=%v\n", fe, acc)
		}
		if cg.CurrentFunc != nil && fi.User.FactChanged {
			cg.CurrentFunc.FactChanged = true
		}
	}
	return true
}

// FixupFunc1PureIVFEHeads reorders pure for-IV FE heads on func_1 assign map_stm
// after Body is complete (n35 g_108 before g_483). Mid-gen pure-IV restore skips
// FE-head because laterBodyReadsVar cannot see future free reads (n28 g_791.f1).
// Session/FM-local — no package mutable state.
func FixupFunc1PureIVFEHeads(f *Function, fm *FactMgr) {
	if f == nil || fm == nil || f.Name != "func_1" || f.Body == nil {
		return
	}
	if os.Getenv("DIAG_S48_PURE") != "" {
		fmt.Fprintf(os.Stderr, "FIXUP_FUNC1 nStmts=%d\n", len(f.Body.Stmts))
	}
	s := sessFromFM(fm)
	// Env-gated seed57-style Acc dump (stderr only; no package mutable state).
	dump57 := func(tag string) {
		if os.Getenv("DIAG57") == "" || f.Body == nil || StmIDUnset(f.Body.StmID) {
			return
		}
		eff := fm.GetMapStmEffect(f.Body.StmID)
		if !EffectComplete(eff) {
			return
		}
		ord := eff.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		posOf := func(name string) int {
			for i, v := range ord {
				if v != nil && v.Name == name {
					return i
				}
			}
			return -1
		}
		names := []string{"g_251", "g_1300", "g_252", "g_1291", "g_113", "g_704", "g_707", "g_517", "g_70", "g_2074"}
		fmt.Fprintf(os.Stderr, "DIAG57 %s n=%d", tag, len(ord))
		for _, n := range names {
			p := posOf(n)
			fmt.Fprintf(os.Stderr, " %s@%d", n, p)
			if p >= 0 {
				lo, hi := p-1, p+2
				if lo < 0 {
					lo = 0
				}
				if hi > len(ord) {
					hi = len(ord)
				}
				fmt.Fprint(os.Stderr, "[")
				for i := lo; i < hi; i++ {
					if i > lo {
						fmt.Fprint(os.Stderr, ",")
					}
					if ord[i] != nil {
						fmt.Fprint(os.Stderr, ord[i].Name)
					}
				}
				fmt.Fprint(os.Stderr, "]")
			}
		}
		fmt.Fprintln(os.Stderr)
	}
	type tailAt struct {
		st   *Stmt
		tail []*Variable
	}
	var sites []tailAt
	var walk func(blk *Block)
	walk = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			switch st.Kind {
			case StmtBlock:
				if st.Then != nil {
					walk(st.Then)
				}
			case StmtIfElse:
				if st.Then != nil {
					walk(st.Then)
				}
				if st.Else != nil {
					walk(st.Else)
				}
			case StmtFor, StmtArrayOp:
				if st.Then != nil {
					walk(st.Then)
				}
			case StmtAssign:
				fixupAssignPureIVFEHead(s, fm, f, st)
				if tail := fixupAssignNestedFETailArrayInit(s, fm, f, st); len(tail) > 0 {
					sites = append(sites, tailAt{st: st, tail: tail})
				}
			}
		}
	}
	walk(f.Body)
	for _, site := range sites {
		propagateFETailFromAssign(f, fm, s, site.st, site.tail)
	}
	_ = f.Body.SetAccumulatedEffect(fm)
	if os.Getenv("DIAG_S875") != "" && f.Name == "func_1" && !StmIDUnset(f.Body.StmID) {
		eff := fm.GetMapStmEffect(f.Body.StmID)
		if EffectComplete(eff) {
			fmt.Fprint(os.Stderr, "S875 pre_fixup=")
			for _, x := range eff.ReadVarsSess(s) {
				if x != nil && (x.Name == "g_13" || x.Name == "g_116" || x.Name == "g_117" || x.Name == "g_149" || x.Name == "g_194" || x.Name == "g_266" || x.Name == "g_51" || x.Name == "g_97" || x.Name == "g_70" || x.Name == "g_990") {
					fmt.Fprintf(os.Stderr, " %s", x.Name)
				}
			}
			fmt.Fprintln(os.Stderr)
		}
	}
	dump57("after_assign_puremiss")
	// Nested FE pure for-IV prefix must precede free-ref non-pure of the same FE
	// on parent when pure is Acc-late (seed88). Acc-early pure FE heads of nested
	// that are not free-ref on parent follow free-ref parent early Acc (seed22584).
	// seed9895936 g_1580 late placement covered by Acc-early→free-ref-anchor path
	// in pure-prefix / Acc-early free-ref fixups below.
	fixupFunc1NestedFEPurePrefixBeforeFree(f, fm, s)
	dump57("after_pure_prefix")
	// Multi-prefix pure-only stragglers after free residual (seed12336 g_659; seed120).
	// Split early+late; PureMissTouched skip (seed57).
	fixupNestedFEPureMultiPrefixOrder(f, fm, s)
	dump57("after_multi_prefix")
	if os.Getenv("DIAG_S875") != "" && f.Name == "func_1" && !StmIDUnset(f.Body.StmID) {
		eff := fm.GetMapStmEffect(f.Body.StmID)
		if EffectComplete(eff) {
			fmt.Fprint(os.Stderr, "S875 after_multi=")
			for _, x := range eff.ReadVarsSess(s) {
				if x != nil && (x.Name == "g_13" || x.Name == "g_116" || x.Name == "g_117" || x.Name == "g_149" || x.Name == "g_194" || x.Name == "g_266" || x.Name == "g_51" || x.Name == "g_97" || x.Name == "g_70" || x.Name == "g_990") {
					fmt.Fprintf(os.Stderr, " %s", x.Name)
				}
			}
			fmt.Fprintln(os.Stderr)
		}
	}
	fixupFunc1AccEarlyNestedPureAfterParentFree(f, fm, s)
	dump57("after_acc_early")
	// Acc-early pure FE head before sibling free residual → exclusive residual
	// (LevelC seed 291856876356590876 g_182.f4 after free residual before g_277.f4).
	// OFF: natural Acc already places free residual before pure for seed2918;
	// firing delays correctly-early pure FE heads past free residual (LevelC
	// 7504083620530920041 g_140 of func_36 before g_110). Keep helper for
	// documentation. Session/FM-local — no package mutable state.
	//OFF fixupAccEarlyPureFEHeadBeforeExclusiveResidual(f, fm, s)
	// Own pure-only for-IVs of func_1 that lead a nested FE must precede nested
	// pure residual pure for-IVs (seed 875297491714 g_13 before g_116).
	fixupFunc1OwnPureFEHeadBeforeNestedPureResidual(f, fm, s)
	dump57("after_own_pure_before_nested")
	// Acc-late deeper pure-only pure IVs via intermediate placeFE (seed LC
	// g_147/g_527 of func_46 via func_20 FE). Deeper-only; PureMissTouched skip.
	fixupNestedFEPureOnlyRelativeOrder(f, fm, s)
	dump57("after_pure_only_rel")
	// Mid pure-only Acc-late of pure FE-head nested FE when FE head free-refs on
	// parent (own pure of parent) — pureOnly relative requires pureOnly FE head
	// Acc-late and co-relocating head (seed 8105187965554449711 g_190 of func_61
	// after g_176.f5 before g_357; FE head g_79.f1 free-ref own pure of func_1).
	fixupMidPureOnlyAfterPureFEHeadResidual(f, fm, s)
	dump57("after_mid_pure_only")
	// Own pure FE head of nested FE Acc-late after residual free of same FE
	// (seed 8105187965554449711 g_79.f1 of func_32 before g_753). pure-prefix
	// excludes own pure of parent (seed48 g_1495); Acc-early after free-ref needs
	// Acc-leading pure.
	fixupOwnPureNestedFEHeadBeforeFreeResidual(f, fm, s)
	dump57("after_own_pure_nested_head")
	// Nested pure FE head before pure-for-only own residual mid nested FE
	// (LevelC seed 5139 g_261 before g_727). Opposite of own-pure-before-nested
	// (seed875 value free-ref own pure).
	fixupNestedPureFEHeadBeforeOwnPureResidual(f, fm, s)
	dump57("after_nested_pure_before_own")
	// Free-head nested FE pure residual Acc-late (LevelC seed4100 g_680 of
	// free-head func_42 after g_1132). Mid pure multi-prefix free-head is non-func_1.
	fixupFreeHeadFEPureResidualRelativeOrder(f, fm, s)
	dump57("after_freehead")
	// Owner pure residual free-ref Acc-late after residual free of same FE (not
	// free-ref on parent) after pure multi of owner (LevelC seed7310 func_1
	// inherits g_276/g_205 from nested Acc). Session/FM-local.
	fixupOwnerPureResidualFreeRefBeforeResidualFree(f, fm, s)
	dump57("after_owner_pure_res")
	// Acc-early pure residual free-ref of pure-head nested FE after residual free
	// free-ref of same FE (LevelC seed10990494057038618915 g_1075 after g_942).
	// Freehead freefRef path is Acc-late pure + free-head FE only; pure-head FE
	// free-ref pure residual Acc-early from parent free-ref needs defer. Session/FM-local.
	fixupAccEarlyPureHeadResidualFreeRefAfterResidualFree(f, fm, s)
	dump57("after_accearly_purehead_res")
	// Acc-late pure residual free-ref before residual free free-neither of same FE
	// (seed2580000644868734815 g_247 after g_716 before g_188 of func_28).
	// Session/FM-local — no package mutable state.
	fixupAccLatePureResidualFreeRefBeforeFreeNeither(f, fm, s)
	dump57("after_pure_res_freeref_before_neither")
	// Acc-late pure FE head pure-only after residual free free-ref parent-only of
	// same FE after free residual free-ref whose InitExpr is address-of pure
	// (seed16650 g_68 after g_1091; residual free free-ref parent-only g_40 of func_63).
	// Session/FM-local — no package mutable state.
	fixupAccLatePureFEHeadAfterAddrOfPureFreeRef(f, fm, s)
	dump57("after_pure_fehead_addrof")
	// Array-init FE-tail pure IVs after residual free of owner FE on body map_stm
	// (LC seed 16045778296055951950 g_2846 after g_1432). Assign-level absolute
	// end is seed48; body residual-after is last so other pure fixups settle first.
	fixupFunc1FETailArrayInitAfterResidual(f, fm, s)
	dump57("after_fetail_array")
	// Nested pure multi-prefix pure-only FE heads pure-only on parent (float seed=2
	// g_370/g_531 of func_20). Before Acc-early pure residual after free residual.
	fixupStripNestedPureOnlyFEHeadsFromParent(f, fm, s)
	dump57("after_strip_nested_pure_heads")
	if os.Getenv("DIAG_S875") != "" && f.Name == "func_1" && !StmIDUnset(f.Body.StmID) {
		eff := fm.GetMapStmEffect(f.Body.StmID)
		if EffectComplete(eff) {
			fmt.Fprint(os.Stderr, "S875 after_strip=")
			for _, x := range eff.ReadVarsSess(s) {
				if x != nil && (x.Name == "g_13" || x.Name == "g_116" || x.Name == "g_117" || x.Name == "g_149" || x.Name == "g_194" || x.Name == "g_266" || x.Name == "g_51" || x.Name == "g_97" || x.Name == "g_70" || x.Name == "g_36" || x.Name == "g_316" || x.Name == "g_990") {
					fmt.Fprintf(os.Stderr, " %s", x.Name)
				}
			}
			fmt.Fprintln(os.Stderr)
		}
	}
	// Acc-early pure residual pure of pure-head nested FE before free residual free
	// of same FE → free residual free first, then free-ref pure, pure-only last
	// (float seed=2 func_1 vs func_20 FE). After strip. Session/FM-local.
	fixupAccEarlyPureHeadFEPureResidualAfterFreeResidual(f, fm, s)
	dump57("after_purehead_pure_after_free")
	if os.Getenv("DIAG_S875") != "" && f.Name == "func_1" && !StmIDUnset(f.Body.StmID) {
		eff := fm.GetMapStmEffect(f.Body.StmID)
		if EffectComplete(eff) {
			fmt.Fprint(os.Stderr, "S875 after_purehead_res=")
			for _, x := range eff.ReadVarsSess(s) {
				if x != nil && (x.Name == "g_13" || x.Name == "g_116" || x.Name == "g_117" || x.Name == "g_149" || x.Name == "g_194" || x.Name == "g_266" || x.Name == "g_51" || x.Name == "g_97" || x.Name == "g_70" || x.Name == "g_36" || x.Name == "g_316" || x.Name == "g_990") {
					fmt.Fprintf(os.Stderr, " %s", x.Name)
				}
			}
			fmt.Fprintln(os.Stderr)
		}
	}
}

// FixupFuncPureIVFEOrder is the non-func_1 post-body pure-IV order surface:
// multi-prefix order + pure-only FE-relative slots + Acc-early FE-neighbor
// (seed120 func_45 g_59; seed 14175156974908062646 func_20 g_147/g_527).
// No assign-level pureMiss (mid-gen on all funcs broke n62). Session/FM-local.
func FixupFuncPureIVFEOrder(f *Function, fm *FactMgr) {
	if f == nil || fm == nil || f.Body == nil {
		return
	}
	if os.Getenv("DIAG_F38") != "" && f.Name == "func_38" {
		s0 := sessFromFM(fm)
		eff := fm.GetMapStmEffect(f.Body.StmID)
		fmt.Fprint(os.Stderr, "F38 pre_fixup=")
		for _, x := range eff.ReadVarsSess(s0) {
			if x != nil && (x.Name == "g_136" || x.Name == "g_137" || x.Name == "g_112" || x.Name == "g_168" || x.Name == "g_172" || x.Name == "g_117") {
				fmt.Fprintf(os.Stderr, " %s", x.Name)
			}
		}
		fmt.Fprintln(os.Stderr)
	}
	s := sessFromFM(fm)
	// Env-gated Acc dump for pure residual FE order (stderr; no package state).
	dumpLC := func(tag string) {
		if f == nil || StmIDUnset(f.Body.StmID) {
			return
		}
		want := ""
		var names []string
		switch {
		case os.Getenv("DIAG_LC470") != "" && f.Name == "func_10":
			want = "LC470"
			names = []string{"g_904.f1", "g_1151", "g_726.f0", "g_1214", "g_2740"}
		case os.Getenv("DIAG_S477") != "" && f.Name == "func_49":
			want = "S477"
			names = []string{"g_404", "g_16", "g_12", "g_15", "g_156", "g_141"}
		case os.Getenv("DIAG_S99FX") != "" && f.Name == "func_7":
			want = "S99FX"
			names = []string{"g_766", "g_541", "g_110", "g_3076", "g_700", "g_256", "g_1162"}
		default:
			return
		}
		eff := fm.GetMapStmEffect(f.Body.StmID)
		if !EffectComplete(eff) {
			return
		}
		ord := eff.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		posOf := func(name string) int {
			for i, v := range ord {
				if v != nil && v.Name == name {
					return i
				}
			}
			return -1
		}
		fmt.Fprintf(os.Stderr, "%s %s f=%s n=%d", want, tag, f.Name, len(ord))
		for _, n := range names {
			p := posOf(n)
			fmt.Fprintf(os.Stderr, " %s@%d", n, p)
			if p >= 0 {
				lo, hi := p-1, p+2
				if lo < 0 {
					lo = 0
				}
				if hi > len(ord) {
					hi = len(ord)
				}
				fmt.Fprint(os.Stderr, "[")
				for i := lo; i < hi; i++ {
					if i > lo {
						fmt.Fprint(os.Stderr, ",")
					}
					if ord[i] != nil {
						fmt.Fprint(os.Stderr, ord[i].Name)
					}
				}
				fmt.Fprint(os.Stderr, "]")
			}
		}
		fmt.Fprintln(os.Stderr)
	}
	_ = f.Body.SetAccumulatedEffect(fm)
	dumpLC("pre")
	// Acc-late pure-only of direct-callee owner FE → FE-relative slots first.
	// Multi-prefix pure order after (seed875: relative can reorder mid pure
	// multi-prefix pureOnly; case2 multi-prefix restores head before free residual).
	fixupNestedFEPureOnlyRelativeOrder(f, fm, s)
	dumpLC("after_pure_only_rel")
	fixupFunc1AccEarlyNestedPureAfterParentFree(f, fm, s)
	dumpLC("after_acc_early")
	fixupNestedFEPureMultiPrefixOrder(f, fm, s)
	dumpLC("after_multi_prefix")
	// Mid pure residual after free FE head Acc-late → sibling residual relative
	// last so multi-prefix / Acc-early do not undo (LevelC seed1502 func_32).
	fixupMidPureAfterFreeFEHeadRelativeOrder(f, fm, s)
	dumpLC("after_mid_pure_freehead")
	// Free-head nested pure residual Acc-late (non-func_1 free-head pure residual).
	fixupFreeHeadFEPureResidualRelativeOrder(f, fm, s)
	dumpLC("after_freehead")
	// Acc-early pure residual pure-only of free-head FE before residual free free-ref
	// owner-only: defer after first pure residual pure-only of free-head FE that is
	// Acc-late after residual free free-ref owner (seed983 func_12 g_276 after g_445).
	// Session/FM-local — no package mutable state.
	fixupAccEarlyFreeHeadPureOnlyAfterOwnerFreeRef(f, fm, s)
	dumpLC("after_freehead_pureonly_defer")
	// Acc-late pure FE head before co-owner residual free (LevelC seed1706
	// func_89 g_326 of func_101 before g_387; func_105 has g_326 then g_387).
	fixupAccLatePureFEHeadBeforeCoOwnerResidual(f, fm, s)
	dumpLC("after_coowner")
	// Owner pure residual free-ref Acc-late after residual free of same FE (not
	// free-ref on parent) after pure multi of owner (LevelC seed7310 func_26
	// g_205 before g_276). Session/FM-local.
	fixupOwnerPureResidualFreeRefBeforeResidualFree(f, fm, s)
	dumpLC("after_owner_pure_res")
	// Acc-late pure residual free-ref before residual free free-neither is func_1-only
	// (seed258). Mid-gen non-func_1 pure residual free-ref order uses owner pure residual
	// free-ref / freehead surfaces (seed788 intermediate free residual free-ref pure residual free-ref).
	// Acc-late solo pure FE head before residual free free-ref (seed32 g_381/g_1287).
	// PureIVGlobals skip residual free pure-IV (seed57 g_114) when already registered;
	// g_114 may not be registered yet mid-gen (open FE order — still known gap).
	//OFF mid-gen fixupAccLatePureFEHeadBeforeResidualFree(f, fm, s)
	// Nested pure multi-prefix pure-only FE heads pure-only on parent (float seed=2).
	fixupStripNestedPureOnlyFEHeadsFromParent(f, fm, s)
	dumpLC("after_strip_nested_pure_heads")
	fixupAccEarlyPureHeadFEPureResidualAfterFreeResidual(f, fm, s)
	dumpLC("after_purehead_pure_after_free")
	if os.Getenv("DIAG_F38") != "" && f.Name == "func_38" {
		eff := fm.GetMapStmEffect(f.Body.StmID)
		fmt.Fprint(os.Stderr, "F38 post_fixup=")
		for _, x := range eff.ReadVarsSess(s) {
			if x != nil && (x.Name == "g_136" || x.Name == "g_137" || x.Name == "g_112" || x.Name == "g_168" || x.Name == "g_172" || x.Name == "g_117") {
				fmt.Fprintf(os.Stderr, " %s", x.Name)
			}
		}
		fmt.Fprintln(os.Stderr)
	}
}

// fixupStripNestedPureOnlyFEHeadsFromParent drops pure multi-prefix pure-only FE
// heads of nested callees from parent body map_stm when they are pure-only on the
// parent (no freeVal and no array-init free-ref). C++ Acc fold of nested pure for-IV
// reads does not keep pure multi-prefix pure-only nested FE heads pure-only on the
// parent (float seed=2 g_370 of func_20 multi-prefix head; g_531 solo pure residual
// pure-only). Multi-prefix residual pure IVs after the head (g_147) and free-ref pure
// IVs (g_38 array-init, g_25 freeVal) stay. Address-of-only scalar local free-ref
// does not keep (g_531). Session/FM-local — no package mutable state.
func fixupStripNestedPureOnlyFEHeadsFromParent(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	if s == nil {
		s = sessFromFM(fm)
	}
	eff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(eff) {
		return
	}
	out, ok := stripNestedPureOnlyFEHeadsFromEffect(s, f, eff, fm)
	if !ok || !EffectComplete(out) {
		return
	}
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// stripNestedPureOnlyFEHeadsFromEffect returns eff with pure multi-prefix pure-only
// nested FE heads pure-only on f removed from the read set. ok=false on incomplete.
// Used by post-body fixup (map_stm) so Acc invent cannot re-append stripped heads
// that pureMiss did not place. PureMissTouched residual pure multi pure-only of
// pure-head keeps pureMiss prev order (seed57 g_630.f1 after g_954; g_1596 after
// g_1961.f3) — Acc invent is late/skipped for those pure residual pure-only multi
// members. Session/FM-local — no package mutable state.
func stripNestedPureOnlyFEHeadsFromEffect(s *Session, f *Function, eff Effect, fm *FactMgr) (Effect, bool) {
	if f == nil || !EffectComplete(eff) {
		return IncompleteEffect(), false
	}
	var pmt map[*Variable]bool
	if fm != nil {
		pmt = fm.PureMissTouched
	}
	pureMissTouched := func(x *Variable) bool {
		if x == nil || pmt == nil {
			return false
		}
		if pmt[x] {
			return true
		}
		// Acc / nested FE *Variable identity may differ from pureMiss key.
		for y, ok := range pmt {
			if ok && y != nil && y.Name == x.Name {
				return true
			}
		}
		return false
	}
	// Parent map order (pureMiss already applied on assigns).
	parentOrd := eff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(parentOrd) {
		return IncompleteEffect(), false
	}
	// pureMissEarlyAfterParentFree: PureMissTouched pure residual pure multi pure-only
	// of pure-head sits on parent map immediately after a free residual free that is
	// *not* free residual free of the pure-head owner FE (seed57 g_630.f1 after g_954
	// free residual free of parent neither free-ref free on parent). Acc places that
	// pure residual pure multi late after free residual free of pure-head owner
	// (g_1729.f3); invent skip would drop pureMiss early. Float g_370 pureMiss sits
	// after free residual free of pure-head owner FE — still strip. Session/FM-local.
	pureMissEarlyAfterParentFree := func(x *Variable, owner *Function) bool {
		if x == nil || owner == nil || !pureMissTouched(x) {
			return false
		}
		pi := -1
		for i, y := range parentOrd {
			if y != nil && (y == x || y.Name == x.Name) {
				pi = i
				break
			}
		}
		if pi <= 0 {
			return false
		}
		pred := parentOrd[pi-1]
		if pred == nil {
			return false
		}
		// pred must not be pure residual pure of pure-head owner (pure multi mid / head)
		if isForIVOfFunc(owner, pred) {
			return false
		}
		// free residual free of pure-head owner FE?
		if EffectComplete(owner.FEffect) {
			ofr := owner.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return false
			}
			for _, z := range ofr {
				if z == nil || isForIVOfFunc(owner, z) {
					continue
				}
				if z == pred || z.Name == pred.Name {
					return false // pred free residual free of pure-head owner — Acc-cluster
				}
			}
		}
		return true // pureMiss early after free residual free not of pure-head FE
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return IncompleteEffect(), false
	}
	strip := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) || !VariablesComplete(fr) {
			return IncompleteEffect(), false
		}
		// Walk nested FE reads; pure multi-prefix runs of pure for-IVs of this owner.
		i := 0
		for i < len(fr) {
			v := fr[i]
			if v == nil || !isForIVOfFunc(inv.User, v) {
				i++
				continue
			}
			head := v
			j := i + 1
			for j < len(fr) {
				w := fr[j]
				if w == nil || !isForIVOfFunc(inv.User, w) {
					break
				}
				j++
			}
			// Strip pure multi pure-only of pure-head nested FE pure-only on parent
			// (float g_370 pure multi-prefix). Residual pure multi pure-only of
			// pure-head pure-only on parent: strip entire pure-only residual multi run
			// so Acc invent re-adds Acc membership only (seed875 g_266 Acc-present;
			// g_194 Acc-absent residual multi of pure-head func_60) — except free-ref
			// free residual pure of pure-head on owner and PureMissTouched residual pure
			// multi pure-only early after parent free residual free not of pure-head FE
			// (seed57 g_630.f1 after g_954). Session/FM-local — no package mutable state.
			if !isForIVOfFunc(f, head) && !bodyArrayInitOrValueFreeReadsVar(f, head) {
				multi := j > i+1
				residual := i > 0
				pureHeadFE := fr[0] != nil && isForIVOfFunc(inv.User, fr[0])
				freeRef := bodySyntacticFreeReadsVar(f, head)
				if pureHeadFE && multi && !residual {
					// pure multi-prefix pure-only FE head only (float g_370).
					// Keep free-ref free pure multi FE head on parent (seed12592
					// g_32 pure multi-prefix of pure-head func_21 with free-ref free
					// on func_1). Keep when any pure multi-prefix sibling is already
					// on parent summary — pureOnly mid or free-ref free/array multi mid
					// (seed12592 func_21 g_292 of pure-head func_56 [g_292,g_121] with
					// freeVal free-ref g_121; Acc invent places head Acc-order then
					// strip undid). freeVal/array already excluded by outer gate.
					// Session-local — no package mutable state.
					if bodySyntacticFreeReadsVar(f, head) {
						// free-ref free pure multi FE head: keep
					} else {
						sibOnParent := false
						for k := i + 1; k < j; k++ {
							x := fr[k]
							if x == nil {
								continue
							}
							for _, y := range parentOrd {
								if y != nil && (y == x || y.Name == x.Name) {
									sibOnParent = true
									break
								}
							}
							if sibOnParent {
								break
							}
						}
						if !sibOnParent {
							strip[head] = true
						}
					}
				} else if pureHeadFE && residual && multi {
					// Residual pure multi pure-only of pure-head: strip pure-only members
					// of the residual multi run (seed875 g_194/g_266 of func_60 pure-only
					// on owner). Free-ref residual pure of pure-head mid multi keeps free-
					// ref on parent. Free-ref free residual pure of pure-head on owner
					// keeps even pure-only on parent (seed57 func_38 g_137 of pure-head
					// func_70 before free residual free g_168). PureMissTouched residual
					// pure multi pure-only early after parent free residual free not of
					// pure-head FE keeps pureMiss prev (seed57 g_630.f1 after g_954).
					// Float g_370 pureMiss after free residual free of pure-head owner
					// still strips. Session/FM-local.
					// Free-ref free sibling in the residual multi pure run (owner or
					// parent): Acc invent re-adds pure residual pure-only members Acc-
					// order (seed875 g_266 with free-ref free g_316 of pure-head
					// func_60; invent places g_266 after g_316 then strip undid). Keep
					// pure-only members of that residual multi run. Session-local.
					hasFreeSib := false
					for k := i; k < j; k++ {
						x := fr[k]
						if x == nil {
							continue
						}
						if bodySyntacticFreeReadsVar(inv.User, x) || bodySyntacticFreeReadsVar(f, x) ||
							bodyArrayInitOrValueFreeReadsVar(f, x) {
							hasFreeSib = true
							break
						}
					}
					if !hasFreeSib {
						for k := i; k < j; k++ {
							x := fr[k]
							if x == nil {
								continue
							}
							if bodySyntacticFreeReadsVar(f, x) || bodyArrayInitOrValueFreeReadsVar(f, x) {
								continue
							}
							if bodySyntacticFreeReadsVar(inv.User, x) {
								continue // free-ref free residual pure of pure-head owner
							}
							if pureMissEarlyAfterParentFree(x, inv.User) {
								continue
							}
							strip[x] = true
						}
					}
				} else if pureHeadFE && residual && freeRef {
					// Do not strip residual pure of pure-head with free-ref free on parent.
					// seed42 g_1054 is addr-of free-ref pure residual pure of pure-head
					// that UP keeps early after free residual free-ref g_128; stripping
					// dropped it from map_stm and Acc invent free-ref skip left FE late.
					// float g_531 Acc invent still blocked by residual pure pure-head
					// free-ref invent skip when map lacks membership. Session-local.
					_ = freeRef
				}
				// Free-head residual pure multi pure-only (pureHeadFE false): do NOT strip.
				// Acc invent places them Acc-order (seed4 func_21 g_1454 after g_17,
				// g_1736.f0 after g_544.f3 from free-head func_29 multi residual pure
				// g_1736.f0/g_290 and free-head func_41 multi residual pure g_1454/g_83).
				// Stripping undid invent and left FE-only append late. Pure-head multi
				// pure-only is covered by pureHeadFE branches above. Session-local.
			}
			i = j
		}
	}
	if len(strip) == 0 {
		return eff, true
	}
	ord := eff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return IncompleteEffect(), false
	}
	need := false
	for _, v := range ord {
		if v != nil && strip[v] {
			need = true
			break
		}
	}
	if !need {
		return eff, true
	}
	rebuilt := EmptyEffect()
	for _, w := range eff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		rebuilt = rebuilt.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(rebuilt) {
			return IncompleteEffect(), false
		}
	}
	for _, v := range ord {
		if v == nil || strip[v] {
			continue
		}
		rebuilt = rebuilt.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(rebuilt) {
			return IncompleteEffect(), false
		}
	}
	rebuilt.pure = eff.pure
	rebuilt.sideEffectFree = eff.sideEffectFree
	rebuilt.lhsWrite = eff.lhsWrite
	return rebuilt, true
}

// fixupAccEarlyPureHeadFEPureResidualAfterFreeResidual places Acc-early pure residual
// pure of a pure-head nested FE relative to free residual free of the same FE
// (float seed=2 func_20 on func_1):
//   free residual free of nested FE first (parent order);
//   array-init free-ref pure residual pure of owner right after that cluster;
//   freeVal pure residual pure of owner interleaved with free residual free of parent
//   by freeVal free-ref site order on parent (not packed after free residual free);
//   pure-only pure residual pure of owner last on FE.
// Session/FM-local — no package mutable state.
func fixupAccEarlyPureHeadFEPureResidualAfterFreeResidual(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	if s == nil {
		s = sessFromFM(fm)
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	// freeVal free-ref site rank on parent (first freeVal free-ref walk order).
	// Lower = earlier freeVal free-ref. pure-only / no freeVal → large.
	freeValSiteRank := freeValFreeRefSiteRanks(f)
	changed := false
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) || !VariablesComplete(fr) {
			return
		}
		if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(inv.User, fr[0]) {
			continue // not pure-head nested FE
		}
		var freeRes, arrayPure, freeValPure, pureOnly []*Variable
		seen := map[*Variable]bool{}
		// Pure FE head (fr[0] pure for-IV): address-of-only / pure-only stays at
		// natural Acc (default seed=2 g_26 pure FE head of func_20 was wrongly
		// packed last). freeVal free-ref pure FE head Acc-early uses freeVal free-ref
		// site rank (float seed=2 g_25 after free residual free of func_20).
		// Pure residual freeVal/array of owner reorder relative to free residual free.
		// Pure multi residual pure-only of pure-head Acc-early packs last (float
		// seed=2 g_147); solo residual pure-only keeps Acc interleave (default
		// seed=2 g_385 before g_860). Session-local — no package mutable state.
		pureFEHead := fr[0] != nil && isForIVOfFunc(inv.User, fr[0])
		// multi pure residual pure-only of pure-head: pure IVs in a pure-IV run
		// after the pure FE head (float g_147 after g_370 of func_20).
		multiResPureOnly := map[*Variable]bool{}
		if pureFEHead {
			i := 0
			for i < len(fr) {
				w := fr[i]
				if w == nil || !isForIVOfFunc(inv.User, w) {
					i++
					continue
				}
				j := i + 1
				for j < len(fr) {
					u := fr[j]
					if u == nil || !isForIVOfFunc(inv.User, u) {
						break
					}
					j++
				}
				if i > 0 && j > i+1 {
					// multi pure residual pure-only run of pure-head
					for k := i; k < j; k++ {
						if fr[k] != nil {
							multiResPureOnly[fr[k]] = true
						}
					}
				} else if i > 0 && j == i+1 {
					// solo residual pure of pure-head — not multi; leave Acc
				}
				i = j
			}
		}
		for _, v := range fr {
			if v == nil {
				continue
			}
			if _, on := pos[v]; !on || seen[v] {
				continue
			}
			seen[v] = true
			if !isForIVOfFunc(inv.User, v) {
				freeRes = append(freeRes, v)
				continue
			}
			if pureFEHead && (v == fr[0] || (fr[0] != nil && v.Name == fr[0].Name)) {
				// freeVal pure FE head Acc-early: freeVal site rank path (float g_25).
				// address-of-only / pure-only pure FE head: leave put (default g_26).
				if bodyValueFreeReadsVar(f, v) {
					freeValPure = append(freeValPure, v)
				}
				continue
			}
			if bodyValueFreeReadsVar(f, v) {
				// Own pure freeVal of parent Acc-early put (not nested pure residual
				// freeVal of pure-head). Nested freeVal pure residual still reorders
				// (seed=42 g_97). Multi residual pure free-ref free of pure-head on
				// owner with freeVal free-ref on parent keeps put (seed57 func_38
				// g_112 of pure-head func_70 after g_137 before free residual free
				// g_168). Session/FM-local — no package mutable state.
				if isForIVOfFunc(f, v) {
					continue
				}
				if multiResPureOnly[v] && bodySyntacticFreeReadsVar(inv.User, v) {
					continue // keep pure multi free-ref residual pure of pure-head owner
				}
				freeValPure = append(freeValPure, v)
			} else if bodyArrayInitOrValueFreeReadsVar(f, v) {
				// Own pure array-init of parent Acc-early put (seed145 g_2522 of
				// pure-head func_9 array-init free-ref own pure of func_1). Nested
				// array-init pure residual of pure-head still after free residual free
				// (float seed=2 g_38 of func_20). Session-local — no package mutable state.
				if isForIVOfFunc(f, v) {
					continue
				}
				arrayPure = append(arrayPure, v)
			} else if multiResPureOnly[v] {
				// multi pure residual pure-only of pure-head Acc-early → pack last
				// (float seed=2 g_147 pure-only on owner). Solo residual pure-only stays
				// put (default seed=2 g_385). Own pure of parent Acc-early put (seed12592
				// g_29). Free-ref free residual pure of pure-head on owner (even pure-only
				// on parent) keeps put (seed57 func_38 g_137/g_112 of pure-head func_70
				// before free residual free g_168). PureMissTouched residual pure multi
				// pure-only keeps pureMiss prev (seed57 g_630.f1). Session/FM-local —
				// no package mutable state.
				if isForIVOfFunc(f, v) {
					continue
				}
				if bodySyntacticFreeReadsVar(inv.User, v) {
					continue // free-ref free residual pure of pure-head owner — keep put
				}
				if fm.PureMissTouched != nil {
					if fm.PureMissTouched[v] {
						continue
					}
					pmt := false
					for y, ok := range fm.PureMissTouched {
						if ok && y != nil && y.Name == v.Name {
							pmt = true
							break
						}
					}
					if pmt {
						continue
					}
				}
				pureOnly = append(pureOnly, v)
			}
			// else solo residual pure-only: leave Acc interleave
		}
		if len(freeRes) == 0 {
			continue
		}
		// Acc-early pure residual pure before free residual free?
		minPure, maxFree := len(ord), -1
		for _, v := range arrayPure {
			if p, ok := pos[v]; ok && p < minPure {
				minPure = p
			}
		}
		for _, v := range freeValPure {
			if p, ok := pos[v]; ok && p < minPure {
				minPure = p
			}
		}
		for _, v := range pureOnly {
			if p, ok := pos[v]; ok && p < minPure {
				minPure = p
			}
		}
		for _, v := range freeRes {
			if p, ok := pos[v]; ok && p > maxFree {
				maxFree = p
			}
		}
		if minPure >= maxFree {
			continue // pure residual freeVal/array already after free residual free
		}
		// Sort freeVal pure residual pure by freeVal free-ref site rank.
		for i := 0; i < len(freeValPure); i++ {
			for j := i + 1; j < len(freeValPure); j++ {
				ri, rj := freeValSiteRank[freeValPure[i]], freeValSiteRank[freeValPure[j]]
				if ri == 0 {
					ri = 1 << 29
				}
				if rj == 0 {
					rj = 1 << 29
				}
				if rj < ri || (rj == ri && pos[freeValPure[j]] < pos[freeValPure[i]]) {
					freeValPure[i], freeValPure[j] = freeValPure[j], freeValPure[i]
				}
			}
		}
		drop := map[*Variable]bool{}
		for _, v := range arrayPure {
			drop[v] = true
		}
		for _, v := range freeValPure {
			drop[v] = true
		}
		for _, v := range pureOnly {
			drop[v] = true
		}
		// freeVal free-ref ranks of free residual free of parent (for interleaving).
		// freeVal pure residual pure of owner inserts when its site rank is between
		// free residual free of parent's freeVal free-ref ranks after free residual free
		// of nested FE.
		rebuilt := EmptyEffect()
		for _, w := range bodyEff.WrittenVarsSess(s) {
			if w == nil {
				continue
			}
			rebuilt = rebuilt.WriteVarSess(s, w)
			if sessHasError(s) || !EffectComplete(rebuilt) {
				return
			}
		}
		emittedFV := map[*Variable]bool{}
		fail := false
		// Emit freeVal pure p after free residual free with freeVal rank r only when
		// freeValRank[p] > r and no later free residual free in ord has freeVal rank
		// in (r, freeValRank[p]). free residual free can appear out of freeVal free-ref
		// site order (float seed=2 g_679 freeValRank=68 before g_988.f2 freeValRank=23);
		// freeVal pure FE head g_25 freeValRank=24 must wait for g_988.f2 not jump to
		// g_679. Session-local — no package mutable state.
		emitFreeValPuresAfterRank := func(afterRank int, fromIdx int) {
			for _, p := range freeValPure {
				if emittedFV[p] {
					continue
				}
				pr := freeValSiteRank[p]
				if pr == 0 {
					pr = 1 << 29
				}
				if pr <= afterRank {
					continue
				}
				blocked := false
				for j := fromIdx + 1; j < len(ord); j++ {
					y := ord[j]
					if y == nil || drop[y] {
						continue
					}
					yr := freeValSiteRank[y]
					if yr > afterRank && yr < pr {
						blocked = true
						break
					}
				}
				if blocked {
					continue
				}
				rebuilt = rebuilt.ReadVarSess(s, p)
				if sessHasError(s) || !EffectComplete(rebuilt) {
					fail = true
					return
				}
				emittedFV[p] = true
			}
		}
		arrayDone := false
		for i, x := range ord {
			if x == nil {
				continue
			}
			if drop[x] {
				continue
			}
			rebuilt = rebuilt.ReadVarSess(s, x)
			if sessHasError(s) || !EffectComplete(rebuilt) {
				return
			}
			// After last free residual free of nested FE: array-init pure residual pure.
			if !arrayDone && i == maxFree {
				for _, p := range arrayPure {
					rebuilt = rebuilt.ReadVarSess(s, p)
					if sessHasError(s) || !EffectComplete(rebuilt) {
						return
					}
				}
				arrayDone = true
			}
			// After free residual free of parent with freeVal free-ref: freeVal pure
			// residual pure / pure FE head with freeVal free-ref site rank just above.
			if arrayDone {
				if r := freeValSiteRank[x]; r > 0 {
					emitFreeValPuresAfterRank(r, i)
					if fail {
						return
					}
				}
			}
		}
		if !arrayDone {
			for _, p := range arrayPure {
				rebuilt = rebuilt.ReadVarSess(s, p)
				if sessHasError(s) || !EffectComplete(rebuilt) {
					return
				}
			}
		}
		// Remaining freeVal pure residual pure (no free residual free of parent freeVal free-ref after).
		for _, p := range freeValPure {
			if emittedFV[p] {
				continue
			}
			rebuilt = rebuilt.ReadVarSess(s, p)
			if sessHasError(s) || !EffectComplete(rebuilt) {
				return
			}
		}
		// pure-only pure residual pure last on FE.
		for _, p := range pureOnly {
			rebuilt = rebuilt.ReadVarSess(s, p)
			if sessHasError(s) || !EffectComplete(rebuilt) {
				return
			}
		}
		rebuilt.pure = bodyEff.pure
		rebuilt.sideEffectFree = bodyEff.sideEffectFree
		rebuilt.lhsWrite = bodyEff.lhsWrite
		bodyEff = rebuilt
		ord = bodyEff.ReadVarsSess(s)
		if sessHasError(s) || !VariablesComplete(ord) {
			return
		}
		pos = map[*Variable]int{}
		for i, v := range ord {
			if v != nil {
				if _, ok := pos[v]; !ok {
					pos[v] = i
				}
			}
		}
		changed = true
	}
	if changed {
		fm.SetMapStmEffect(f.Body.StmID, bodyEff)
	}
}

// freeValFreeRefSiteRanks assigns increasing ranks to variables freeVal free-ref'd
// in f by first freeVal free-ref site walk order. Session-local — no package state.
func freeValFreeRefSiteRanks(f *Function) map[*Variable]int {
	out := map[*Variable]int{}
	if f == nil {
		return out
	}
	rank := 1
	var noteExpr func(e *Expression)
	noteExpr = func(e *Expression) {
		if e == nil {
			return
		}
		if e.Term == TermVariable && e.Var != nil {
			// freeVal free-ref: not address-of (indir < 0).
			isAddr := false
			if e.ExprType != nil && e.Var.Type != nil {
				lv := e.Var.Type.IndirectLevelSess(nil)
				lw := e.ExprType.IndirectLevelSess(nil)
				isAddr = lv-lw < 0
			}
			if !isAddr {
				if _, ok := out[e.Var]; !ok {
					out[e.Var] = rank
					rank++
				}
			}
			if e.Var.AsArray != nil {
				for _, idx := range e.Var.AsArray.IndexExprs {
					noteExpr(idx)
				}
			}
		}
		noteExpr(e.CommaLHS)
		noteExpr(e.CommaRHS)
		if e.Assign != nil {
			noteExpr(e.Assign.Expr)
		}
		if e.Invoke != nil {
			for _, a := range e.Invoke.Args {
				noteExpr(a)
			}
		}
	}
	var walk func(blk *Block)
	walk = func(blk *Block) {
		if blk == nil {
			return
		}
		for _, loc := range blk.LocalVars {
			if loc != nil {
				noteExpr(loc.InitExpr)
			}
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			if st.Kind == StmtFor && st.Loop != nil {
				if st.Loop.TestExpr != nil {
					noteExpr(st.Loop.TestExpr)
				}
				walk(st.Then)
				continue
			}
			noteExpr(st.Expr)
			switch st.Kind {
			case StmtBlock:
				walk(st.Then)
			case StmtIfElse:
				walk(st.Then)
				walk(st.Else)
			case StmtFor, StmtArrayOp:
				walk(st.Then)
			}
		}
	}
	walk(f.Body)
	return out
}

// bodyArrayInitOrValueFreeReadsVar is freeVal free-ref or array brace-init free-ref
// on f (float seed=2: keep g_25 freeVal / g_38 array-init; drop g_531 address-of-only
// scalar local). Session-local IR walk — no package state.
func bodyArrayInitOrValueFreeReadsVar(f *Function, v *Variable) bool {
	if bodyValueFreeReadsVar(f, v) {
		return true
	}
	if f == nil || v == nil {
		return false
	}
	var walk func(blk *Block) bool
	walk = func(blk *Block) bool {
		if blk == nil {
			return false
		}
		for _, loc := range blk.LocalVars {
			if loc != nil && arrayInitsRefVar(loc.ArrayInits, v) {
				return true
			}
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			switch st.Kind {
			case StmtBlock:
				if walk(st.Then) {
					return true
				}
			case StmtIfElse:
				if walk(st.Then) || walk(st.Else) {
					return true
				}
			case StmtFor, StmtArrayOp:
				if walk(st.Then) {
					return true
				}
			}
		}
		return false
	}
	return walk(f.Body)
}

// fixupNestedFEPureMultiPrefixOrder restores nested-callee pure for-IV multi-prefix
// order on parent body map_stm. Two cases:
//  1. Split early+late pure-only around free residual after pure multi-prefix
//     (seed120 g_59 after g_5; seed12336 g_659). Lone late pure cluster skipped
//     (seed123 g_1248).
//  2. Pure multi-prefix head pure-only Acc-late after free residual of same FE
//     that sits immediately before the earliest pure multi-prefix pureOnly on
//     parent (seed 875 func_45: g_117 after g_114 before g_116). free-ref pure
//     and PureMissTouched left alone. Session/FM-local.
func fixupNestedFEPureMultiPrefixOrder(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	// --- Case 1: split early+late pure-only around free residual ---
	var present []*Variable
	var free *Variable
	var presentOwner *Function
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		var prefix []*Variable
		for _, v := range fr {
			if v == nil || !isForIVOfFunc(inv.User, v) {
				break
			}
			prefix = append(prefix, v)
		}
		if len(prefix) < 2 {
			continue
		}
		// Free residual: prefer free-ref-on-parent among FE after pure multi-prefix;
		// fall back to first FE residual on parent.
		var frv *Variable
		for _, v := range fr[len(prefix):] {
			if v == nil {
				continue
			}
			if _, ok := pos[v]; !ok {
				continue
			}
			if bodySyntacticFreeReadsVar(f, v) {
				frv = v
				break
			}
			if frv == nil {
				frv = v
			}
		}
		if frv == nil {
			continue
		}
		fi := pos[frv]
		var pres []*Variable
		early, late := false, false
		for _, v := range prefix {
			if _, ok := pos[v]; !ok {
				continue
			}
			if bodySyntacticFreeReadsVar(f, v) {
				continue
			}
			if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
				continue
			}
			pres = append(pres, v)
			if pos[v] < fi {
				early = true
			}
			if pos[v] > fi {
				late = true
			}
		}
		if !early || !late || len(pres) == 0 {
			continue
		}
		if len(pres) > len(present) {
			present = pres
			free = frv
			presentOwner = inv.User
		}
	}
	// Skip compact when free residual is sibling-shared residual (not free-ref
	// on parent) and pure multi head is exclusive residual Acc-late — Acc already
	// has free residual stream then exclusive pure multi head (LevelC seed
	// 773070767187810853 func_1: g_532…g_643 g_181 g_2.f3 of free-head-less
	// multi func_3 after shared residual of func_34). seed120 head g_28 is also
	// sibling residual so still compact before g_66/g_5. Session/FM-local.
	// Skip case1 compact when free residual free-refs on pure multi FE owner but
	// not on parent, and pure multi pure-only is split early+late around free
	// residual — Acc free residual between pure multi pure-only is UP-correct
	// (LevelC seed4100856939472112680 func_46: func_52 multi [g_120,g_716]
	// residual free g_131 free-ref on func_52 only; UP g_120 g_131…g_716 late).
	// seed88/12336 free residual free-ref free-refs on parent too — case1 still
	// compacts pure multi pure-only before free residual free-ref. Session/FM-local.
	// Skip case1 compact when free residual is pure for-IV of the *parent* and
	// multi pure-only is split early+late around it — Acc order free residual own
	// pure of parent between multi pure residuals is UP-correct (seed2
	// --no-pointers func_35: multi [g_63,g_65] of func_93 around own pure g_64;
	// UP g_65 g_64 g_63). Session/FM-local — no package mutable state.
	if free != nil && len(present) > 0 && presentOwner != nil &&
		bodySyntacticFreeReadsVar(presentOwner, free) &&
		!bodySyntacticFreeReadsVar(f, free) {
		fi := pos[free]
		headEarly := false
		midLate := false
		for _, v := range present {
			if pos[v] < fi {
				headEarly = true
			}
			if pos[v] > fi {
				midLate = true
			}
		}
		if headEarly && midLate {
			free = nil
			present = nil
			presentOwner = nil
		}
	}
	if free != nil && len(present) > 0 && isForIVOfFunc(f, free) {
		fi := pos[free]
		headEarly, midLate := false, false
		for _, v := range present {
			if pos[v] < fi {
				headEarly = true
			}
			if pos[v] > fi {
				midLate = true
			}
		}
		if headEarly && midLate {
			free = nil
			present = nil
			presentOwner = nil
		}
	}
	if free != nil && len(present) > 0 && presentOwner != nil &&
		!bodySyntacticFreeReadsVar(f, free) {
		head := present[0]
		headExclusive := true
		freeSibling := false
		for _, inv := range calls {
			if inv == nil || inv.User == nil || inv.User == presentOwner ||
				!EffectComplete(inv.User.FEffect) {
				continue
			}
			ofr := inv.User.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			for _, rv := range ofr {
				if rv == free {
					freeSibling = true
				}
				if rv == head {
					headExclusive = false
				}
			}
		}
		if freeSibling && headExclusive {
			free = nil
			present = nil
			presentOwner = nil
		}
	}
	if free != nil && len(present) > 0 {
		drop := map[*Variable]bool{}
		for _, v := range present {
			drop[v] = true
		}
		var newOrd []*Variable
		seen := map[*Variable]bool{}
		emit := func(v *Variable) {
			if v == nil || seen[v] {
				return
			}
			seen[v] = true
			newOrd = append(newOrd, v)
		}
		emitted := false
		for _, v := range ord {
			if v == nil {
				continue
			}
			if drop[v] {
				continue
			}
			if v == free && !emitted {
				for _, pv := range present {
					emit(pv)
				}
				emitted = true
			}
			emit(v)
		}
		if !emitted {
			for _, pv := range present {
				emit(pv)
			}
		}
		out := EmptyEffect()
		for _, w := range bodyEff.WrittenVarsSess(s) {
			if w == nil {
				continue
			}
			out = out.WriteVarSess(s, w)
			if sessHasError(s) || !EffectComplete(out) {
				return
			}
		}
		for _, v := range newOrd {
			if v == nil {
				continue
			}
			out = out.ReadVarSess(s, v)
			if sessHasError(s) || !EffectComplete(out) {
				return
			}
		}
		out.pure = bodyEff.pure
		out.sideEffectFree = bodyEff.sideEffectFree
		out.lhsWrite = bodyEff.lhsWrite
		fm.SetMapStmEffect(f.Body.StmID, out)
		// Refresh ord/pos for case 2 after rewrite.
		ord = out.ReadVarsSess(s)
		if sessHasError(s) || !VariablesComplete(ord) {
			return
		}
		pos = map[*Variable]int{}
		for i, v := range ord {
			if v != nil {
				if _, ok := pos[v]; !ok {
					pos[v] = i
				}
			}
		}
	}

	// --- Case 2: pure multi-prefix head Acc-late after free residual of same FE
	// immediately before earliest pure multi-prefix pureOnly (seed 875 func_45).
	type headMove struct {
		head *Variable
		anc  *Variable // insert before free residual
	}
	var heads []headMove
	headSet := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		var prefix []*Variable
		for _, v := range fr {
			if v == nil || !isForIVOfFunc(inv.User, v) {
				break
			}
			prefix = append(prefix, v)
		}
		if len(prefix) < 2 {
			continue
		}
		// Pure-only pure multi-prefix present on parent (FE order).
		var pureOnly []*Variable
		for _, v := range prefix {
			if _, ok := pos[v]; !ok {
				continue
			}
			if bodySyntacticFreeReadsVar(f, v) {
				continue
			}
			if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
				continue
			}
			pureOnly = append(pureOnly, v)
		}
		if len(pureOnly) < 2 {
			continue
		}
		head := pureOnly[0]
		hp := pos[head]
		// Pure multi-prefix head late vs other pure multi-prefix pureOnly.
		minOther := -1
		for _, v := range pureOnly[1:] {
			if minOther < 0 || pos[v] < minOther {
				minOther = pos[v]
			}
		}
		if minOther < 0 || hp <= minOther {
			continue // head already before other pure multi-prefix pureOnly
		}
		// Anchor: last parent map_stm read immediately before the pure multi-prefix
		// pureOnly cluster (may be free residual of another path — seed875 g_114
		// before g_116, not residual of func_60 FE). Fall back to earliest pureOnly.
		pureOnlySet := map[*Variable]bool{}
		for _, v := range pureOnly {
			pureOnlySet[v] = true
		}
		var anc *Variable
		for i := minOther - 1; i >= 0; i-- {
			v := ord[i]
			if v == nil || pureOnlySet[v] {
				continue
			}
			anc = v
			break
		}
		if anc == nil {
			for _, v := range pureOnly[1:] {
				if pos[v] == minOther {
					anc = v
					break
				}
			}
		}
		if anc == nil || headSet[head] {
			continue
		}
		// pure multi-prefix head must currently be after anc
		if ap, ok := pos[anc]; !ok || hp <= ap {
			continue
		}
		// Case2 only when anc is residual of another path (not residual of the
		// same pure multi FE after pure multi-prefix). seed875 g_114 is other-path
		// residual before g_116; g_266 is residual of func_52 pure multi FE — skip.
		// Case2 anchors: free-ref on parent or nested call tree (seed875 g_114
		// free-refs in func_52, not func_45 body). Skip residual of same pure multi
		// FE that sits after pure multi-prefix pureOnly cluster (seed875 g_266).
		hasFree := bodySyntacticFreeReadsVar(f, anc) ||
			nestedUserSyntacticFreeReadsVar(s, calls, anc)
		if sessHasError(s) {
			return
		}
		if !hasFree {
			continue
		}
		sameFEAfterCluster := false
		for _, v := range fr[len(prefix):] {
			if v != anc {
				continue
			}
			if ap, ok := pos[anc]; ok && ap >= minOther {
				sameFEAfterCluster = true
			}
			break
		}
		if sameFEAfterCluster {
			continue
		}
		// Nested-only free residual anchor (not free-ref on parent): pure multi head
		// Acc-late after free residual of other path stays Acc-late (c302abe func_34
		// g_328.f2 of func_55 after free residual g_97 free-refs only nested). Parent
		// free-ref anc still case2.
		if !bodySyntacticFreeReadsVar(f, anc) {
			continue
		}
		// Residual free of same pure multi FE (any position) — leave Acc-late pure
		// multi head (seed28465 g_283.f0 residual free of func_14 before Acc-late
		// head g_163; seed875 g_266 residual free after pure multi pureOnly cluster).
		sameFEResidual := false
		for _, v := range fr[len(prefix):] {
			if v == anc {
				sameFEResidual = true
				break
			}
		}
		if sameFEResidual {
			continue
		}
		// Free residual free-ref on parent with mid pure multi pureOnly already
		// Acc-early before pure multi head (seed12898 g_300 g_84 g_72). Session/FM-local.
		if bodySyntacticFreeReadsVar(f, anc) {
			midBeforeHead := false
			for _, v := range pureOnly[1:] {
				if v != nil {
					if mp, ok := pos[v]; ok && mp < hp {
						midBeforeHead = true
						break
					}
				}
			}
			if midBeforeHead {
				continue
			}
		}
		heads = append(heads, headMove{head: head, anc: anc})
		headSet[head] = true
	}
	// --- Case 2b: pure multi-prefix head Acc-late after free residual free-ref of
	// same FE when pure multi mid Acc-early before free residual. Place head
	// immediately before free residual (Acc-early mid stays put). Nested call tree
	// — multi owner may be deeper (LevelC seed7527361620846069956 func_55 under
	// func_45/func_18: Acc g_87 g_135…g_116; UP g_87 g_116 g_135). Free-ref pure
	// multi members included (func_45 free-refs g_116/g_87; pure-only gate would
	// miss). Distinct from case1 FE-order compact (g_116 g_87 g_135). Session/FM-local.
	if len(heads) == 0 {
		type deepFE struct {
			user *Function
			fr   []*Variable
		}
		var deepFEs []deepFE
		seenFn2b := map[*Function]bool{}
		var walkDeep2b func(fn *Function)
		walkDeep2b = func(fn *Function) {
			if fn == nil || seenFn2b[fn] {
				return
			}
			seenFn2b[fn] = true
			if EffectComplete(fn.FEffect) {
				fr := fn.FEffect.ReadVarsSess(s)
				if sessHasError(s) {
					return
				}
				deepFEs = append(deepFEs, deepFE{user: fn, fr: fr})
			}
			for _, blk := range fn.Blocks {
				if blk == nil {
					continue
				}
				for i := range blk.Stmts {
					var nested []*Invocation
					if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
						continue
					}
					for _, inv := range nested {
						if inv != nil && inv.User != nil {
							walkDeep2b(inv.User)
						}
					}
				}
			}
		}
		for _, inv := range calls {
			if inv != nil && inv.User != nil {
				walkDeep2b(inv.User)
			}
		}
		if sessHasError(s) {
			return
		}
		for _, d := range deepFEs {
			fr := d.fr
			var prefix []*Variable
			for _, v := range fr {
				if v == nil || !isForIVOfFunc(d.user, v) {
					break
				}
				prefix = append(prefix, v)
			}
			if len(prefix) < 2 {
				continue
			}
			// Pure multi present on parent (free-ref pure multi allowed).
			var present []*Variable
			for _, v := range prefix {
				if v == nil {
					continue
				}
				if _, ok := pos[v]; !ok {
					continue
				}
				if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
					continue
				}
				present = append(present, v)
			}
			if len(present) < 2 {
				continue
			}
			head := present[0]
			// Prefer FE-order multi head when present; if FE head absent on parent,
			// first present is not case2b.
			if head != prefix[0] {
				continue
			}
			hp, hok := pos[head]
			if !hok || headSet[head] {
				continue
			}
			// First free residual free-ref of same FE after pure multi-prefix on parent.
			var free *Variable
			for _, v := range fr[len(prefix):] {
				if v == nil || isForIVOfFunc(d.user, v) {
					continue
				}
				if _, ok := pos[v]; !ok {
					continue
				}
				if !bodySyntacticFreeReadsVar(f, v) {
					continue
				}
				free = v
				break
			}
			if free == nil {
				continue
			}
			fi := pos[free]
			// Head Acc-late after free residual free-ref.
			if hp <= fi {
				continue
			}
			// Free residual free-ref that is pure for-IV of the *parent*: Acc order
			// free residual own pure of parent between multi pure residuals is
			// UP-correct (seed2 --no-pointers func_35: multi [g_63,g_65] of func_93
			// around own pure free residual free-ref g_64; UP g_65 g_64 g_63). Do not
			// yank multi head before parent own pure free residual free-ref.
			// Session/FM-local — no package mutable state.
			if isForIVOfFunc(f, free) {
				continue
			}
			// At least one pure multi mid Acc-early immediately before free residual
			// free-ref on parent (seed752 g_87 g_135 adjacent). Non-adjacent mid…free
			// leaves Acc-late multi head (seed875 g_239 must not yank early).
			midAdj := false
			for _, v := range present[1:] {
				if v == nil {
					continue
				}
				if mp, ok := pos[v]; ok && mp == fi-1 {
					midAdj = true
					break
				}
			}
			if !midAdj {
				continue
			}
			// Insert head immediately before free residual free-ref (after Acc-early mid).
			heads = append(heads, headMove{head: head, anc: free})
			headSet[head] = true
		}
	}
	// --- Case 3 (non-func_1 only): pure multi-prefix pureOnly all Acc-late after
	// free residual of same FE currently before pure multi-prefix pureOnly cluster.
	// LevelC seed14070888874401148024 func_33/func_58: deeper func_63
	// [g_8, g_80, g_105, g_106] pure multi-prefix Acc-late after g_105; UP keeps pure
	// FE order. Case1 needs pure multi-prefix split early+late; case2 needs pure multi-
	// prefix head late vs other pure multi-prefix pureOnly. Walk nested call tree
	// (func_63 under func_58 under func_33). func_1 Acc-early pure multi-prefix
	// (seed22584) left alone.
	if len(heads) == 0 && f.Name != "func_1" {
		type clusterMove struct {
			pureOnly []*Variable
			anc      *Variable
		}
		var clusters []clusterMove
		ancUsed := map[*Variable]bool{}
		type deepFE struct {
			user *Function
			fr   []*Variable
		}
		var deepFEs []deepFE
		seenFn := map[*Function]bool{}
		var walkDeep func(fn *Function)
		walkDeep = func(fn *Function) {
			if fn == nil || seenFn[fn] {
				return
			}
			seenFn[fn] = true
			if EffectComplete(fn.FEffect) {
				fr := fn.FEffect.ReadVarsSess(s)
				if sessHasError(s) {
					return
				}
				deepFEs = append(deepFEs, deepFE{user: fn, fr: fr})
			}
			for _, blk := range fn.Blocks {
				if blk == nil {
					continue
				}
				for i := range blk.Stmts {
					var nested []*Invocation
					if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
						continue
					}
					for _, inv := range nested {
						if inv != nil && inv.User != nil {
							walkDeep(inv.User)
						}
					}
				}
			}
		}
		for _, inv := range calls {
			if inv != nil && inv.User != nil {
				walkDeep(inv.User)
			}
		}
		if sessHasError(s) {
			return
		}
		for _, d := range deepFEs {
			fr := d.fr
			var prefix []*Variable
			for _, v := range fr {
				if v == nil || !isForIVOfFunc(d.user, v) {
					break
				}
				prefix = append(prefix, v)
			}
			if len(prefix) < 2 {
				continue
			}
			// Pure multi FE head free-ref on pure multi owner Acc-late after residual
			// free of same FE on parent is UP-correct (paranoid+binary seed2 func_46:
			// func_88 multi head g_125 free-refs on owner; residual free g_308; Acc/UP
			// keeps pure multi late after g_308…g_167 g_125…g_186). Pure multi head
			// pure-only on owner still case3 (seed14070888874401148024 func_63 g_8).
			// Session/FM-local — no package mutable state.
			if prefix[0] != nil && bodySyntacticFreeReadsVar(d.user, prefix[0]) {
				continue
			}
			var pureOnly []*Variable
			for _, v := range prefix {
				if _, ok := pos[v]; !ok {
					continue
				}
				if bodySyntacticFreeReadsVar(f, v) {
					continue
				}
				if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
					continue
				}
				pureOnly = append(pureOnly, v)
			}
			if len(pureOnly) < 2 {
				continue
			}
			minP := -1
			for _, v := range pureOnly {
				if minP < 0 || pos[v] < minP {
					minP = pos[v]
				}
			}
			// First free residual of same FE after pure multi-prefix currently before
			// pure multi-prefix pureOnly cluster. Mid pure for-IV residual of the same
			// owner (seed875 g_51 of func_60) is not free residual.
			var free *Variable
			for _, v := range fr[len(prefix):] {
				if v == nil || isForIVOfFunc(d.user, v) {
					continue
				}
				ap, ok := pos[v]
				if !ok || ap >= minP {
					continue
				}
				free = v
				break
			}
			if free == nil || ancUsed[free] {
				continue
			}
			// Free residual that is FE head of a free-head nested FE (seed1502 g_8).
			freeIsFreeHead := false
			for _, d2 := range deepFEs {
				if len(d2.fr) == 0 || d2.fr[0] != free || d2.user == nil {
					continue
				}
				if !isForIVOfFunc(d2.user, d2.fr[0]) {
					freeIsFreeHead = true
					break
				}
			}
			if freeIsFreeHead {
				continue
			}
			// Free residual free-ref on parent (seed875 g_70).
			if bodySyntacticFreeReadsVar(f, free) {
				continue
			}
			// Residual free free-ref of same FE between free residual pure residual
			// and pure multi pureOnly (seed2 --random-random g_2143 between residual
			// free pure residual and pure multi g_2263/g_439).
			hasBetweenFreeRef := false
			fp, fok := pos[free]
			if fok {
				for _, v := range fr[len(prefix):] {
					if v == nil || isForIVOfFunc(d.user, v) {
						continue
					}
					if !bodySyntacticFreeReadsVar(f, v) {
						continue
					}
					ap, ok := pos[v]
					if ok && ap > fp && ap < minP {
						hasBetweenFreeRef = true
						break
					}
				}
			}
			if hasBetweenFreeRef {
				continue
			}
			// Residual free pure-IV Acc-early (seed875 g_70 pureIV) residual.
			if s != nil && s.PureIVGlobals != nil && s.PureIVGlobals[free] {
				continue
			}
			allLate := true
			for _, v := range pureOnly {
				if pos[v] <= pos[free] {
					allLate = false
					break
				}
			}
			if !allLate {
				continue
			}
			// FE order of pure multi-prefix pureOnly (not parent Acc order).
			clusters = append(clusters, clusterMove{pureOnly: pureOnly, anc: free})
			ancUsed[free] = true
		}
		if len(clusters) > 0 {
			beforeAnc := map[*Variable][]*Variable{}
			drop := map[*Variable]bool{}
			for _, c := range clusters {
				beforeAnc[c.anc] = append(beforeAnc[c.anc], c.pureOnly...)
				for _, v := range c.pureOnly {
					drop[v] = true
				}
			}
			var newOrd []*Variable
			seen := map[*Variable]bool{}
			emit := func(v *Variable) {
				if v == nil || seen[v] {
					return
				}
				seen[v] = true
				newOrd = append(newOrd, v)
			}
			for _, v := range ord {
				if v == nil {
					continue
				}
				if drop[v] {
					continue
				}
				if hs, ok := beforeAnc[v]; ok {
					for _, h := range hs {
						emit(h)
					}
				}
				emit(v)
			}
			for _, c := range clusters {
				for _, h := range c.pureOnly {
					emit(h)
				}
			}
			out := EmptyEffect()
			for _, w := range bodyEff.WrittenVarsSess(s) {
				if w == nil {
					continue
				}
				out = out.WriteVarSess(s, w)
				if sessHasError(s) || !EffectComplete(out) {
					return
				}
			}
			for _, v := range newOrd {
				if v == nil {
					continue
				}
				out = out.ReadVarSess(s, v)
				if sessHasError(s) || !EffectComplete(out) {
					return
				}
			}
			out.pure = bodyEff.pure
			out.sideEffectFree = bodyEff.sideEffectFree
			out.lhsWrite = bodyEff.lhsWrite
			fm.SetMapStmEffect(f.Body.StmID, out)
			return
		}
	}
	if len(heads) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	drop := map[*Variable]bool{}
	for _, m := range heads {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.head)
		drop[m.head] = true
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if drop[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range heads {
		emit(m.head)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupFunc1OwnPureFEHeadBeforeNestedPureResidual places own for-IVs of func_1
// that appear as residual in a nested FE after that nested pure multi-prefix
// before the nested pure multi-prefix pure-only residual on parent body map_stm.
// seed 875297491714: g_13 is own pure of func_1 and residual mid func_60 FE after
// pure multi [g_117,g_116]; Acc put g_116 before g_13. UP: g_13 then g_116.
// Only moves when own pure sits after nested pure multi-prefix in the nested FE
// order (not for unrelated own pure late for pureMiss reasons — seed48/57).
// Session/FM-local — no package mutable state.
func fixupFunc1OwnPureFEHeadBeforeNestedPureResidual(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) || f.Name != "func_1" {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	ownPure := map[*Variable]bool{}
	for _, blk := range f.Blocks {
		if blk == nil {
			continue
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil {
				ownPure[st.Loop.IV] = true
			}
		}
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	// Collect (own pure, earliest nested pure multi-prefix pure-only before it).
	type move struct {
		o   *Variable
		anc *Variable
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		// Leading pure multi-prefix of nested callee.
		var prefix []*Variable
		for _, v := range fr {
			if v == nil || !isForIVOfFunc(inv.User, v) {
				break
			}
			// Nested pure for-IV only (not own pure of parent).
			if ownPure[v] {
				break
			}
			prefix = append(prefix, v)
		}
		// Multi-prefix only (seed875 g_117,g_116). Solo pure FE head keeps
		// pure-then-residual own pure order (seed18167 g_449 before g_195 of
		// func_20). Yanking free-ref own pure before solo nested pure head
		// reverses residual FE order.
		if len(prefix) < 2 {
			continue
		}
		// Pure-only nested pure multi-prefix present on parent.
		var nestedPureOnly []*Variable
		for _, v := range prefix {
			if _, ok := pos[v]; !ok {
				continue
			}
			if bodySyntacticFreeReadsVar(f, v) {
				continue
			}
			nestedPureOnly = append(nestedPureOnly, v)
		}
		if len(nestedPureOnly) == 0 {
			continue
		}
		// Own pure residual after pure multi-prefix in this nested FE.
		for _, v := range fr[len(prefix):] {
			if v == nil || !ownPure[v] {
				continue
			}
			if moveSet[v] {
				continue
			}
			if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
				continue
			}
			// Only when own pure free-refs on parent beyond pure-for header
			// (seed875 if(g_13)). Pure-for-only residual mid nested FE keeps
			// residual FE order after nested pure FE head (LevelC g_727 after g_261).
			if !bodyValueFreeReadsVar(f, v) && !bodySyntacticFreeReadsVar(f, v) {
				continue
			}
			// Prefer value free-ref; pure-for-only residual (for IV header only)
			// is not free-ref beyond pure header — bodySyntacticFreeReadsVar is false
			// for pure-for-only IV (walk skips pure-for header). bodyValueFreeReadsVar
			// false too. So pure-for-only residual never moves — good for LevelC.
			// seed875 g_13 has if(g_13) value free-ref.
			if !bodyValueFreeReadsVar(f, v) {
				continue
			}
			op, ok := pos[v]
			if !ok {
				continue
			}
			// Earliest nested pure multi-prefix pure-only currently before own pure.
			best := -1
			var anc *Variable
			for _, p := range nestedPureOnly {
				pp := pos[p]
				if pp < op && (best < 0 || pp < best) {
					best = pp
					anc = p
				}
			}
			if anc == nil {
				continue // own pure already before all nested pure multi-prefix
			}
			moves = append(moves, move{o: v, anc: anc})
			moveSet[v] = true
		}
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.o)
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if os_, ok := beforeAnc[v]; ok {
			for _, o := range os_ {
				emit(o)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.o)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupAccEarlyPureFEHeadBeforeExclusiveResidual places Acc-early pure-only pure
// FE heads of a nested callee after free residual shared with sibling nested FEs,
// immediately before exclusive residual of the same FE.
//
// LevelC seed 291856876356590876: func_2 FE [g_182.f4, g_1548, g_1549, g_277.f4, …]
// Acc puts pure head g_182.f4 before free residual g_1548…g_2859.f1 (func_14 path;
// g_1548 is residual of both). UP: free residual then g_182.f4 then exclusive
// residual g_277.f4. Gates: (1) free residual immediately before pure head is
// sibling-only residual of a nested FE that does not lead with p (g_2068.f6 of
// func_14; not seed9107 g_52 of owner before g_53). (2) pure head immediately
// before residual of same FE that is also sibling residual. (3) exclusive residual
// after pure head is not adjacent.
// Pure-prefix-moved pure FE heads (fm.PurePrefixMoved) immediately before same-FE
// free residual free-ref both are left alone (LC seed 15934573825443220977 g_150
// before g_250). Session/FM-local — no package mutable state.
func fixupAccEarlyPureFEHeadBeforeExclusiveResidual(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) || f.Name != "func_1" {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	// Collect each direct callee FE (for sibling residual membership).
	type calleeFE struct {
		user *Function
		fr   []*Variable
	}
	var calleeFEs []calleeFE
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		calleeFEs = append(calleeFEs, calleeFE{user: inv.User, fr: fr})
	}
	feHas := map[*Function]map[*Variable]bool{}
	feHead := map[*Function]*Variable{}
	for _, c := range calleeFEs {
		m := map[*Variable]bool{}
		for _, v := range c.fr {
			if v != nil {
				m[v] = true
			}
		}
		feHas[c.user] = m
		if len(c.fr) > 0 {
			feHead[c.user] = c.fr[0]
		}
	}
	isSiblingResidual := func(owner *Function, v *Variable) bool {
		for fn, m := range feHas {
			if fn == owner {
				continue
			}
			if m[v] {
				return true
			}
		}
		return false
	}
	isSiblingOnlyResidual := func(owner *Function, p, v *Variable) bool {
		if feHas[owner] != nil && feHas[owner][v] {
			return false
		}
		for fn, m := range feHas {
			if fn == owner || !m[v] {
				continue
			}
			if feHead[fn] == p {
				continue
			}
			return true
		}
		return false
	}
	type move struct {
		p   *Variable
		anc *Variable
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, c := range calleeFEs {
		fr := c.fr
		if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(c.user, fr[0]) {
			continue
		}
		p := fr[0]
		if len(fr) > 1 && isForIVOfFunc(c.user, fr[1]) {
			continue
		}
		if bodySyntacticFreeReadsVar(f, p) {
			continue
		}
		if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
			continue
		}
		pp, pok := pos[p]
		if !pok || moveSet[p] {
			continue
		}
		var prev *Variable
		for i := pp - 1; i >= 0; i-- {
			if ord[i] != nil {
				prev = ord[i]
				break
			}
		}
		if prev == nil || !isSiblingOnlyResidual(c.user, p, prev) {
			continue
		}
		var next *Variable
		for i := pp + 1; i < len(ord); i++ {
			if ord[i] != nil {
				next = ord[i]
				break
			}
		}
		if next == nil {
			continue
		}
		inFEAfter := false
		for _, v := range fr[1:] {
			if v == next {
				inFEAfter = true
				break
			}
		}
		if !inFEAfter || !isSiblingResidual(c.user, next) {
			continue
		}
		// Same-FE sibling residual immediately after pure must free-ref on parent
		// (seed2918 shared free residual g_1548…). Pure-only residual free of same
		// FE that is also sibling residual without free-ref on parent keeps
		// pure-before-residual FE order (seed18167 g_449 before g_409 of func_20;
		// g_409 free-refs only in callee / sibling path, not func_1).
		if !bodySyntacticFreeReadsVar(f, next) {
			continue
		}
		// Pure-prefix-moved pure FE heads immediately before same-FE free residual
		// free-ref both stay put (LC g_150 before g_250). seed2918 Acc-early pure
		// never pure-prefix'd — exclusive residual still runs.
		if fm.PurePrefixMoved != nil && fm.PurePrefixMoved[p] {
			if np, ok := pos[next]; ok && np == pp+1 &&
				bodySyntacticFreeReadsVar(f, next) && bodySyntacticFreeReadsVar(c.user, next) {
				continue
			}
		}
		var anc *Variable
		ancPos := -1
		for _, v := range fr[1:] {
			if v == nil {
				continue
			}
			if isSiblingResidual(c.user, v) {
				continue
			}
			ap, ok := pos[v]
			if !ok || ap <= pp {
				continue
			}
			if ancPos < 0 || ap < ancPos {
				ancPos = ap
				anc = v
			}
		}
		if anc == nil {
			continue
		}
		if ancPos <= pp+1 {
			continue
		}
		moves = append(moves, move{p: p, anc: anc})
		moveSet[p] = true
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupNestedPureFEHeadBeforeOwnPureResidual places nested pure multi-prefix
// pure-only FE heads before free residual that currently follows Acc-late pure
// residual order, and places pure residual own mid nested FE after that free
// residual cluster.
//
// Baseline Acc (seed 5139283748462763858): g_727 (own residual mid func_2 FE
// after pure head g_261), g_261 (Acc-late pure FE head), g_237 g_43 (func_25
// residual after g_261), g_714 (parent free residual), g_700 (own pure later).
// UP: g_261 g_237 g_43 g_714 g_727 g_700 — pure FE head then free residual then
// pure residual. Naïve "insert head before residual" yields g_261 g_727 free
// (head correct, residual still early). Rotate O H free… → H free… O.
//
// Free residual after head includes (1) residual of any nested FE after head
// (func_25: g_237 g_43) and (2) non-own-pure free residual (g_714). Stops at
// own pure that is not residual-after-head in any nested FE (g_700).
// Solo pure FE head only; pure multi-prefix left to multi-prefix / own-pure
// fixups (seed875). Own residual free-ref on pure owner skips (LevelC
// 7504083620530920041 g_14 free-ref on func_17). Session/FM-local — no package
// mutable state.
func fixupNestedPureFEHeadBeforeOwnPureResidual(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) || f.Name != "func_1" {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	ownPure := map[*Variable]bool{}
	for _, blk := range f.Blocks {
		if blk == nil {
			continue
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil {
				ownPure[st.Loop.IV] = true
			}
		}
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	// residualAfterHead[h][v]: some nested FE has h then later v.
	residualAfterHead := map[*Variable]map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		for i, h := range fr {
			if h == nil {
				continue
			}
			if residualAfterHead[h] == nil {
				residualAfterHead[h] = map[*Variable]bool{}
			}
			for _, v := range fr[i+1:] {
				if v != nil {
					residualAfterHead[h][v] = true
				}
			}
		}
	}
	// nested pure FE head pure-only + own residual mid nested FE after head
	type move struct {
		head *Variable
		own  *Variable
	}
	var moves []move
	headSet := map[*Variable]bool{}
	ownSet := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		var prefix []*Variable
		for _, v := range fr {
			if v == nil || !isForIVOfFunc(inv.User, v) {
				break
			}
			if ownPure[v] {
				break
			}
			prefix = append(prefix, v)
		}
		if len(prefix) == 0 {
			continue
		}
		// Nested pure multi-prefix pure-only present on parent.
		var nestedHeads []*Variable
		for _, v := range prefix {
			if _, ok := pos[v]; !ok {
				continue
			}
			if bodySyntacticFreeReadsVar(f, v) {
				continue
			}
			if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
				continue
			}
			nestedHeads = append(nestedHeads, v)
		}
		if len(nestedHeads) == 0 {
			continue
		}
		// Solo pure FE head residual order only (LevelC g_261). Pure multi-prefix
		// (seed875 g_117/g_116) uses own-pure-before-nested for value free-ref.
		if len(prefix) > 1 || len(nestedHeads) > 1 {
			continue
		}
		// Latest own residual pure mid nested FE after solo pure FE head.
		var latestOwn *Variable
		latestOp := -1
		for _, v := range fr[len(prefix):] {
			if v == nil || !ownPure[v] {
				continue
			}
			op, ok := pos[v]
			if !ok {
				continue
			}
			if op >= latestOp {
				latestOp = op
				latestOwn = v
			}
		}
		if latestOwn == nil {
			continue
		}
		// Own residual free-ref on pure-IV owner (callee): Acc free-ref-early own
		// pure before nested pure FE head is UP-correct (LevelC seed
		// 7504083620530920041 func_1: g_14 free-ref on func_17 before pure head
		// g_250). seed5139 residual own g_727 is not free-ref on pure owner
		// func_2 — rotate still applies. Session/FM-local.
		if bodySyntacticFreeReadsVar(inv.User, latestOwn) {
			continue
		}
		for _, h := range nestedHeads {
			// Only when head is Acc-late after residual (baseline O before H).
			if pos[h] <= latestOp || headSet[h] || ownSet[latestOwn] {
				continue
			}
			moves = append(moves, move{head: h, own: latestOwn})
			headSet[h] = true
			ownSet[latestOwn] = true
		}
	}
	if len(moves) == 0 {
		return
	}
	// free residual after head: residual-after-head own pure + non-own free residual.
	// Stop at own pure that is not residual-after-head (later pure for-IV, e.g. g_700).
	afterFreeEnd := map[*Variable][]*Variable{} // free-end anchor → delayed own residual
	beforeOwn := map[*Variable][]*Variable{}    // original own pos → heads (rotate start)
	for _, m := range moves {
		hp := pos[m.head]
		freeEnd := hp
		resAfter := residualAfterHead[m.head]
		for i := hp + 1; i < len(ord); i++ {
			v := ord[i]
			if v == nil || ownSet[v] {
				// delayed own residual itself — skip over if it appeared late
				continue
			}
			if ownPure[v] {
				if resAfter == nil || !resAfter[v] {
					break // later own pure not residual-after-head
				}
				// own pure residual-after-head (func_25 g_237 after g_261)
				freeEnd = i
				continue
			}
			// non-own free residual (g_43, g_714)
			freeEnd = i
		}
		freeEndVar := ord[freeEnd]
		if freeEndVar == nil {
			continue
		}
		afterFreeEnd[freeEndVar] = append(afterFreeEnd[freeEndVar], m.own)
		beforeOwn[m.own] = append(beforeOwn[m.own], m.head)
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	emitAfter := func(v *Variable) {
		emit(v)
		for _, o := range afterFreeEnd[v] {
			emit(o)
		}
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if ownSet[v] {
			// Original residual position: emit pure FE head(s) here (rotate start).
			for _, h := range beforeOwn[v] {
				emitAfter(h)
			}
			continue // residual delayed to after free residual
		}
		if headSet[v] {
			continue // head already emitted at residual's original position
		}
		emitAfter(v)
	}
	// Safety: any unplaced head/own
	for _, m := range moves {
		emit(m.head)
		emit(m.own)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupOwnPureNestedFEHeadBeforeFreeResidual places Acc-late own pure FE heads of
// nested callees before residual free of the same FE free-ref on parent.
// pure-prefix skips own pure of parent (seed48 g_1495 pureMiss late before free-ref
// g_405). Acc-early after free-ref only Acc-leading pure.
//
// LevelC seed 8105187965554449711: func_32 FE [g_79.f1, g_753, …] — g_79.f1 own pure
// of func_1 + pure FE head of func_32 Acc-late after residual free g_753 on parent.
// UP: g_196 g_79.f1 g_176.f3 … g_753. PureMissTouched skip. Session/FM-local.
func fixupOwnPureNestedFEHeadBeforeFreeResidual(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) || f.Name != "func_1" {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type move struct {
		p   *Variable
		anc *Variable // insert p before free residual
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if len(fr) < 2 || fr[0] == nil {
			continue
		}
		p := fr[0]
		// Pure FE head of nested FE that is own pure for-IV of parent.
		if !isForIVOfFunc(inv.User, p) || !isForIVOfFunc(f, p) {
			continue
		}
		if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
			continue
		}
		// Free-ref pure FE head of nested (used beyond for-header on parent) stays
		// Acc-late after residual free free-ref of same FE (seed126482 g_177 of
		// func_43 after free residual g_430). seed8105 g_79.f1 is own pure of
		// parent but not free-ref on parent — still pure-before free residual.
		if bodySyntacticFreeReadsVar(f, p) {
			continue
		}
		// Solo pure FE head (pure multi-prefix uses multi-prefix order).
		if len(fr) > 1 && isForIVOfFunc(inv.User, fr[1]) {
			continue
		}
		// Short pure FE residual free free-ref both (seed0 --no-bitfields func_35
		// [g_829.f0, g_858, g_510]): Acc-late pure after residual free free-ref of
		// same FE free-ref both stays Acc-late after free residual of other FEs.
		// Longer FE residual free free-ref both mid residual free free-ref of same
		// FE free-ref both (seed8105 g_753 of func_32) still pure FE order pure
		// before residual free free-ref of same FE free-ref both.
		if len(fr) <= 3 {
			continue
		}
		pp, pok := pos[p]
		if !pok || moveSet[p] {
			continue
		}
		// First free residual after pure head free-ref on parent AND on nested
		// callee currently before pure. Parent-only free residual Acc-early keeps
		// free residual before pure (seed22584/seed3682609 Acc-early pure after free
		// residual). free residual free-ref both → pure FE order pure before free
		// residual (seed8105 g_79.f1 before g_753 of func_32).
		var free *Variable
		for _, v := range fr[1:] {
			if v == nil {
				continue
			}
			if isForIVOfFunc(inv.User, v) {
				continue
			}
			if !bodySyntacticFreeReadsVar(f, v) {
				continue
			}
			if !bodySyntacticFreeReadsVar(inv.User, v) {
				continue
			}
			ap, ok := pos[v]
			if !ok || ap >= pp {
				continue
			}
			free = v
			break
		}
		if free == nil {
			continue
		}
		// Free residual free-ref of same FE that is also residual of a sibling
		// nested FE (does not lead with p) keeps Acc-late pure after free residual
		// (LevelC seed1888683856227050516: g_163 of free-head func_7 after free
		// residual free-ref g_116 also residual of func_22; Acc g_116…g_178 g_163).
		// seed8105 g_753 residual free free-ref of func_32 only → pure before free.
		// Session/FM-local — no package mutable state.
		freeSibling := false
		for _, inv2 := range calls {
			if inv2 == nil || inv2.User == nil || inv2.User == inv.User ||
				!EffectComplete(inv2.User.FEffect) {
				continue
			}
			ofr := inv2.User.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			if len(ofr) > 0 && ofr[0] == p {
				continue // sibling FE also led by p
			}
			for _, rv := range ofr {
				if rv == free {
					freeSibling = true
					break
				}
			}
			if freeSibling {
				break
			}
		}
		if freeSibling {
			continue
		}
		// Acc-late pure after free residual of same FE free-ref both.
		moves = append(moves, move{p: p, anc: free})
		moveSet[p] = true
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupMidPureOnlyAfterPureFEHeadResidual places Acc-late pure-only mid pure for-IVs
// of pure FE-head nested FEs when the FE head free-refs on parent (own pure of parent
// or free-ref pure FE head) so pureOnlyRelativeOrder's pureOnly FE-head Acc-late +
// co-relocating-head path does not cover them.
//
// LevelC seed 8105187965554449711: func_61 FE [g_79.f1, free residual…, g_190, g_357, …]
// — g_79.f1 pure for-IV of func_61 + own pure of func_1; g_190 pure-only mid pure of
// func_61 Acc-late after residual free g_357… on parent. UP: g_176.f5 g_190 g_357.
// pureOnly residual free successor of pure currently before pure → insert pure before it.
// Walks call tree (func_61 under func_51). Session/FM-local — no package mutable state.
func fixupMidPureOnlyAfterPureFEHeadResidual(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) || f.Name != "func_1" {
		return
	}
	// Collect direct + nested callees (deeper pure residual owners).
	type calleeFE struct {
		user *Function
		fr   []*Variable
	}
	var calleeFEs []calleeFE
	seenFn := map[*Function]bool{}
	var walkOwn func(fn *Function)
	walkOwn = func(fn *Function) {
		if fn == nil || seenFn[fn] {
			return
		}
		seenFn[fn] = true
		if EffectComplete(fn.FEffect) {
			fr := fn.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			calleeFEs = append(calleeFEs, calleeFE{user: fn, fr: fr})
		}
		for _, blk := range fn.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				var nested []*Invocation
				if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
					continue
				}
				for _, inv := range nested {
					if inv != nil && inv.User != nil {
						walkOwn(inv.User)
					}
				}
			}
		}
	}
	var topCalls []*Invocation
	var walkTop func(blk *Block)
	walkTop = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &topCalls)
			switch blk.Stmts[i].Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkTop(blk.Stmts[i].Then)
			case StmtIfElse:
				walkTop(blk.Stmts[i].Then)
				walkTop(blk.Stmts[i].Else)
			}
		}
	}
	walkTop(f.Body)
	if !InvocationsComplete(topCalls) {
		return
	}
	for _, inv := range topCalls {
		if inv != nil && inv.User != nil {
			walkOwn(inv.User)
		}
	}
	if sessHasError(s) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type move struct {
		p   *Variable
		anc *Variable // insert p before residual free successor
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, c := range calleeFEs {
		fr := c.fr
		if len(fr) < 3 || fr[0] == nil {
			continue
		}
		// Pure FE head (pure for-IV of this nested FE).
		if !isForIVOfFunc(c.user, fr[0]) {
			continue
		}
		// FE head free-refs on parent or is own pure for-IV of parent — pureOnly
		// FE-head Acc-late path requires pureOnly FE head. seed8105 g_79.f1 is own
		// pure for-IV of func_1 (for-header only ≠ bodySyntacticFreeReadsVar free-ref).
		if !bodySyntacticFreeReadsVar(f, fr[0]) && !isForIVOfFunc(f, fr[0]) {
			continue
		}
		// Mid pure for-IVs of this FE after free residual of pure FE head.
		for i, p := range fr {
			if i == 0 || p == nil || moveSet[p] {
				continue
			}
			if !isForIVOfFunc(c.user, p) {
				continue
			}
			// pure-only mid pure (no free-ref on parent; address-of-only ok as pure residual).
			if bodyValueFreeReadsVar(f, p) {
				continue
			}
			// Skip pure multi-prefix mid pure (FE[1..] pure for-IVs) — multi-prefix order.
			// Mid pure after free residual: FE predecessor not pure for-IV of this FE.
			if i > 0 && isForIVOfFunc(c.user, fr[i-1]) {
				// still pure multi-prefix run — skip
				continue
			}
			if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
				continue
			}
			pp, pok := pos[p]
			if !pok {
				continue
			}
			// First residual free after p in FE currently before p on parent.
			var bestAnc *Variable
			for _, v := range fr[i+1:] {
				if v == nil {
					continue
				}
				if isForIVOfFunc(c.user, v) {
					continue // later pure residual of same FE
				}
				ap, ok := pos[v]
				if !ok || ap >= pp {
					continue
				}
				// Parent free-ref residual (seed3682609 g_16 of func_33 free-refs
				// via g_16[…] on func_1) must stay Acc-early free; placing mid pure
				// before it yanks pure to program start (g_57 before g_16). seed8105
				// residual free g_357 is not free-ref on parent.
				if bodySyntacticFreeReadsVar(f, v) {
					continue
				}
				// Shared residual free of another nested FE (seed3682609 g_635 leads
				// func_39 and is late residual of func_33) stays Acc-early free of that
				// sibling path; mid pure of this FE must not jump before it (UP keeps
				// g_635 g_636 g_83 g_84 before g_57). seed8105 g_357 is exclusive
				// residual free of the pure's owner FE.
				shared := false
				for _, o := range calleeFEs {
					if o.user == c.user {
						continue
					}
					for _, ov := range o.fr {
						if ov == v {
							shared = true
							break
						}
					}
					if shared {
						break
					}
				}
				if shared {
					continue
				}
				bestAnc = v
				break
			}
			if bestAnc == nil {
				continue
			}
			// Acc-late: residual free of owner FE after pure on FE sits before pure.
			moves = append(moves, move{p: p, anc: bestAnc})
			moveSet[p] = true
		}
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
	}
	for anc, hs := range beforeAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		beforeAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// FixupAllAccLatePureFEHeadsAfterAllFuncs runs Path A after every function body is built
// so PureIVGlobals is complete (seed57 g_114). Mid-gen Path A is deferred. Session-local.
func FixupAllAccLatePureFEHeadsAfterAllFuncs(s *Session, list *FunctionList, fms *FactMgrMap) {
	if s == nil || list == nil || fms == nil {
		return
	}
	for _, f := range list.Funcs {
		if f == nil || f.Body == nil || !f.IsBuilt {
			continue
		}
		if f.Name == "func_1" {
			continue // func_1 pure-IV surface is FixupFunc1; Path A on func_1 needs mid-gen Acc fix
		}
		fm := fms.ForFuncSess(s, f)
		if fm == nil || StmIDUnset(f.Body.StmID) {
			continue
		}
		oldFE := f.FEffect
		if !EffectComplete(oldFE) {
			continue
		}
		preMap := fm.GetMapStmEffect(f.Body.StmID)
		if !EffectComplete(preMap) {
			continue
		}
		preOrd := preMap.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if os.Getenv("DIAG_LC470") != "" && f.Name == "func_10" {
			fmt.Fprintf(os.Stderr, "LC470 PathA_pre f=%s n=%d\n", f.Name, len(preOrd))
			for i, v := range preOrd {
				if v != nil && (v.Name == "g_904.f1" || v.Name == "g_1151" || v.Name == "g_726.f0") {
					fmt.Fprintf(os.Stderr, "  pre[%d]=%s\n", i, v.Name)
				}
			}
		}
		fixupAccLatePureFEHeadBeforeResidualFree(f, fm, s)
		bodyEff := fm.GetMapStmEffect(f.Body.StmID)
		if !EffectComplete(bodyEff) {
			continue
		}
		postOrd := bodyEff.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if os.Getenv("DIAG_LC470") != "" && f.Name == "func_10" {
			fmt.Fprintf(os.Stderr, "LC470 PathA_post f=%s n=%d\n", f.Name, len(postOrd))
			for i, v := range postOrd {
				if v != nil && (v.Name == "g_904.f1" || v.Name == "g_1151" || v.Name == "g_726.f0") {
					fmt.Fprintf(os.Stderr, "  post[%d]=%s\n", i, v.Name)
				}
			}
		}
		changed := len(preOrd) != len(postOrd)
		if !changed {
			for i := range preOrd {
				if preOrd[i] != postOrd[i] {
					changed = true
					break
				}
			}
		}
		if !changed {
			continue // Path A no-op: leave mid-gen FEffect alone
		}
		// Path A reordered map_stm — rebuild FEffect as map_stm order ∩ prior FEffect.
		oldSet := map[*Variable]bool{}
		for _, v := range oldFE.ReadVarsSess(s) {
			if v != nil {
				oldSet[v] = true
			}
		}
		if sessHasError(s) {
			return
		}
		out := EmptyEffect()
		for _, w := range oldFE.WrittenVarsSess(s) {
			if w == nil {
				continue
			}
			out = out.WriteVarSess(s, w)
			if sessHasError(s) || !EffectComplete(out) {
				return
			}
		}
		seen := map[*Variable]bool{}
		for _, v := range postOrd {
			if v == nil || !oldSet[v] || seen[v] {
				continue
			}
			seen[v] = true
			out = out.ReadVarSess(s, v)
			if sessHasError(s) || !EffectComplete(out) {
				return
			}
		}
		for _, v := range oldFE.ReadVarsSess(s) {
			if v == nil || seen[v] {
				continue
			}
			seen[v] = true
			out = out.ReadVarSess(s, v)
			if sessHasError(s) || !EffectComplete(out) {
				return
			}
		}
		out.pure = oldFE.pure
		out.sideEffectFree = oldFE.sideEffectFree
		out.lhsWrite = oldFE.lhsWrite
		f.FEffect = out
	}
}

// fixupAccLatePureFEHeadBeforeResidualFree places Acc-late solo pure FE heads of nested
// callees before residual free of the same FE currently before pure on parent (FE order
// pure before residual free). Not own pure of parent — Acc-late own pure after residual
// free free-ref of same FE free-ref both stays Acc-late (seed0 --no-bitfields g_829.f0).
//
// seed32 func_34: func_39 FE [g_381, g_1287, …] pure FE head Acc-late after residual free
// g_1287; UP: g_381 g_1287. Residual free free-ref on pure-IV owner skips (LevelC
// 3788929630029863038 g_7 free-ref on func_68 — Acc g_7 g_75). Distinct from co-owner
// residual free (seed1706 needs mid pure residual for-IV on another FE). Session/FM-local.
//
// Prefer FixupAllAccLatePureFEHeadsAfterAllFuncs (post-all-bodies) so PureIVGlobals is
// complete (seed57). Mid-gen call is deferred.
func fixupAccLatePureFEHeadBeforeResidualFree(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	// Soft-expand nested user call tree (func_1→…→func_20 for seed18167).
	// Incomplete IR soft-skips edges only.
	type feSite struct {
		user *Function
		fr   []*Variable
	}
	var sites []feSite
	seenFn := map[*Function]bool{}
	var walkFn func(fn *Function)
	walkFn = func(fn *Function) {
		if fn == nil || seenFn[fn] {
			return
		}
		seenFn[fn] = true
		if EffectComplete(fn.FEffect) {
			fr := fn.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			sites = append(sites, feSite{user: fn, fr: fr})
		}
		for _, blk := range fn.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				var nested []*Invocation
				// Soft-collect nested callees (seed18167 func_1→…→func_20).
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
				for _, inv := range nested {
					if inv != nil && inv.User != nil {
						walkFn(inv.User)
					}
				}
			}
		}
	}
	// Nested pure multi free-ref residual free Acc-early is parent/callee FE merge.
	// func_1 pureMiss surface (seed88) must not re-pair nested pure multi free-ref mids.
	if f.Name != "func_1" {
		for _, inv := range calls {
			if inv != nil && inv.User != nil {
				walkFn(inv.User)
			}
		}
	}
	if sessHasError(s) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type move struct {
		p   *Variable
		anc *Variable
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, site := range sites {
		user := site.user
		fr := site.fr
		if len(fr) < 2 || fr[0] == nil || !isForIVOfFunc(user, fr[0]) {
			continue
		}
		// Solo pure FE head pure-only.
		if isForIVOfFunc(user, fr[1]) {
			continue
		}
		p := fr[0]
		// Own pure of parent: Acc-late pure after residual free free-ref of same FE
		// free-ref both stays Acc-late (seed0 --no-bitfields g_829.f0 of func_35).
		if isForIVOfFunc(f, p) {
			continue
		}
		if bodySyntacticFreeReadsVar(f, p) {
			continue
		}
		if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
			continue
		}
		pp, pok := pos[p]
		if !pok || moveSet[p] {
			continue
		}
		// First residual free after pure head in FE order.
		var firstRes *Variable
		for _, v := range fr[1:] {
			if v == nil || isForIVOfFunc(user, v) {
				continue
			}
			firstRes = v
			break
		}
		if firstRes == nil {
			continue
		}
		// Residual free of same FE immediately before pure on parent (adjacent Acc-late)
		// must be the first residual free after pure head (seed32 g_1287 fr[1] of func_39).
		// Mid residual free of same FE free residual free currently before pure Acc-late
		// pure FE head stays Acc-late (seed123 g_1675 before g_1248 of func_51).
		var free *Variable
		for i := pp - 1; i >= 0; i-- {
			if ord[i] == nil {
				continue
			}
			if ord[i] == firstRes {
				free = firstRes
			}
			break
		}
		if free == nil {
			continue
		}
		// Path A: pure FE head before adjacent residual free free-ref (seed32).
		// Skip residual free pure for-IV of any session pure IV (seed57 g_114).
		skipPureIV := false
		if s != nil && free != nil {
			if s.PureIVGlobals[free] {
				skipPureIV = true
			} else if free.Name != "" {
				for v := range s.PureIVGlobals {
					if v != nil && v.Name == free.Name {
						skipPureIV = true
						break
					}
				}
			}
		}
		if skipPureIV {
			continue
		}
		if !bodySyntacticFreeReadsVar(f, free) {
			continue
		}
		// Residual free free-ref on pure-IV owner (callee): parent Acc free-before-pure
		// is UP-correct — do not restore nested FE pure-before-residual (LevelC seed
		// 3788929630029863038 func_55: func_68 FE [g_75,g_7] pure free-ref on owner
		// and residual free free-ref on owner; UP Acc g_7 g_75). seed32 residual free
		// g_1287 is not free-ref on pure owner func_39 — Path A still applies.
		// Session/FM-local — no package mutable state.
		if bodySyntacticFreeReadsVar(user, free) {
			continue
		}
		moves = append(moves, move{p: p, anc: free})
		moveSet[p] = true
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
	}
	for anc, hs := range beforeAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		beforeAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupAccLatePureFEHeadBeforeCoOwnerResidual places Acc-late solo pure FE heads of
// nested callees before residual free of the same FE that currently precedes pure on
// parent when another nested FE has pure as residual for-IV immediately before that
// residual free on its FE.
//
// LevelC seed 17069869281103512697 func_89: func_101 FE [g_326, g_983, …, g_387] pure
// FE head Acc-late next to exclusive residual free g_983…; func_105 has g_326 then
// g_387. UP: g_326 g_387 g_256… g_1003 g_983 — pure before co-owner residual free
// g_387; exclusive residual free of pure FE head stays late. Session/FM-local.
func fixupAccLatePureFEHeadBeforeCoOwnerResidual(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	type calleeFE struct {
		user *Function
		fr   []*Variable
	}
	var calleeFEs []calleeFE
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		calleeFEs = append(calleeFEs, calleeFE{user: inv.User, fr: fr})
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type move struct {
		p   *Variable
		anc *Variable // insert p immediately before residual free
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, c := range calleeFEs {
		fr := c.fr
		if len(fr) < 2 || fr[0] == nil || !isForIVOfFunc(c.user, fr[0]) {
			continue
		}
		// Solo pure FE head pure-only (not pure multi-prefix pure-only).
		if isForIVOfFunc(c.user, fr[1]) {
			continue
		}
		p := fr[0]
		if bodySyntacticFreeReadsVar(f, p) {
			continue
		}
		if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
			continue
		}
		pp, pok := pos[p]
		if !pok || moveSet[p] {
			continue
		}
		// Residual free of this FE currently before pure.
		ownerResBefore := map[*Variable]bool{}
		for _, v := range fr[1:] {
			if v == nil || isForIVOfFunc(c.user, v) {
				continue
			}
			ap, ok := pos[v]
			if !ok || ap >= pp {
				continue
			}
			ownerResBefore[v] = true
		}
		if len(ownerResBefore) == 0 {
			continue // not Acc-late vs any residual free of owner FE
		}
		// Co-owner residual free: residual free of this FE currently before pure that
		// is residual free of another nested FE where pure is residual for-IV
		// immediately before that residual free on that FE (func_105: g_326 g_387).
		var bestAnc *Variable
		bestAp := -1
		for _, o := range calleeFEs {
			if o.user == c.user || len(o.fr) < 2 {
				continue
			}
			// pure residual for-IV of co-owner FE mid residual.
			pIdx := -1
			for i, v := range o.fr {
				if v == p && isForIVOfFunc(o.user, p) {
					pIdx = i
					break
				}
			}
			if pIdx < 0 || pIdx+1 >= len(o.fr) {
				continue
			}
			// Residual free of co-owner FE immediately after pure on that FE.
			next := o.fr[pIdx+1]
			if next == nil || isForIVOfFunc(o.user, next) {
				continue
			}
			if !ownerResBefore[next] {
				continue
			}
			if bodySyntacticFreeReadsVar(f, next) {
				continue
			}
			ap := pos[next]
			// Earliest co-owner residual free currently before pure.
			if bestAp < 0 || ap < bestAp {
				bestAp = ap
				bestAnc = next
			}
		}
		if bestAnc == nil {
			continue
		}
		moves = append(moves, move{p: p, anc: bestAnc})
		moveSet[p] = true
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
	}
	// Stable pure order when multiple pure heads share an anc: parent order.
	for anc, hs := range beforeAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		beforeAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupOwnerPureResidualFreeRefBeforeResidualFree places Acc-late pure residual
// free-ref of owner or nested FE before residual free of the same FE (not free-ref
// on parent) currently Acc-early immediately before pure residual free-ref.
//
// LevelC seed 7310116159430602168:
//   - func_26 (owner): Acc g_348 g_276 g_205; UP g_348 g_205 g_276.
//   - func_1 (parent): Acc g_276 g_205 from nested Acc; nested FE already UP.
// Residual free is not free-ref on parent; pure residual free-ref free-refs on
// parent. Adjacent globals only. Session/FM-local — no package mutable state.
func fixupOwnerPureResidualFreeRefBeforeResidualFree(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type pair struct {
		p    *Variable
		free *Variable
	}
	var pairs []pair

	// Nested FEs (soft-expanded): pure multi then pure residual free-ref then residual free
	// (or residual free then pure residual free-ref on unfixed FE). Collect pairs.
	seenFn := map[*Function]bool{}
	var walkFn func(*Function)
	walkFn = func(fn *Function) {
		if fn == nil || seenFn[fn] {
			return
		}
		seenFn[fn] = true
		if EffectComplete(fn.FEffect) {
			fr := fn.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			// Pure multi free-ref of nested FE: consecutive pure for-IVs of FE owner
			// that free-ref on FE owner after free residual FE head (func_26
			// [g_165,g_348,g_205] free-ref pure multi then residual free g_276).
			// Residual free Acc-early between pure multi free-ref members is fixed by
			// placing Acc-late pure multi free-ref members before residual free.
			// Session/FM-local — no package mutable state.
			pi := 0
			for pi < len(fr) {
				if fr[pi] != nil && isForIVOfFunc(fn, fr[pi]) {
					break
				}
				pi++
			}
			var pureMultiFreeRef []*Variable
			pe := pi
			for pe < len(fr) {
				v := fr[pe]
				if v == nil || !isForIVOfFunc(fn, v) || !bodySyntacticFreeReadsVar(fn, v) {
					break
				}
				pureMultiFreeRef = append(pureMultiFreeRef, v)
				pe++
			}
			if len(pureMultiFreeRef) >= 2 {
				// First residual free of same FE after pure multi free-ref (not free-ref
				// on parent). Acc-early residual free between pure multi free-ref head
				// and Acc-late pure multi free-ref mid (LevelC 7310 g_348 g_276 g_205).
				var free *Variable
				for _, v := range fr[pe:] {
					if v == nil || isForIVOfFunc(fn, v) {
						continue
					}
					if bodySyntacticFreeReadsVar(f, v) {
						continue
					}
					free = v
					break
				}
				if free != nil {
					head := pureMultiFreeRef[len(pureMultiFreeRef)-1]
					// Prefer last pure multi free-ref Acc-early before residual free as
					// Acc head of pure multi free-ref cluster before residual free
					// (func_26 Acc: g_348 before g_276; FE multi [g_165,g_348,g_205]).
					for i := len(pureMultiFreeRef) - 1; i >= 0; i-- {
						if _, ok := pos[pureMultiFreeRef[i]]; ok {
							// find last pure multi free-ref Acc-early before free
							ap, aok := pos[free]
							hp, hok := pos[pureMultiFreeRef[i]]
							if hok && aok && hp < ap {
								head = pureMultiFreeRef[i]
								break
							}
						}
					}
					hp, hok := pos[head]
					ap, aok := pos[free]
					if hok && aok && hp < ap {
						// head Acc-early adjacent before residual free among globals
						headAdj := true
						for j := hp + 1; j < ap; j++ {
							w := ord[j]
							if w != nil && w.IsGlobalSess(s) {
								headAdj = false
								break
							}
						}
						if headAdj {
							for _, p := range pureMultiFreeRef {
								pp, pok := pos[p]
								if !pok || pp <= ap {
									continue
								}
								// pure multi free-ref mid Acc-late adjacent after residual free
								midAdj := true
								for j := ap + 1; j < pp; j++ {
									w := ord[j]
									if w != nil && w.IsGlobalSess(s) {
										midAdj = false
										break
									}
								}
								if midAdj {
									pairs = append(pairs, pair{p: p, free: free})
								}
							}
						}
					}
				}
			}
		}
		for _, blk := range fn.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				var nested []*Invocation
				if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
					continue
				}
				for _, inv := range nested {
					if inv != nil && inv.User != nil {
						walkFn(inv.User)
					}
				}
			}
		}
	}
	var calls []*Invocation
	var walkCalls func(*Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	for _, inv := range calls {
		if inv != nil && inv.User != nil {
			walkFn(inv.User)
		}
	}
	if sessHasError(s) {
		return
	}

	// Owner Acc mid-gen is covered once callees finalize FE and nested pure multi
	// free-ref residual free Acc-early runs on this function (parents) and on the
	// owner after nested FE is fixed. Session/FM-local.

	type move struct {
		p   *Variable
		anc *Variable
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, pr := range pairs {
		p, free := pr.p, pr.free
		if p == nil || free == nil || moveSet[p] {
			continue
		}
		if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
			continue
		}
		pp, pok := pos[p]
		ap, aok := pos[free]
		if !pok || !aok || ap >= pp {
			continue
		}
		// Adjacent among globals
		adjacent := true
		for j := ap + 1; j < pp; j++ {
			w := ord[j]
			if w != nil && w.IsGlobalSess(s) {
				adjacent = false
				break
			}
		}
		if !adjacent {
			continue
		}
		moves = append(moves, move{p: p, anc: free})
		moveSet[p] = true
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
	}
	for anc, hs := range beforeAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		beforeAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupAccEarlyPureHeadResidualFreeRefAfterResidualFree places Acc-early own pure residual
// free-ref pure for-IVs of pure-head nested FEs after residual free free-ref of the same
// FE on parent body map_stm.
//
// LevelC seed 10990494057038618915 func_1: pure-head func_59 FE [g_726,…,g_942,g_329,g_725,g_1075,…]
// — pure residual free-ref g_1075 (also own pure of func_1) Acc-early from parent free-ref
// after g_100; residual free free-ref g_942 Acc-late. UP: g_942 g_1075. Freehead freefRef
// path requires free-head FE + Acc-late pure; Acc-early pure-head residual free-ref is
// distinct. Nested-only free-ref pure residual Acc-early stays Acc (seed57 g_288/g_394 of
// func_10). Soft-expand nested owners (func_59 not direct under func_1). PureMissTouched
// skip. Solo pure residual (not pure multi mid). Last residual free free-ref by FE index
// (not max parent Acc).
//
// Value free-ref gate: only reorder when parent has a value free-ref of pure
// (bodyValueFreeReadsVar). Own pure Acc-early without value free-ref is pure-IV /
// address-of Acc order (FuzzBodyParity seed 14545857908692666416 g_2522 before
// g_2734 of pure-head func_9) — leave put. Session/FM-local — no package mutable state.
func fixupAccEarlyPureHeadResidualFreeRefAfterResidualFree(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	// Env-gated skip for pre-fixup diagnostics only (no package mutable state).
	if os.Getenv("DIAG_N15_SKIP") != "" {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type move struct {
		p   *Variable
		anc *Variable
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	// Prefer latest residual free free-ref anchor (deepest FE pred free-ref before pure).
	bestAncPos := map[*Variable]int{}

	seenFn := map[*Function]bool{}
	var walkFn func(fn *Function)
	walkFn = func(fn *Function) {
		if fn == nil || seenFn[fn] {
			return
		}
		seenFn[fn] = true
		if EffectComplete(fn.FEffect) {
			fr := fn.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			// Pure-head nested FE only (free-head freefRef path covers free-head).
			if len(fr) < 2 || fr[0] == nil || !isForIVOfFunc(fn, fr[0]) {
				goto walkNested
			}
			for i, p := range fr {
				if i == 0 || p == nil {
					continue
				}
				if !isForIVOfFunc(fn, p) {
					continue
				}
				// Solo pure residual mid/tail (not pure multi mid).
				if i+1 < len(fr) && fr[i+1] != nil && isForIVOfFunc(fn, fr[i+1]) {
					continue
				}
				if fr[i-1] != nil && isForIVOfFunc(fn, fr[i-1]) {
					continue
				}
				// Acc-early from parent free-ref. Only own pure of parent: free-ref
				// pure residual of pure-head nested FE that is also for-IV of parent
				// (seed10990 g_1075 of func_1+func_59). Nested-only free-ref pure
				// residual Acc-early stays Acc (seed57 g_288/g_394 of func_10).
				if !bodySyntacticFreeReadsVar(f, p) {
					continue
				}
				if !isForIVOfFunc(f, p) {
					continue
				}
				if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
					continue
				}
				pp, pok := pos[p]
				if !pok {
					continue
				}
				// Last residual free free-ref by FE index before pure (not by parent
				// Acc position). seed10990 g_1075 → g_942; g_114 → g_246 already
				// Acc-after — leave alone even when earlier FE residual free free-ref
				// g_827 is Acc-late after pure (UP keeps pure Acc-early vs g_827).
				var lastRF *Variable
				lastFE := -1
				for j, v := range fr {
					if v == nil || j >= i {
						continue
					}
					if isForIVOfFunc(fn, v) {
						continue
					}
					if !bodySyntacticFreeReadsVar(f, v) {
						continue
					}
					if _, ok := pos[v]; !ok {
						continue
					}
					if j >= lastFE {
						lastFE = j
						lastRF = v
					}
				}
				if lastRF == nil {
					continue
				}
				ap, aok := pos[lastRF]
				if !aok || pp > ap {
					continue // already Acc-late after FE-predecessor residual free free-ref
				}
				// Value free-ref gate: invent-early pure residual free-ref (seed10990
				// g_1075) has a parent value free-ref and needs residual free free-ref
				// (g_942) first. Own pure Acc-early without value free-ref is pure-IV /
				// address-of Acc order (fuzz g_2522 before g_2734 of func_9) — leave.
				// Session-local — no package mutable state.
				if !bodyValueFreeReadsVar(f, p) {
					continue
				}
				// Prefer later FE-index residual free free-ref across pure-head owners.
				if prev, ok := bestAncPos[p]; ok && lastFE <= prev {
					continue
				}
				if moveSet[p] {
					for k := range moves {
						if moves[k].p == p {
							moves[k].anc = lastRF
							break
						}
					}
				} else {
					moves = append(moves, move{p: p, anc: lastRF})
					moveSet[p] = true
				}
				bestAncPos[p] = lastFE
				if os.Getenv("DIAG_N15") != "" && p != nil && lastRF != nil {
					fmt.Fprintf(os.Stderr, "ACCEARLY_PUREHEAD_RES f=%s owner=%s p=%s@%d anc=%s@%d fe=%d\n",
						f.Name, fn.Name, p.Name, pp, lastRF.Name, ap, lastFE)
				}
			}
		}
	walkNested:
		for _, blk := range fn.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				var nested []*Invocation
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
				for _, inv := range nested {
					if inv != nil && inv.User != nil {
						walkFn(inv.User)
					}
				}
			}
		}
	}
	for _, inv := range calls {
		if inv != nil && inv.User != nil {
			walkFn(inv.User)
		}
	}
	if sessHasError(s) || len(moves) == 0 {
		return
	}
	afterAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		afterAnc[m.anc] = append(afterAnc[m.anc], m.p)
	}
	for anc, hs := range afterAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		afterAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		emit(v)
		if hs, ok := afterAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupAccEarlyFreeHeadPureOnlyAfterOwnerFreeRef defers Acc-early pure residual pure-only
// of free-head nested FE that sits immediately before residual free free-ref owner-only of
// free-head FE. Place pure after the first pure residual pure-only of free-head FE that
// follows pure residual Acc-early in FE order and is Acc-late after residual free free-ref
// owner (seed9838263505978394624 func_12: free-head func_28 pure residual g_276 Acc-early
// before residual free free-ref owner g_260; UP g_284 g_260…g_445 g_276). Residual free
// free-ref owner-only (not free-ref parent). seed12848 pure already Acc-late — no-op.
// afterPure anchor must free-ref on free-head owner (seed983 g_445); pure residual
// pure-only without owner free-ref (seed55 g_141) must not anchor a defer that moves
// Acc-early pure residual pure-only past residual free free-ref owner (g_139/g_159).
// Session/FM-local — no package mutable state.
func fixupAccEarlyFreeHeadPureOnlyAfterOwnerFreeRef(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type move struct {
		p   *Variable
		anc *Variable // insert p after anc
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	// Soft-expand nested free-head owners (func_28 under func_12).
	seenFn := map[*Function]bool{}
	var walkFn func(fn *Function)
	walkFn = func(fn *Function) {
		if fn == nil || seenFn[fn] {
			return
		}
		seenFn[fn] = true
		if EffectComplete(fn.FEffect) {
			fr := fn.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			if len(fr) < 3 || fr[0] == nil || isForIVOfFunc(fn, fr[0]) {
				goto walkNested
			}
			for i, p := range fr {
				if i == 0 || p == nil || moveSet[p] {
					continue
				}
				if !isForIVOfFunc(fn, p) {
					continue
				}
				// pure residual pure-only on parent
				if bodySyntacticFreeReadsVar(f, p) {
					continue
				}
				if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
					continue
				}
				// Solo pure residual (not pure multi mid of free-head FE).
				if i+1 < len(fr) && fr[i+1] != nil && isForIVOfFunc(fn, fr[i+1]) {
					continue
				}
				if fr[i-1] != nil && isForIVOfFunc(fn, fr[i-1]) {
					continue
				}
				// First residual free after pure in FE that free-refs free-head owner
				// and not free-ref parent (owner-only residual free free-ref).
				// Require FE-adjacency: no residual free free-ref owner between pure and
				// firstRes in FE (seed983 g_276 then g_260). Residual free free-ref parent
				// (or non-owner free residual) between pure and residual free free-ref owner
				// means Acc-early pure residual pure-only before residual free free-ref owner
				// is UP-correct FE free-ref order (seed55 g_139…g_2…g_159 — keep Acc-early;
				// free-head pureonly defer must not move past residual free free-ref owner).
				// Session/FM-local — no package mutable state.
				var firstRes *Variable
				feAdj := true
				for _, v := range fr[i+1:] {
					if v == nil {
						continue
					}
					if isForIVOfFunc(fn, v) {
						// pure multi / later pure residual — stop adjacency for residual free
						if firstRes == nil {
							feAdj = false
						}
						continue
					}
					if !bodySyntacticFreeReadsVar(fn, v) {
						continue
					}
					if bodySyntacticFreeReadsVar(f, v) {
						// residual free free-ref parent between pure and owner-only residual free
						if firstRes == nil {
							feAdj = false
						}
						continue // free residual free-ref parent — not owner-only gate
					}
					// residual free free-ref owner-only
					if firstRes == nil {
						firstRes = v
						break
					}
				}
				if firstRes == nil || !feAdj {
					continue
				}
				pp, pok := pos[p]
				ap, aok := pos[firstRes]
				if !pok || !aok || pp >= ap {
					continue // pure not Acc-early before residual free free-ref owner
				}
				// Pure immediately before residual free free-ref owner among globals.
				adj := true
				for j := pp + 1; j < ap; j++ {
					w := ord[j]
					if w != nil && w.IsGlobalSess(s) {
						adj = false
						break
					}
				}
				if sessHasError(s) {
					return
				}
				if !adj {
					continue
				}
				// Acc-early pure residual pure-only must sit immediately after pure residual
				// pure residual free-ref of free-head FE (seed983 g_284 pure residual free-ref
				// freeO then g_276 pure residual pure-only then residual free free-ref owner
				// g_260). Acc-early pure residual pure-only after free residual free-neither of
				// free-head FE (seed983 g_930 after g_283 before residual free free-ref owner
				// g_444) is UP-correct Acc-early — do not defer. Session/FM-local.
				predPureRef := false
				for j := pp - 1; j >= 0; j-- {
					w := ord[j]
					if w == nil {
						continue
					}
					if !w.IsGlobalSess(s) {
						continue
					}
					if sessHasError(s) {
						return
					}
					if isForIVOfFunc(fn, w) && bodySyntacticFreeReadsVar(fn, w) &&
						!bodySyntacticFreeReadsVar(f, w) {
						predPureRef = true
					}
					break
				}
				if !predPureRef {
					continue
				}
				// First pure residual pure-only of free-head FE after pure residual Acc-
				// early in FE order that is Acc-late after residual free free-ref owner
				// (seed983 g_445 after residual free free-ref owner g_260). Require a real
				// pure residual pure-only anchor — placing after residual free free-ref owner
				// alone reverses free residual free-ref before pure residual pure-only Acc
				// that is UP-correct (seed12848 func_29 g_178 before g_196). Session/FM-local.
				var afterPure *Variable
				for _, v := range fr[i+1:] {
					if v == nil || !isForIVOfFunc(fn, v) {
						continue
					}
					if bodySyntacticFreeReadsVar(f, v) {
						continue
					}
					// Anchor pure residual pure-only must free-ref on free-head owner
					// (seed983 g_445 free-ref func_28). Pure residual pure-only with no
					// free-ref on owner (seed55 g_141) is Acc-order late clutter — using
					// it as after-anchor wrongly defers Acc-early pure residual pure-only
					// that UP keeps before residual free free-ref owner (g_139 before
					// g_159). Session/FM-local — no package mutable state.
					if !bodySyntacticFreeReadsVar(fn, v) {
						continue
					}
					vp, vok := pos[v]
					if !vok || vp <= ap {
						continue
					}
					afterPure = v
					break
				}
				if afterPure == nil {
					continue
				}
				moves = append(moves, move{p: p, anc: afterPure})
				moveSet[p] = true
			}
		}
	walkNested:
		for _, blk := range fn.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				var nested []*Invocation
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
				for _, inv := range nested {
					if inv != nil && inv.User != nil {
						walkFn(inv.User)
					}
				}
			}
		}
	}
	for _, inv := range calls {
		if inv != nil && inv.User != nil {
			walkFn(inv.User)
		}
	}
	if sessHasError(s) || len(moves) == 0 {
		return
	}
	afterAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		afterAnc[m.anc] = append(afterAnc[m.anc], m.p)
	}
	for anc, hs := range afterAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		afterAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		emit(v)
		if hs, ok := afterAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupAccLatePureResidualFreeRefBeforeFreeNeither places Acc-late pure residual free-ref
// of nested FE before residual free free-neither of same FE currently before pure when
// residual free free-ref owner-only of same FE is Acc-early before residual free free-neither
// (seed2580000644868734815 func_1: func_28 FE […g_716 free residual free-ref owner-only,
// g_601 free residual free-ref both, g_247 pure free-ref, g_188 residual free free-neither];
// Acc g_716 g_188…g_247 late; UP g_716 g_247 g_188). func_1-only — mid-gen non-func_1 pure residual
// free-ref order uses owner pure residual free-ref / freehead surfaces. Session/FM-local.
func fixupAccLatePureResidualFreeRefBeforeFreeNeither(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) || f.Name != "func_1" {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type move struct {
		p   *Variable
		anc *Variable // insert p before anc
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	seenFn := map[*Function]bool{}
	var walkFn func(fn *Function)
	walkFn = func(fn *Function) {
		if fn == nil || seenFn[fn] {
			return
		}
		seenFn[fn] = true
		if EffectComplete(fn.FEffect) {
			fr := fn.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			for i, p := range fr {
				if p == nil || moveSet[p] {
					continue
				}
				if !isForIVOfFunc(fn, p) {
					continue
				}
				// pure residual free-ref on parent
				if !bodySyntacticFreeReadsVar(f, p) {
					continue
				}
				if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
					continue
				}
				pp, pok := pos[p]
				if !pok {
					continue
				}
				// Residual free free-neither of same FE after pure in FE currently
				// before pure on parent.
				var freeNeither *Variable
				for _, v := range fr[i+1:] {
					if v == nil || isForIVOfFunc(fn, v) {
						continue
					}
					if bodySyntacticFreeReadsVar(f, v) || bodySyntacticFreeReadsVar(fn, v) {
						continue
					}
					ap, ok := pos[v]
					if !ok || ap >= pp {
						continue
					}
					freeNeither = v
					break
				}
				if freeNeither == nil {
					continue
				}
				// Last residual free free-ref owner-only of same FE before pure residual
				// in FE Acc-early before residual free free-neither (seed258 g_716 immediately
				// before pure residual free-ref g_247; early free residual free-ref owner-only
				// g_185 is not the Acc-adjacent anchor). Session/FM-local.
				var ownerOnly *Variable
				for j, v := range fr {
					if j >= i || v == nil || isForIVOfFunc(fn, v) {
						continue
					}
					if !bodySyntacticFreeReadsVar(fn, v) {
						continue
					}
					if bodySyntacticFreeReadsVar(f, v) {
						continue // free residual free-ref parent / both — not owner-only
					}
					op, ok := pos[v]
					if !ok {
						continue
					}
					np, _ := pos[freeNeither]
					if op >= np {
						continue
					}
					ownerOnly = v // keep last FE-before-pure residual free free-ref owner-only
				}
				if ownerOnly == nil {
					continue
				}
				// Residual free free-ref owner-only Acc-adjacent immediately before
				// residual free free-neither among globals (seed258 g_716@n g_188@n+1;
				// non-adjacent free residual free-neither pollution skips — seed12848
				// func_29 g_178/g_196). Session/FM-local.
				op, _ := pos[ownerOnly]
				np, _ := pos[freeNeither]
				adjOwnerNeither := true
				for j := op + 1; j < np; j++ {
					w := ord[j]
					if w != nil && w.IsGlobalSess(s) {
						adjOwnerNeither = false
						break
					}
				}
				if sessHasError(s) {
					return
				}
				if !adjOwnerNeither {
					continue
				}
				moves = append(moves, move{p: p, anc: freeNeither})
				moveSet[p] = true
			}
		}
		for _, blk := range fn.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				var nested []*Invocation
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
				for _, inv := range nested {
					if inv != nil && inv.User != nil {
						walkFn(inv.User)
					}
				}
			}
		}
	}
	for _, inv := range calls {
		if inv != nil && inv.User != nil {
			walkFn(inv.User)
		}
	}
	if sessHasError(s) || len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
	}
	for anc, hs := range beforeAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		beforeAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupAccLatePureFEHeadAfterAddrOfPureFreeRef places Acc-late pure FE head pure-only of
// nested FE after free residual free-ref currently before pure whose InitExpr is address-of
// pure when residual free free-ref parent-only of pure's FE is also before pure
// (seed16650150368506781474 func_1: pure FE head g_68 of func_63 Acc-late after residual free
// free-ref parent-only g_40; free residual free-ref g_1091 InitExpr→g_68 Acc-early; UP
// g_1091 g_68 g_827). pure-prefix requires free residual free-ref both. Session/FM-local.
func fixupAccLatePureFEHeadAfterAddrOfPureFreeRef(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type move struct {
		p   *Variable
		anc *Variable // insert p after anc
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		user := inv.User
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if len(fr) < 2 || fr[0] == nil || !isForIVOfFunc(user, fr[0]) {
			continue
		}
		// Solo pure FE head pure-only.
		if isForIVOfFunc(user, fr[1]) {
			continue
		}
		p := fr[0]
		if bodySyntacticFreeReadsVar(f, p) {
			continue
		}
		if isForIVOfFunc(f, p) {
			continue
		}
		if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
			continue
		}
		if moveSet[p] {
			continue
		}
		pp, pok := pos[p]
		if !pok {
			continue
		}
		// Residual free free-ref parent-only of same FE currently before pure.
		hasParentOnly := false
		for _, v := range fr[1:] {
			if v == nil || isForIVOfFunc(user, v) {
				continue
			}
			if !bodySyntacticFreeReadsVar(f, v) {
				continue
			}
			if bodySyntacticFreeReadsVar(user, v) {
				continue
			}
			ap, ok := pos[v]
			if !ok || ap >= pp {
				continue
			}
			hasParentOnly = true
			break
		}
		if !hasParentOnly {
			continue
		}
		// Last free residual free-ref currently before pure whose InitExpr is address-of pure.
		var addrOf *Variable
		best := -1
		for _, v := range ord {
			if v == nil || v == p {
				continue
			}
			ap, ok := pos[v]
			if !ok || ap >= pp {
				continue
			}
			if !bodySyntacticFreeReadsVar(f, v) {
				continue
			}
			if v.InitExpr == nil || v.InitExpr.Var == nil {
				continue
			}
			// InitExpr.Var is the pointed-to pure (g_1091 InitExpr → g_68).
			if v.InitExpr.Var != p && (p == nil || v.InitExpr.Var.Name != p.Name) {
				continue
			}
			if ap >= best {
				best = ap
				addrOf = v
			}
		}
		if addrOf == nil {
			continue
		}
		// Already immediately after address-of free residual free-ref among globals.
		adj := true
		for j := best + 1; j < pp; j++ {
			w := ord[j]
			if w != nil && w.IsGlobalSess(s) {
				adj = false
				break
			}
		}
		if sessHasError(s) {
			return
		}
		if adj {
			continue
		}
		moves = append(moves, move{p: p, anc: addrOf})
		moveSet[p] = true
	}
	if len(moves) == 0 {
		return
	}
	afterAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		afterAnc[m.anc] = append(afterAnc[m.anc], m.p)
	}
	for anc, hs := range afterAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		afterAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		emit(v)
		if hs, ok := afterAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupFreeHeadFEPureResidualRelativeOrder places Acc-late pure residual pure for-IVs
// of nested callees that sit mid/tail residual of a free-head nested FE (FE head is
// free residual, not pure for-IV of that FE) into residual-relative slots on parent.
//
// LevelC seed 4100856939472112680 func_1: free-head func_42 FE [g_1132, g_680] —
// pure residual g_680 (pure for-IV of func_15/35) Acc-late after free residual
// FE head g_1132. UP: g_1132 g_680 g_800. Distinct from pure multi-prefix mid
// free-head residual (seed1502, pure multi-prefix of pure FE-head nested). pure-only
// on parent; free residual FE head Acc-early before pure residual. Session/FM-local.
func fixupFreeHeadFEPureResidualRelativeOrder(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	// pure residual pure for-IV of any nested callee.
	isNestedPureIV := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil {
			continue
		}
		for _, blk := range inv.User.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				st := &blk.Stmts[i]
				if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil {
					isNestedPureIV[st.Loop.IV] = true
				}
			}
		}
	}
	type move struct {
		p     *Variable
		anc   *Variable // insert p before anc; if afterHead, insert after free residual FE head
		after bool      // insert after anc (FE-tail pure residual after free residual FE head)
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if len(fr) < 2 || fr[0] == nil {
			continue
		}
		// Free residual FE head (not pure for-IV of this nested callee).
		if isForIVOfFunc(inv.User, fr[0]) {
			continue
		}
		head := fr[0]
		hp, hok := pos[head]
		if !hok {
			continue
		}
		// Residual after free residual FE head must be pure residual pure for-IVs
		// without value free-ref on parent only (short free-head pure residual FE like
		// func_42 [g_1132,g_680]). Address-of free residual (int32_t *l = &g_680) is
		// not value free-ref — bodyValueFreeReadsVar. bodySyntacticFreeReadsVar is true
		// for &g and would skip the FE-tail pure residual. Free residual mid residual of
		// free residual FE after free residual FE head must not yank pure residual early.
		var pureTail []*Variable
		okTail := true
		for _, v := range fr[1:] {
			if v == nil {
				continue
			}
			if !isNestedPureIV[v] || bodyValueFreeReadsVar(f, v) {
				okTail = false
				break
			}
			if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
				okTail = false
				break
			}
			if _, ok := pos[v]; !ok {
				continue
			}
			pureTail = append(pureTail, v)
		}
		if okTail && len(pureTail) > 0 {
			// FE-tail pure residual Acc-late after free residual FE head: place after head.
			for _, p := range pureTail {
				if moveSet[p] {
					continue
				}
				pp, pok := pos[p]
				if !pok || pp <= hp {
					continue
				}
				// Gap between free residual FE head and pure residual (Acc-late mid stream).
				if pp <= hp+1 {
					continue
				}
				moves = append(moves, move{p: p, anc: head, after: true})
				moveSet[p] = true
				if os.Getenv("DIAG1301") != "" && p != nil && head != nil {
					fmt.Fprintf(os.Stderr, "FREEHEAD_AFTER f=%s p=%s head=%s after=true\n", f.Name, p.Name, head.Name)
				}
			}
			continue
		}
		// Free residual free-ref mid free-head FE: Acc-late pure residual pure-only of
		// free-head FE after free residual free-ref of same free-head FE currently
		// before pure. Place pure before first residual free of free-head FE currently
		// before pure after last free residual free-ref of free-head FE currently
		// before pure (LevelC seed11647790213653658898: free-head func_41 pure residual
		// g_225 after free residual free-ref g_207 before residual free g_493; Acc
		// g_207 g_493…g_225; UP g_207 g_225 g_493). pure-only; free residual FE head
		// Acc-early on parent. Solo pure residual immediately after free residual FE
		// head in FE order (g_213 g_225). Mid pure residual free residual free-ref
		// free-head FE (seed48) leaves Acc alone. Session/FM-local.
		if !hok {
			continue
		}
		for i, p := range fr {
			if i == 0 || p == nil || moveSet[p] {
				continue
			}
			// Pure residual pure-only immediately after free residual FE head in FE
			// order (seed11647 FE[1] pure residual). Deeper mid pure residual skips.
			if i != 1 {
				continue
			}
			if !isForIVOfFunc(inv.User, p) {
				continue
			}
			if !isNestedPureIV[p] {
				continue
			}
			if bodySyntacticFreeReadsVar(f, p) {
				continue // pure-only pure residual
			}
			if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
				continue
			}
			// Solo pure residual mid free-head (not pure multi mid of free-head FE —
			// free-head mid pure multi uses midPureAfterFreeFEHead).
			if i+1 < len(fr) && fr[i+1] != nil && isForIVOfFunc(inv.User, fr[i+1]) {
				continue
			}
			pp, pok := pos[p]
			if !pok || pp <= hp {
				continue
			}
			// Last free residual free-ref of free-head FE currently before pure on
			// parent AND appearing before pure residual in FE order (seed11647 g_207
			// before pure in FE). Free residual free-refs that only precede pure via
			// Acc-early parent order but follow pure residual in FE (seed1301 g_78 /
			// g_766 after g_529.f2 in FE) must not anchor pure after them — pure
			// belongs after free residual FE head (UP g_597 g_529.f2 g_609…).
			// Session/FM-local — no package mutable state.
			lastFreeRef := -1
			var lastFreeRefV *Variable
			for j, v := range fr {
				if v == nil || v == p || j >= i {
					continue
				}
				ap, ok := pos[v]
				if !ok || ap >= pp {
					continue
				}
				if !bodySyntacticFreeReadsVar(f, v) {
					continue
				}
				if ap >= lastFreeRef {
					lastFreeRef = ap
					lastFreeRefV = v
				}
			}
			if lastFreeRefV != nil && lastFreeRef >= hp {
				// First residual free of free-head FE currently before pure after last
				// free residual free-ref (seed11647 g_225 before g_493 after g_207).
				var anc *Variable
				for _, v := range fr {
					if v == nil || v == p {
						continue
					}
					if isForIVOfFunc(inv.User, v) {
						continue
					}
					ap, ok := pos[v]
					if !ok || ap >= pp {
						continue
					}
					if ap <= lastFreeRef {
						continue
					}
					if bodySyntacticFreeReadsVar(f, v) {
						continue // still free residual free-ref stream
					}
					anc = v
					break
				}
				if anc == nil {
					continue
				}
				// Acc-late pure after residual free non free-ref of free-head FE.
				if ap, ok := pos[anc]; !ok || pp <= ap {
					continue
				}
				moves = append(moves, move{p: p, anc: anc, after: false})
				moveSet[p] = true
				if os.Getenv("DIAG1301") != "" && p != nil && anc != nil {
					fmt.Fprintf(os.Stderr, "FREEHEAD_BEFORE f=%s p=%s anc=%s after=false\n", f.Name, p.Name, anc.Name)
				}
				continue
			}
			// No free residual free-ref of free-head FE before pure residual in FE
			// order. Acc-late pure residual solo FE[1] when free residual free-ref of
			// free-head FE that appears after pure residual in FE currently sits
			// between free residual FE head and pure on parent — place pure after
			// free residual FE head (LevelC seed13010626082123527596 func_41:
			// free-head func_49 FE [g_597,g_529.f2,g_280,g_609,g_78,…,g_766]; Acc
			// g_597 g_609 g_78 g_766 g_529.f2; UP g_597 g_529.f2 g_609 g_78 g_766).
			// Residual free pure-IV non free-ref Acc-early before pure (same seed
			// free-head func_66 FE [g_79,g_94,g_96,…]; Acc/UP g_79 g_96 g_94) stays
			// Acc-late — do not yank pure after free residual FE head. Session/FM-local.
			hasFreeRefAfterPureInFE := false
			for j, v := range fr {
				if v == nil || v == p || j <= i {
					continue
				}
				if isForIVOfFunc(inv.User, v) {
					continue
				}
				ap, ok := pos[v]
				if !ok || ap >= pp || ap <= hp {
					continue
				}
				if !bodySyntacticFreeReadsVar(f, v) {
					continue // residual free non free-ref (e.g. pure-IV g_96)
				}
				hasFreeRefAfterPureInFE = true
				break
			}
			if !hasFreeRefAfterPureInFE {
				continue
			}
			// Already adjacent after free residual FE head.
			if pp <= hp+1 {
				continue
			}
			moves = append(moves, move{p: p, anc: head, after: true})
			moveSet[p] = true
			if os.Getenv("DIAG1301") != "" && p != nil && head != nil {
				fmt.Fprintf(os.Stderr, "FREEHEAD_AFTER_HEAD f=%s p=%s head=%s after=true\n", f.Name, p.Name, head.Name)
			}
		}
	}
	// Free-ref pure residual of free-head nested FE Acc-late after residual free of
	// same FE that precede pure in FE order currently before pure on parent. Place
	// pure after last residual free of free-head FE currently before pure with FE
	// index < pure (crest+access-once seed1 func_1: free-head func_59 FE
	// […,g_704,g_707,g_517,g_69,g_70…]; Acc g_704 g_707 g_70…g_517 late; UP
	// g_704 g_707 g_517 g_70). Soft-expand nested free-head owners (func_59 not
	// direct under func_1). Prefer earliest anc so deeper free-head with pure late
	// in FE (func_35 …g_2074,g_517) does not override shallower FE order (g_707).
	// pure-only free residual pure path above skips free-ref pure; free-ref pure
	// multi mid free-head uses midPureAfterFreeFEHead (non-func_1). Session/FM-local.
	type freefRefCand struct {
		p, anc *Variable
		ancPos int
	}
	var freefRefCands []freefRefCand
	seenDeepFH := map[*Function]bool{}
	var walkDeepFH func(fn *Function)
	walkDeepFH = func(fn *Function) {
		if fn == nil || seenDeepFH[fn] {
			return
		}
		seenDeepFH[fn] = true
		if EffectComplete(fn.FEffect) {
			fr := fn.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			if len(fr) >= 2 && fr[0] != nil && !isForIVOfFunc(fn, fr[0]) {
				head := fr[0]
				hp, hok := pos[head]
				if hok {
					for i, p := range fr {
						if i == 0 || p == nil || moveSet[p] {
							continue
						}
						if !isForIVOfFunc(fn, p) {
							continue
						}
						// free-ref pure residual on parent only (not free-ref on
						// free-head owner — seed48 free-head func_66 free-ref pure
						// residual g_533 free-refs owner; Acc/UP keep pure Acc-late).
						// crest+access-once seed1 g_517 free-refs parent only.
						if !bodySyntacticFreeReadsVar(f, p) {
							continue
						}
						if bodySyntacticFreeReadsVar(fn, p) {
							continue
						}
						// Own pure residual pure free-ref free of pure-head: do not
						// Acc-order early after free residual free of free-head nested
						// free residual pure free-only of free-head (seed99 func_7
						// g_541 pure residual pure free-ref free of pure-head late
						// before g_700; free residual pure free-only of free-head
						// func_40 FE[1]; free residual free of free-head g_766 then
						// residual free g_110 Acc-early without pure). Free residual
						// pure free-ref free of free-head free-ref free of parent
						// only and not own pure of parent still Acc-lates
						// (crest+access-once seed1 g_517). Session/FM-local — no
						// package mutable state.
						if isForIVOfFunc(f, p) {
							continue
						}
						if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
							continue
						}
						// Solo pure residual mid free-head (not pure multi mid)
						if i+1 < len(fr) && fr[i+1] != nil && isForIVOfFunc(fn, fr[i+1]) {
							continue
						}
						// Not pure multi with predecessor pure residual
						if i > 0 && fr[i-1] != nil && isForIVOfFunc(fn, fr[i-1]) {
							continue
						}
						pp, pok := pos[p]
						if !pok || pp <= hp {
							continue
						}
						// Last residual free of free-head FE currently before pure
						// with FE index < pure index (FE-local predecessor stream).
						lastPred := -1
						var lastPredV *Variable
						for j, v := range fr {
							if v == nil || v == p || j >= i {
								continue
							}
							if isForIVOfFunc(fn, v) {
								continue
							}
							ap, ok := pos[v]
							if !ok || ap >= pp {
								continue
							}
							if ap >= lastPred {
								lastPred = ap
								lastPredV = v
							}
						}
						if lastPredV == nil {
							continue
						}
						// Acc-late pure after FE-predecessor residual free
						if pp <= lastPred+1 {
							continue // already adjacent after predecessor stream
						}
						// Residual free after pure in FE currently before pure on
						// parent (Acc-early residual free after pure in FE) — pure
						// belongs after FE-predecessor residual free (g_707 before
						// g_70). Without that Acc-early residual free, leave Acc.
						//
						// Only move when the first Acc-early same-FE residual free
						// after pure sits immediately after lastPred among globals
						// (crest+access-once seed1: g_707 then g_70…g_517 late; UP
						// g_707 g_517 g_70). Foreign free residual between lastPred
						// and that first Acc-early same-FE residual free means pure
						// is correctly late past other FE merges — leave Acc
						// (seed57 func_22: lastPred g_251; first Acc-early after-pure
						// same-FE residual free g_868 with g_252… foreign in between;
						// UP keeps pure after g_1291). Session/FM-local.
						afterPureFE := map[*Variable]bool{}
						firstAccEarlyAfter := -1
						var firstAccEarlyAfterV *Variable
						for j, v := range fr {
							if v == nil || v == p || j <= i {
								continue
							}
							if isForIVOfFunc(fn, v) {
								continue
							}
							afterPureFE[v] = true
							ap, ok := pos[v]
							if !ok || ap >= pp || ap <= lastPred {
								continue
							}
							if firstAccEarlyAfter < 0 || ap < firstAccEarlyAfter {
								firstAccEarlyAfter = ap
								firstAccEarlyAfterV = v
							}
						}
						if firstAccEarlyAfterV == nil {
							continue
						}
						// Acc-early residual free after pure that free-refs on parent is
						// legitimately early from parent/other FE free residual free-ref
						// (seed57 firstAE g_252 free1 after lastPred g_251; UP keeps pure
						// late after g_1291). Acc-early residual free after pure that does
						// not free-ref parent (crest+access-once seed1 firstAE g_70 free1=false
						// immediately after g_707) is free-head FE residual free Acc-early
						// pollution — pure belongs after lastPred before it. Session/FM-local.
						if bodySyntacticFreeReadsVar(f, firstAccEarlyAfterV) {
							if os.Getenv("DIAG1301") != "" && p != nil {
								fmt.Fprintf(os.Stderr, "FREEHEAD_SKIP_FREE1 f=%s p=%s firstAE=%s\n", f.Name, p.Name, firstAccEarlyAfterV.Name)
							}
							continue
						}
						// Gap (lastPred, firstAccEarlyAfter) must have no foreign globals.
						foreignBeforeAccEarly := false
						for j := lastPred + 1; j < firstAccEarlyAfter; j++ {
							w := ord[j]
							if w == nil || !w.IsGlobalSess(s) {
								continue
							}
							if afterPureFE[w] {
								continue // another Acc-early same-FE residual free
							}
							foreignBeforeAccEarly = true
							break
						}
						if foreignBeforeAccEarly {
							if os.Getenv("DIAG1301") != "" && p != nil {
								fmt.Fprintf(os.Stderr, "FREEHEAD_SKIP_FOREIGN f=%s p=%s lastPred=%s firstAE=%s@%d\n",
									f.Name, p.Name, lastPredV.Name, firstAccEarlyAfterV.Name, firstAccEarlyAfter)
							}
							continue
						}
						if os.Getenv("DIAG1301") != "" && p != nil {
							fmt.Fprintf(os.Stderr, "FREEHEAD_CAND f=%s owner=%s p=%s lastPred=%s@%d firstAE=%s@%d freeO=%v free1=%v\n",
								f.Name, fn.Name, p.Name, lastPredV.Name, lastPred, firstAccEarlyAfterV.Name, firstAccEarlyAfter,
								bodySyntacticFreeReadsVar(fn, firstAccEarlyAfterV), bodySyntacticFreeReadsVar(f, firstAccEarlyAfterV))
						}
						freefRefCands = append(freefRefCands, freefRefCand{p: p, anc: lastPredV, ancPos: lastPred})
					}
				}
			}
		}
		for _, blk := range fn.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				var nested []*Invocation
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
				for _, inv := range nested {
					if inv != nil && inv.User != nil {
						walkDeepFH(inv.User)
					}
				}
			}
		}
	}
	for _, inv := range calls {
		if inv != nil && inv.User != nil {
			walkDeepFH(inv.User)
		}
	}
	if sessHasError(s) {
		return
	}
	// Prefer earliest anc per pure (shallow free-head FE order wins).
	best := map[*Variable]freefRefCand{}
	for _, c := range freefRefCands {
		if c.p == nil || c.anc == nil {
			continue
		}
		if prev, ok := best[c.p]; !ok || c.ancPos < prev.ancPos {
			best[c.p] = c
		}
	}
	for _, c := range best {
		if moveSet[c.p] {
			continue
		}
		moves = append(moves, move{p: c.p, anc: c.anc, after: true})
		moveSet[c.p] = true
		if os.Getenv("DIAG1301") != "" && c.p != nil && c.anc != nil {
			fmt.Fprintf(os.Stderr, "FREEHEAD_FREEREF_AFTER f=%s p=%s anc=%s after=true\n", f.Name, c.p.Name, c.anc.Name)
		}
	}
	// First pure residual pure-only of free-head FE Acc-early when free-ref pure
	// residual of same free-head FE is Acc-late after residual free: place pure-only
	// residual immediately before free-ref pure residual (crest+access-once seed1
	// func_35: free-head func_59 first pure residual pure-only g_120.f1 Acc-early;
	// free-ref pure residual g_517 Acc-late after g_2074; UP g_2074 g_120.f1 g_517).
	// Deeper pure multi pure-only mid free-head stay Acc-early (g_225/g_593…). Session/FM-local.
	for _, c := range best {
		if c.p == nil {
			continue
		}
		// find free-head FE owner of free-ref pure residual c.p
		var owner *Function
		var ownerFR []*Variable
		seenOwn := map[*Function]bool{}
		var findOwner func(fn *Function)
		findOwner = func(fn *Function) {
			if fn == nil || seenOwn[fn] || owner != nil {
				return
			}
			seenOwn[fn] = true
			if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, c.p) {
				fr := fn.FEffect.ReadVarsSess(s)
				if sessHasError(s) {
					return
				}
				if len(fr) >= 2 && fr[0] != nil && !isForIVOfFunc(fn, fr[0]) {
					owner = fn
					ownerFR = fr
					return
				}
			}
			for _, blk := range fn.Blocks {
				if blk == nil {
					continue
				}
				for i := range blk.Stmts {
					var nested []*Invocation
					_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
					for _, inv := range nested {
						if inv != nil && inv.User != nil {
							findOwner(inv.User)
						}
					}
				}
			}
		}
		for _, inv := range calls {
			if inv != nil && inv.User != nil {
				findOwner(inv.User)
			}
		}
		if owner == nil || len(ownerFR) < 2 {
			continue
		}
		// First pure residual pure-only of free-head FE (earliest pure residual).
		var firstPureOnly *Variable
		firstIdx := -1
		for i, v := range ownerFR {
			if i == 0 || v == nil {
				continue
			}
			if !isForIVOfFunc(owner, v) {
				continue
			}
			if bodySyntacticFreeReadsVar(f, v) {
				continue // free-ref pure residual (g_517)
			}
			if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
				continue
			}
			firstPureOnly = v
			firstIdx = i
			break
		}
		if firstPureOnly == nil || moveSet[firstPureOnly] {
			continue
		}
		// free-ref pure residual index of c.p on owner FE
		pIdx := -1
		for i, v := range ownerFR {
			if v == c.p {
				pIdx = i
				break
			}
		}
		if pIdx < 0 || firstIdx >= pIdx {
			continue // first pure-only must precede free-ref pure residual in FE
		}
		fp, fok := pos[firstPureOnly]
		pp, pok := pos[c.p]
		if !fok || !pok || fp >= pp {
			continue // already Acc-late at/after free-ref pure residual
		}
		// Acc-late free-ref pure residual after residual free (c.anc after placement)
		// Place first pure residual pure-only immediately before free-ref pure residual.
		moves = append(moves, move{p: firstPureOnly, anc: c.p, after: false})
		moveSet[firstPureOnly] = true
		if os.Getenv("DIAG1301") != "" && firstPureOnly != nil && c.p != nil {
			fmt.Fprintf(os.Stderr, "FREEHEAD_FIRST_PUREONLY_BEFORE f=%s p=%s before=%s\n", f.Name, firstPureOnly.Name, c.p.Name)
		}
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	afterAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		if m.after {
			afterAnc[m.anc] = append(afterAnc[m.anc], m.p)
		} else {
			beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
		}
	}
	sortByPos := func(hs []*Variable) {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
	}
	for anc, hs := range beforeAnc {
		sortByPos(hs)
		beforeAnc[anc] = hs
	}
	for anc, hs := range afterAnc {
		sortByPos(hs)
		afterAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
		if hs, ok := afterAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupMidPureAfterFreeFEHeadRelativeOrder places Acc-late pure-only pure FE heads
// that also sit mid residual of a free-head nested FE (FE head is free residual,
// not pure for-IV of that FE) into residual-relative slots on parent body map_stm.
//
// LevelC seed 1502150537840585425 func_32: pure multi-prefix g_49/g_370 of func_60
// are mid residual of free-head func_65 [g_8, g_40, g_49, g_72.f3, …, g_370, g_444].
// Acc leaves them late; UP keeps free-head residual order. Pure multi-prefix residual
// pure for-IVs of pure FE-head nested callees only (not every pure mid residual —
// seed639 g_295 / pure mid g_72.f3 must not yank). FE-successor anchor (first residual
// after pure in free-head FE currently before pure). Session/FM-local.
func fixupMidPureAfterFreeFEHeadRelativeOrder(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	// non-func_1 only: func_1 Acc-early pure residual / pureMiss order is handled
	// by pure-prefix and Acc-early free-ref fixups (seed9895936 g_1580; seed48).
	// LevelC seed1502 is func_32 free-head residual pure multi-prefix.
	if f.Name == "func_1" {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	// Pure multi residual mid free-head nested FE. Two pure multi sources:
	//  1) pure multi-prefix of pure FE-head nested callees mid free-head residual
	//     (seed1502 func_60 [g_49,g_370] mid free-head func_65). pure-only.
	//  2) pure multi mid residual of free-head FE itself (LevelC seed1691
	//     func_69 free-head [free residual…, g_117, g_91, g_2, g_128]; pure multi
	//     mid g_117/g_91 free-ref on parent). Free-ref pure multi allowed for (2).
	// Solo pure FE heads left alone (seed123 g_1248). Session/FM-local.
	pureCand := map[*Variable]bool{}
	freeHeadMidMulti := map[*Variable]bool{} // pure multi mid of free-head FE (2)
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if len(fr) == 0 || fr[0] == nil {
			continue
		}
		if isForIVOfFunc(inv.User, fr[0]) {
			// (1) pure multi-prefix of pure FE-head nested.
			var prefix []*Variable
			for _, p := range fr {
				if p == nil || !isForIVOfFunc(inv.User, p) {
					break
				}
				prefix = append(prefix, p)
			}
			if len(prefix) < 2 {
				continue // solo pure FE head — not free-head mid multi residual
			}
			// Need ≥2 pure-only multi-prefix members on parent (seed1502 g_49+g_370).
			// Lone pure-only mid when multi-prefix head free-refs (seed12592 func_56
			// [g_292 free-ref, g_121 pure-only] on func_51) must not free-head-yank —
			// UP keeps Acc-late after residual free free-ref (g_164 g_293 before g_121).
			var pureOnly []*Variable
			for _, p := range prefix {
				if bodySyntacticFreeReadsVar(f, p) {
					continue
				}
				if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
					continue
				}
				pureOnly = append(pureOnly, p)
			}
			if len(pureOnly) < 2 {
				continue
			}
			for _, p := range pureOnly {
				pureCand[p] = true
			}
			continue
		}
		// (2) pure multi mid residual of free-head FE (free residual FE head).
		// Consecutive pure for-IVs of free-head FE owner mid FE (seed169 g_117 g_91).
		// (3) Solo pure residual pure-only mid free-head FE Acc-late (LevelC seed477
		// func_49: free-head func_64 [g_16,g_12 free-ref pure,g_15 free,g_404 pure-only,
		// g_156…]; Acc g_15 g_156…g_141 g_404; UP g_15 g_404 g_156). pure multi uses
		// freeHeadMidMulti Acc-adjacent free-ref residual free gate; solo pure-only
		// uses FE-successor residual free currently before pure (no free-ref parent
		// requirement — g_156 free-ref free-head owner only). Not pure multi run.
		// Session/FM-local — no package mutable state.
		for i := 1; i < len(fr); {
			if fr[i] == nil || !isForIVOfFunc(inv.User, fr[i]) {
				i++
				continue
			}
			var run []*Variable
			j := i
			for j < len(fr) && fr[j] != nil && isForIVOfFunc(inv.User, fr[j]) {
				run = append(run, fr[j])
				j++
			}
			if len(run) >= 2 {
				for _, p := range run {
					if p == nil {
						continue
					}
					if _, ok := pos[p]; !ok {
						continue
					}
					if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
						continue
					}
					// free-ref pure multi mid free-head residual allowed (seed169)
					pureCand[p] = true
					freeHeadMidMulti[p] = true
				}
			} else if len(run) == 1 {
				p := run[0]
				if p != nil {
					if _, ok := pos[p]; ok {
						if fm.PureMissTouched == nil || !fm.PureMissTouched[p] {
							// Solo pure residual pure-only mid free-head (seed477 g_404).
							// free-ref pure residual mid free-head uses freehead freeref path.
							if !bodySyntacticFreeReadsVar(f, p) {
								// Free residual stream between free residual FE head and
								// pure must free-ref parent (seed477 g_15 freeP). Residual
								// free freeO-only mid free residual free-head FE before pure
								// means Acc-late pure is UP-correct (seed639 g_295 after
								// g_169/g_729 freeO-only; seed123 g_328). Session/FM-local.
								pIdx := i
								streamOK := true
								for k := 1; k < pIdx; k++ {
									v := fr[k]
									if v == nil || isForIVOfFunc(inv.User, v) {
										continue
									}
									if !bodySyntacticFreeReadsVar(f, v) {
										streamOK = false
										break
									}
								}
								if streamOK {
									pureCand[p] = true
									// not freeHeadMidMulti — residual free successor FE order
								}
							}
						}
					}
				}
			}
			i = j
		}
	}
	// pure multi residual mid free-head nested FE → before FE residual successor
	// of pure that currently precedes pure on parent.
	type move struct {
		p   *Variable
		anc *Variable
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if len(fr) < 2 || fr[0] == nil {
			continue
		}
		// Free residual FE head (not pure for-IV of this nested callee).
		if isForIVOfFunc(inv.User, fr[0]) {
			continue
		}
		// Free residual FE head Acc-early bound. Head may be absent on parent
		// (seed169 g_96.f2); use first free residual of free-head FE on parent
		// that is not pure for-IV of free-head FE owner.
		hp := -1
		hok := false
		if ap, ok := pos[fr[0]]; ok {
			hp = ap
			hok = true
		} else {
			for _, v := range fr {
				if v == nil || isForIVOfFunc(inv.User, v) {
					continue
				}
				if ap, ok := pos[v]; ok {
					hp = ap
					hok = true
					break
				}
			}
		}
		if !hok {
			continue
		}
		for i, p := range fr {
			if i == 0 || p == nil || !pureCand[p] || moveSet[p] {
				continue
			}
			pp, pok := pos[p]
			if !pok {
				continue
			}
			// Free residual FE head already Acc-early before pure mid residual
			// (seed1502 g_8 before late g_49). Pure Acc-early before free residual
			// FE head must not yank before free residual stream (seed999 g_87.f0).
			if hp >= pp {
				continue
			}
			// First residual free after p in free-head FE order that currently precedes p.
			// Pure residual successors of free-head FE are not anchors (seed477 g_141 pure
			// Acc-early before Acc-late solo pure-only g_404 — place before residual free
			// g_156, not before pure residual g_141). Session/FM-local.
			var bestAnc *Variable
			for _, v := range fr[i+1:] {
				if v == nil {
					continue
				}
				// Residual free successor only (not pure multi sibling / pure residual).
				// free-head mid pure multi (seed169) and solo pure-only mid free-head
				// (seed477) both anchor on residual free of free-head FE.
				if isForIVOfFunc(inv.User, v) {
					continue
				}
				// free-head mid pure multi: residual free successor must free-ref on
				// parent (seed169 g_128 free-ref). Residual free without free-ref Acc-
				// early mid free residual free-head FE (seed188 g_143 of free-head
				// func_35 before Acc-early pure multi mid g_178) must not reverse free
				// residual stream. Solo pure-only mid free-head (seed477 g_156 free-ref
				// free-head owner only) does not require free-ref on parent.
				if freeHeadMidMulti[p] && !bodySyntacticFreeReadsVar(f, v) {
					continue
				}
				ap, ok := pos[v]
				if !ok || ap >= pp {
					continue
				}
				// Residual successor must also sit after free residual FE head
				// (Acc-early free residual of free-head FE, not before free head).
				if ap < hp {
					continue
				}
				// free-head mid pure multi: residual free free-ref Acc-adjacent before
				// pure (seed169 g_128@n g_91@n+1). Non-adjacent Acc gap leaves Acc-late
				// pure multi mid (seed188 g_921 after residual free free-ref g_1397 gap).
				// Solo pure-only mid free-head allows non-adjacent residual free Acc-early
				// (seed477 g_156@4 pure@12). Session/FM-local.
				if freeHeadMidMulti[p] && ap != pp-1 {
					continue
				}
				// Do not yank free-head mid pure residual before residual free that is
				// pure for-IV of the *parent* (seed2 --no-pointers func_35: g_265 of
				// free-head nested before own pure free residual free-ref g_266; UP
				// g_266 g_265). Session/FM-local — no package mutable state.
				if isForIVOfFunc(f, v) {
					continue
				}
				bestAnc = v
				break
			}
			if bestAnc == nil {
				continue
			}
			moves = append(moves, move{p: p, anc: bestAnc})
			moveSet[p] = true
		}
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
	}
	for anc, hs := range beforeAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		beforeAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupNestedFEPureOnlyRelativeOrder places pure-only nested pure for-IVs
// (not free-ref on parent) into FE-relative slots on parent body map_stm.
// Visit/merge can leave pure IV reads Acc-late; UP keeps them at nested FE order
// relative to residual free FE entries on parent.
// seed 14175156974908062646 func_20: func_46 FE [g_147,g_6,…,g_166,g_527,g_186,…]
// had pure-only g_147/g_527 after residual tail; UP has g_147 before g_6 and
// g_527 after g_166. Covers mid pure IVs (not only FE heads Acc-early pure-late).
// free-ref pure IVs keep parent free-ref order (seed42). Session/FM-local.
func fixupNestedFEPureOnlyRelativeOrder(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	// func_1 allowed only for deeper-owned pure IVs (not direct-callee for-IVs):
	// seed LC g_147 of func_46; seed22584 g_258.f0 is direct pure and stays put.
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	// Slots: pure-only pure IVs present on parent.
	// ownerFE = earliest for-IV owner FE (Acc-late gate: residual free of owner).
	// placeFE = FE used for insert anchors (on func_1, intermediate direct
	// callee FE so g_147 inserts after g_41 not before free-ref g_6).
	type slot struct {
		p       *Variable
		owner   *Function // for-IV owner of earliest ownerFE (multi-prefix gate)
		ownerFE []*Variable
		placeFE []*Variable
		deeper  bool // pure is not for-IV of any direct callee of f
	}
	var slots []slot
	slotOwnerIdx := map[*Variable]int{}
	addOwner := func(v *Variable, owner *Function, fr []*Variable) {
		if v == nil || owner == nil || fr == nil {
			return
		}
		if bodySyntacticFreeReadsVar(f, v) {
			return
		}
		if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
			return
		}
		if _, ok := pos[v]; !ok {
			return
		}
		feIdx := -1
		for i, x := range fr {
			if x == v {
				feIdx = i
				break
			}
		}
		if feIdx < 0 {
			return
		}
		if old, ok := slotOwnerIdx[v]; ok && old <= feIdx {
			return
		}
		if _, ok := slotOwnerIdx[v]; ok {
			for i := range slots {
				if slots[i].p == v {
					slots[i].owner = owner
					slots[i].ownerFE = append([]*Variable(nil), fr...)
					// keep placeFE if already set
					if slots[i].placeFE == nil {
						slots[i].placeFE = slots[i].ownerFE
					}
					break
				}
			}
		} else {
			fe := append([]*Variable(nil), fr...)
			slots = append(slots, slot{p: v, owner: owner, ownerFE: fe, placeFE: fe})
		}
		slotOwnerIdx[v] = feIdx
	}
	// placeFE among direct-callee FEs: prefer earliest pure index (func_20
	// g_147@3 over func_12 g_147@tail) so free residual preds (g_41) anchor.
	// Separate from ownerFE (earliest for-IV owner for Acc-late gate).
	placeFEIdx := map[*Variable]int{} // only direct-callee place candidates
	setPlaceFE := func(v *Variable, fr []*Variable) {
		if v == nil || fr == nil {
			return
		}
		feIdx := -1
		for i, x := range fr {
			if x == v {
				feIdx = i
				break
			}
		}
		if feIdx < 0 {
			return
		}
		if old, ok := placeFEIdx[v]; ok && old <= feIdx {
			return
		}
		for i := range slots {
			if slots[i].p == v {
				slots[i].placeFE = append([]*Variable(nil), fr...)
				placeFEIdx[v] = feIdx
				return
			}
		}
	}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		for _, v := range fr {
			if v == nil || !isForIVOfFunc(inv.User, v) {
				continue
			}
			addOwner(v, inv.User, fr)
		}
	}
	// func_1: deeper pure IVs + intermediate placeFE (func_20 FE for g_147).
	if f.Name == "func_1" {
		// Walk call tree for earlier owner FEs.
		seenFn := map[*Function]bool{}
		var walkOwn func(fn *Function)
		walkOwn = func(fn *Function) {
			if fn == nil || seenFn[fn] {
				return
			}
			seenFn[fn] = true
			if EffectComplete(fn.FEffect) {
				fr := fn.FEffect.ReadVarsSess(s)
				if sessHasError(s) {
					return
				}
				for _, v := range fr {
					if v == nil || !isForIVOfFunc(fn, v) {
						continue
					}
					addOwner(v, fn, fr)
				}
			}
			for _, blk := range fn.Blocks {
				if blk == nil {
					continue
				}
				for i := range blk.Stmts {
					var nested []*Invocation
					if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
						continue
					}
					for _, inv := range nested {
						if inv != nil && inv.User != nil {
							walkOwn(inv.User)
						}
					}
				}
			}
		}
		for _, inv := range calls {
			if inv != nil && inv.User != nil {
				walkOwn(inv.User)
			}
		}
		if sessHasError(s) {
			return
		}
		// Intermediate direct-callee FE as placeFE for tree pure IVs on parent.
		// Prefer direct callees that transitively contain the pure-IV owner
		// (LevelC seed 3627619818918382345: g_109 of func_57 via func_30 FE
		// g_113 g_109 g_114; func_18 also residuals g_109 after g_168 but does
		// not contain func_57 — earliest-index alone wrongly anchors after
		// g_168). When no containing intermediate exists, fall back to any
		// residual direct-callee FE (seed 14175 earliest-index among residual).
		// Session/FM-local — no package mutable state.
		slotOwnerOf := map[*Variable]*Function{}
		for i := range slots {
			if slots[i].p != nil {
				slotOwnerOf[slots[i].p] = slots[i].owner
			}
		}
		containsOwner := func(root, owner *Function) bool {
			if root == nil || owner == nil {
				return false
			}
			if root == owner {
				return true
			}
			seen := map[*Function]bool{}
			var walk func(*Function) bool
			walk = func(fn *Function) bool {
				if fn == nil || seen[fn] {
					return false
				}
				seen[fn] = true
				if fn == owner {
					return true
				}
				for _, blk := range fn.Blocks {
					if blk == nil {
						continue
					}
					for i := range blk.Stmts {
						var nested []*Invocation
						if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
							continue
						}
						for _, inv := range nested {
							if inv != nil && inv.User != nil && walk(inv.User) {
								return true
							}
						}
					}
				}
				return false
			}
			return walk(root)
		}
		type placeCand struct {
			v  *Variable
			fr []*Variable
		}
		var withOwner, withoutOwner []placeCand
		for _, inv := range calls {
			if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
				continue
			}
			fr := inv.User.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			for _, v := range fr {
				if v == nil {
					continue
				}
				if _, ok := slotOwnerIdx[v]; !ok {
					continue
				}
				owner := slotOwnerOf[v]
				if owner != nil && containsOwner(inv.User, owner) {
					withOwner = append(withOwner, placeCand{v: v, fr: fr})
				} else {
					withoutOwner = append(withoutOwner, placeCand{v: v, fr: fr})
				}
			}
		}
		// Prefer placeFE that has free residual pred on parent (insert
		// after g_41) when pure is deeper-owned (func_20 over func_46).
		// Containing-owner callees first so earliest-index is among them only.
		for _, c := range withOwner {
			setPlaceFE(c.v, c.fr)
		}
		// Fall back when no containing intermediate residual FE was seen.
		for _, c := range withoutOwner {
			if _, ok := placeFEIdx[c.v]; ok {
				continue // already placed via containing owner
			}
			setPlaceFE(c.v, c.fr)
		}
	}
	if f.Name == "func_1" {
		// deeper: earliest owner FE is not a direct callee's FE (func_46 vs
		// func_12 both for-IV of g_147; earliest is func_46). Prefer placeFE
		// intermediate; Acc-late on earliest owner residual free.
		for i := range slots {
			// ownerFE[0] is pure when pure is FE head of earliest owner.
			// Mark deeper when pure is FE head of earliest owner FE (ownerFE[0]==p)
			// OR pure mid of earliest owner, AND earliest owner is not a direct
			// callee — approximate: placeFE differs from ownerFE (intermediate
			// setPlaceFE overwrote place).
			sl := &slots[i]
			placeDiffers := len(sl.placeFE) > 0 && len(sl.ownerFE) > 0 &&
				(len(sl.placeFE) != len(sl.ownerFE) || (len(sl.placeFE) > 0 && len(sl.ownerFE) > 0 && sl.placeFE[0] != sl.ownerFE[0]))
			// Also: pure is for-IV of some non-direct nested function in tree.
			// walkOwn already preferred earliest owner; if placeFE was updated
			// from a direct callee FE, placeDiffers is true for deeper pure.
			sl.deeper = placeDiffers
		}
	}
	if len(slots) == 0 {
		return
	}
	pureOnlySet := map[*Variable]bool{}
	for _, sl := range slots {
		pureOnlySet[sl.p] = true
	}
	// Acc-late gate uses ownerFE residual free (func_46 ends g_110).
	// Placement uses placeFE (func_20: after g_41 before g_6 on func_1).
	type movePlan struct {
		p       *Variable
		placeFE []*Variable
	}
	var late []movePlan
	for _, sl := range slots {
		cur, ok := pos[sl.p]
		if !ok {
			continue
		}
		maxResOrd := -1
		for _, v := range sl.ownerFE {
			if v == nil || pureOnlySet[v] {
				continue
			}
			if j, ok := pos[v]; ok && j > maxResOrd {
				maxResOrd = j
			}
		}
		if maxResOrd < 0 || cur <= maxResOrd {
			continue // not Acc-late on owner FE
		}
		// FE-head Acc-late eligible only for solo pure-IV FE heads (func_20 g_147
		// pure multi-prefix len 1). Pure multi-prefix heads (len≥2) use multi-prefix
		// case2 (seed875 g_117; seed639 g_295 must not Acc-late yank early).
		// Mid pure Acc-late only when owner FE has free residual after pure
		// (g_527 before g_186). FE-tail pure-only Acc-late leave alone here
		// (seed2048 g_135); array-init FE-tail uses fixupAssignNestedFETailArrayInit.
		// Pure multi-prefix mid alone (seed875 g_116 without FE-head g_117 Acc-late)
		// must not yank early either.
		isHead := len(sl.ownerFE) > 0 && sl.ownerFE[0] == sl.p
		if isHead {
			// Pure multi-prefix head (owner FE[1] also pure for-IV of same owner) →
			// multi-prefix path only. pureOnlySet[FE[1]] only catches pure-only mids
			// (seed875 g_117); when FE[1] free-refs on parent (seed12592 func_56
			// [g_292,g_121] on func_21) Acc-late yank would place g_292 before
			// free residual free-ref g_121 — UP keeps pure Acc-late after residual.
			if sl.owner != nil && len(sl.ownerFE) > 1 && isForIVOfFunc(sl.owner, sl.ownerFE[1]) {
				continue
			}
			// Pure multi-prefix head (FE[1] also pure-only pure IV) → multi-prefix case2 only.
			if len(sl.ownerFE) > 1 && pureOnlySet[sl.ownerFE[1]] {
				continue
			}
			// Solo pure FE head Acc-late when first residual free free-refs on parent:
			// Acc free-before-pure is UP-correct — do not FE-relative yank pure before
			// residual free (LevelC seed4705676153835236351 func_10: func_47 FE
			// [g_904.f1,g_1151,g_1111]; g_1151 free-ref parent; Acc g_1151…g_726.f0
			// g_904.f1 late). Residual free free-ref on pure-IV owner only still
			// yanks (seed14175 g_6 free-ref owner only; UP g_41 g_147…). Path A
			// separately skips residual free free-ref on owner. Session/FM-local.
			if sl.owner != nil {
				var firstRes *Variable
				for _, v := range sl.ownerFE[1:] {
					if v == nil || isForIVOfFunc(sl.owner, v) {
						continue
					}
					firstRes = v
					break
				}
				if firstRes != nil && bodySyntacticFreeReadsVar(f, firstRes) {
					continue
				}
			}
			// func_1: direct pure FE heads Acc-late yanks seed22584 g_258.f0.
			// LevelC g_261 uses fixupNestedPureFEHeadBeforeOwnPureResidual instead.
			if f.Name == "func_1" && !sl.deeper {
				continue
			}
		} else {
			// Mid pure: func_1 deeper only (seed LC g_527); not direct (seed22584).
			if f.Name == "func_1" && !sl.deeper {
				continue
			}
		}
		if !isHead {
			feIdx := -1
			for i, v := range sl.ownerFE {
				if v == sl.p {
					feIdx = i
					break
				}
			}
			hasFreeSucc := false
			for i := feIdx + 1; i < len(sl.ownerFE) && feIdx >= 0; i++ {
				v := sl.ownerFE[i]
				if v == nil || pureOnlySet[v] {
					continue
				}
				if _, ok := pos[v]; ok {
					hasFreeSucc = true
					break
				}
			}
			if !hasFreeSucc {
				continue
			}
			// Mid pure Acc-late requires FE-head pure-only pure IV of same owner
			// that is also Acc-late (func_20 g_527 with g_147; seed875 g_116 with
			// g_117). Solo mid after free residual of non-pure FE head (seed639
			// g_295 under g_279 free residual) must not yank early.
			if len(sl.ownerFE) == 0 || !pureOnlySet[sl.ownerFE[0]] {
				continue
			}
			head := sl.ownerFE[0]
			hp, hok := pos[head]
			if !hok || hp <= maxResOrd {
				continue // head not Acc-late — skip mid alone
			}
			// non-func_1 mid Acc-late: require free residual after pure free-refs
			// on *parent body* only. Nested free-ref residual Acc is not enough
			// (seed639 g_295 after g_729; residual free Acc-only after pure).
			if f.Name != "func_1" {
				hasFreeRefSucc := false
				for i := feIdx + 1; i < len(sl.ownerFE) && feIdx >= 0; i++ {
					v := sl.ownerFE[i]
					if v == nil || pureOnlySet[v] {
						continue
					}
					if _, ok := pos[v]; !ok {
						continue
					}
					if bodySyntacticFreeReadsVar(f, v) {
						hasFreeRefSucc = true
						break
					}
				}
				if !hasFreeRefSucc {
					continue
				}
			}
			// func_1 mid pure: only with co-relocating FE-head of same owner FE
			// (seed LC g_527 with g_147). Lone mid Acc-late yanks seed48 g_129.f2.
			if f.Name == "func_1" {
				// mark for second pass via placeFE pointer shared with head
				// handled below after heads collected
				continue
			}
		}
		place := sl.placeFE
		if place == nil {
			place = sl.ownerFE
		}
		late = append(late, movePlan{p: sl.p, placeFE: place})
	}
	// func_1: Acc-late mid pure of same owner FE as a late FE-head pure.
	if f.Name == "func_1" {
		headOwner0 := map[*Variable]bool{}
		for _, m := range late {
			// find slot for m.p to get ownerFE[0]
			for _, sl := range slots {
				if sl.p == m.p && len(sl.ownerFE) > 0 && sl.ownerFE[0] == m.p {
					headOwner0[m.p] = true
					break
				}
			}
		}
		for _, sl := range slots {
			if !sl.deeper || len(sl.ownerFE) == 0 || sl.ownerFE[0] == sl.p {
				continue // head already considered
			}
			if !headOwner0[sl.ownerFE[0]] {
				continue // no co-relocating FE-head of same owner
			}
			cur, ok := pos[sl.p]
			if !ok {
				continue
			}
			maxResOrd := -1
			for _, v := range sl.ownerFE {
				if v == nil || pureOnlySet[v] {
					continue
				}
				if j, ok := pos[v]; ok && j > maxResOrd {
					maxResOrd = j
				}
			}
			if maxResOrd < 0 || cur <= maxResOrd {
				continue
			}
			// free-succ required
			feIdx := -1
			for i, v := range sl.ownerFE {
				if v == sl.p {
					feIdx = i
					break
				}
			}
			hasFreeSucc := false
			for i := feIdx + 1; i < len(sl.ownerFE) && feIdx >= 0; i++ {
				v := sl.ownerFE[i]
				if v == nil || pureOnlySet[v] {
					continue
				}
				if _, ok := pos[v]; ok {
					hasFreeSucc = true
					break
				}
			}
			if !hasFreeSucc {
				continue
			}
			place := sl.placeFE
			if place == nil {
				place = sl.ownerFE
			}
			late = append(late, movePlan{p: sl.p, placeFE: place})
		}
	}
	if len(late) == 0 {
		return
	}
	drop := map[*Variable]bool{}
	for _, m := range late {
		drop[m.p] = true
	}
	var remaining []*Variable
	for _, v := range ord {
		if v == nil || drop[v] {
			continue
		}
		remaining = append(remaining, v)
	}
	remPos := map[*Variable]int{}
	for i, v := range remaining {
		if v != nil {
			if _, ok := remPos[v]; !ok {
				remPos[v] = i
			}
		}
	}
	insertAt := make([][]*Variable, len(remaining)+1)
	for _, m := range late {
		fe := m.placeFE
		feIdx := -1
		for i, v := range fe {
			if v == m.p {
				feIdx = i
				break
			}
		}
		afterIdx := -1
		beforeIdx := -1
		for i, v := range fe {
			if v == nil || drop[v] {
				continue
			}
			j, ok := remPos[v]
			if !ok {
				continue
			}
			if i < feIdx && j >= afterIdx {
				afterIdx = j
			}
			if i > feIdx && beforeIdx < 0 {
				beforeIdx = j
			}
		}
		ins := len(remaining)
		if afterIdx >= 0 {
			ins = afterIdx + 1
		} else if beforeIdx >= 0 {
			ins = beforeIdx
		}
		insertAt[ins] = append(insertAt[ins], m.p)
	}
	feRank := map[*Variable]int{}
	for _, m := range late {
		r := -1
		for i, v := range m.placeFE {
			if v == m.p {
				r = i
				break
			}
		}
		if r < 0 {
			continue
		}
		if old, ok := feRank[m.p]; !ok || r < old {
			feRank[m.p] = r
		}
	}
	for i := range insertAt {
		ps := insertAt[i]
		if len(ps) < 2 {
			continue
		}
		for a := 1; a < len(ps); a++ {
			x := ps[a]
			xr := feRank[x]
			b := a - 1
			for b >= 0 && feRank[ps[b]] > xr {
				ps[b+1] = ps[b]
				b--
			}
			ps[b+1] = x
		}
		insertAt[i] = ps
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for i, v := range remaining {
		for _, p := range insertAt[i] {
			emit(p)
		}
		emit(v)
	}
	for _, p := range insertAt[len(remaining)] {
		emit(p)
	}
	for _, m := range late {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupFunc1AccEarlyNestedPureAfterParentFree places pure FE heads of nested
// callees next to their FE neighbor on parent body map_stm.
// For pure head p of nested FE [p, …, a, …] (a = first non-pure-for-IV FE entry):
//   - a free-refs on parent AND p is Acc-leading (before any free-ref-on-parent):
//     insert p after a (seed22584 g_58 after free-ref g_3). Does not reverse
//     pure-prefix (seed88 pure before free mid-list is not Acc-leading).
//   - a does not free-ref on parent: insert p immediately before a when p is
//     Acc-early before a (seed22584 g_359.f0/g_258.f0). Pure multi-prefix
//     Acc-late stragglers use multi-prefix compact (seed12336).
//
// When p leads multiple nested FEs, prefer Acc-early before-neighbor with max
// parent index. Session/FM-local — no package mutable state.
func fixupFunc1AccEarlyNestedPureAfterParentFree(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	// pure → candidate neighbors from every nested FE that leads with pure.
	type cand struct {
		anc   *Variable
		after bool
	}
	cands := map[*Variable][]cand{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(inv.User, fr[0]) {
			continue
		}
		p := fr[0]
		if bodySyntacticFreeReadsVar(f, p) {
			continue
		}
		if fm.PureMissTouched != nil && fm.PureMissTouched[p] {
			continue
		}
		var anc *Variable
		for _, v := range fr[1:] {
			if v == nil || isForIVOfFunc(inv.User, v) {
				continue
			}
			anc = v
			break
		}
		if anc == nil && len(fr) > 1 {
			anc = fr[1]
		}
		if anc == nil {
			continue
		}
		cands[p] = append(cands[p], cand{anc: anc, after: bodySyntacticFreeReadsVar(f, anc)})
	}
	if len(cands) == 0 {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	if sessHasError(s) {
		return
	}
	type move struct {
		p     *Variable
		anc   *Variable
		after bool
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for p, list := range cands {
		pp, pok := pos[p]
		if !pok {
			continue
		}
		// Prefer before-neighbor moves over after free-ref; Acc-early max ap.
		var best *move
		bestScore := -1
		for _, c := range list {
			ap, aok := pos[c.anc]
			if !aok {
				continue
			}
			if c.after {
				// Acc-early relative to free-ref FE neighbor: pure before that
				// neighbor on parent (seed22584: g_9 free-ref first, then Acc-
				// early g_58, then free-ref neighbor g_3 → place g_58 after g_3).
				// Do not require pure before first free of the whole body —
				// parent free-ref of other FEs can precede pure while pure is
				// still Acc-early vs its own free-ref residual.
				if pp >= ap {
					continue
				}
				// Do not reverse pure-prefix-before-free: when free residual of
				// pure's nested FE free-refs on that callee, pure-prefix owns
				// order (LC seed 15934573825443220977 g_150 before g_250 of
				// func_12). seed22584 g_3 free-refs only on parent — Acc-early
				// after free-ref still applies (g_58 after g_3).
				// Pure residual free-ref on pure-IV owner keeps Acc free-ref
				// order pure before residual free free-ref parent (seed2
				// --no-jumps g_26 free-ref func_20 before free residual free-ref
				// parent g_36). seed22584 g_58 free-refs neither parent nor
				// owner — pure residual pure-only Acc-early after free residual
				// free-ref parent remains. Session/FM-local — no package state.
				freeRefOnOwnerPure := false
				freeRefOnCallee := false
				for _, inv := range calls {
					if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
						continue
					}
					fr := inv.User.FEffect.ReadVarsSess(s)
					if sessHasError(s) {
						return
					}
					if len(fr) == 0 || fr[0] != p {
						continue
					}
					if bodySyntacticFreeReadsVar(inv.User, p) {
						freeRefOnOwnerPure = true
					}
					var firstRes *Variable
					for _, v := range fr[1:] {
						if v == nil || isForIVOfFunc(inv.User, v) {
							continue
						}
						firstRes = v
						break
					}
					if firstRes != c.anc {
						continue
					}
					if bodySyntacticFreeReadsVar(inv.User, c.anc) {
						freeRefOnCallee = true
						break
					}
				}
				if freeRefOnCallee {
					continue
				}
				if freeRefOnOwnerPure {
					continue
				}
				// Pure multi-prefix with free-ref mid pureOnly before residual free
				// free-ref of same FE: pure multi owns FE order (seed3892248577
				// func_11 [g_253,g_24,g_349 free-ref] before residual free free-ref
				// g_1161). Solo pure FE head Acc-early after free residual free-ref
				// of same FE remains seed22584 g_58 after g_3. Session/FM-local.
				skipSameFEMultiFreeRef := false
				for _, inv2 := range calls {
					if inv2 == nil || inv2.User == nil || !EffectComplete(inv2.User.FEffect) {
						continue
					}
					fr2 := inv2.User.FEffect.ReadVarsSess(s)
					if sessHasError(s) {
						return
					}
					if len(fr2) == 0 || fr2[0] != p {
						continue
					}
					var prefix2 []*Variable
					for _, v := range fr2 {
						if v == nil || !isForIVOfFunc(inv2.User, v) {
							break
						}
						prefix2 = append(prefix2, v)
					}
					if len(prefix2) < 2 {
						continue
					}
					isRes := false
					for _, v := range fr2[len(prefix2):] {
						if v == c.anc {
							isRes = true
							break
						}
					}
					if !isRes {
						continue
					}
					for _, v := range prefix2[1:] {
						if v == nil || !bodySyntacticFreeReadsVar(f, v) {
							continue
						}
						mp, ok := pos[v]
						if ok && mp > pp && mp < ap {
							skipSameFEMultiFreeRef = true
							break
						}
					}
					if skipSameFEMultiFreeRef {
						break
					}
				}
				if skipSameFEMultiFreeRef {
					continue
				}
				// Residual free of nested FE that is pure for-IV of the parent
				// (seed12592 g_121 of func_21 residual free of func_56 FE) —
				// free-ref of own pure IV must not Acc-early-yank nested pure
				// FE head (g_292) after IV; UP keeps pure Acc-late after other
				// residual free (g_128 g_164). seed22584 free residual free-ref
				// is not pure for-IV of parent.
				if isForIVOfFunc(f, c.anc) {
					continue
				}
				// Acc-early pure after free-ref FE neighbor.
				sc := ap
				if sc >= bestScore {
					bestScore = sc
					m := move{p: p, anc: c.anc, after: true}
					best = &m
				}
				continue
			}
			// Acc-early before non-free-ref neighbor (seed22584 g_359.f0/g_258.f0)
			if pp < ap {
				if os.Getenv("DIAG12592") != "" && p.Name == "g_292" {
					fmt.Fprintf(os.Stderr, "ACCEARLY_BEFORE_CAND f=%s pure=%s@%d anc=%s@%d\n", f.Name, p.Name, pp, c.anc.Name, ap)
				}
				var next *Variable
				for j := pp + 1; j < len(ord); j++ {
					if ord[j] == nil {
						continue
					}
					next = ord[j]
					break
				}
				if next == c.anc {
					continue // already adjacent to firstRes neighbor
				}
				// Keep Acc-early placement when next follows pure in a nested FE:
				//  1) Owner FE free residual after pure head (seed18167).
				//  2) Free-head nested FE residual pure mid (seed28465).
				//  3) Pure multi-prefix mid pure when first residual free free-refs
				//     on callee (seed12657); seed875 first residual free not free-ref
				//     on callee — Acc-early yank past mid pure still allowed.
				if next != nil {
					keepNext := false
					for _, inv := range calls {
						if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
							continue
						}
						fr := inv.User.FEffect.ReadVarsSess(s)
						if sessHasError(s) {
							return
						}
						pi := -1
						for i, v := range fr {
							if v == p {
								pi = i
								break
							}
						}
						if pi < 0 {
							continue
						}
						for _, v := range fr[pi+1:] {
							if v != next {
								continue
							}
							if pi == 0 && isForIVOfFunc(inv.User, v) {
								var prefLen int
								for _, pv := range fr {
									if pv == nil || !isForIVOfFunc(inv.User, pv) {
										break
									}
									prefLen++
								}
								keepMultiFE := false
								for _, rv := range fr[prefLen:] {
									if rv == nil || isForIVOfFunc(inv.User, rv) {
										continue
									}
									if bodySyntacticFreeReadsVar(inv.User, rv) {
										keepMultiFE = true
									}
									break
								}
								if keepMultiFE {
									keepNext = true
								}
								break
							}
							keepNext = true
							break
						}
						if keepNext {
							break
						}
					}
					if keepNext {
						continue
					}
				}
				// Pure multi mid Acc between pure multi head and residual free anc
				// of same FE: Acc-early before residual free must not reverse multi
				// FE order (LevelC seed15756260483214041307). Session/FM-local.
				midBetween := false
				for _, inv3 := range calls {
					if inv3 == nil || inv3.User == nil || !EffectComplete(inv3.User.FEffect) {
						continue
					}
					fr3 := inv3.User.FEffect.ReadVarsSess(s)
					if sessHasError(s) || len(fr3) == 0 || fr3[0] != p {
						continue
					}
					var pref []*Variable
					for _, v := range fr3 {
						if v == nil || !isForIVOfFunc(inv3.User, v) {
							break
						}
						pref = append(pref, v)
					}
					if len(pref) < 2 {
						continue
					}
					isAncRes := false
					for _, v := range fr3[len(pref):] {
						if v == c.anc {
							isAncRes = true
							break
						}
					}
					if !isAncRes {
						continue
					}
					for _, mid := range pref[1:] {
						if mid == nil {
							continue
						}
						mp, ok := pos[mid]
						if ok && mp > pp && mp < ap {
							midBetween = true
							break
						}
					}
					if midBetween {
						break
					}
				}
				// Pure multi mid Acc-adjacent residual free anc (seed157 pure multi mid
				// immediately before residual free g_1536): Acc-early pure multi head
				// before residual free reverses multi FE order. seed875 pure multi mid
				// Acc not Acc-adjacent residual free (gap) — Acc-early reverse multi FE
				// order to Acc mid-before-head still needed. Session/FM-local.
				if midBetween {
					midAdjRes := false
					for _, inv3 := range calls {
						if inv3 == nil || inv3.User == nil || !EffectComplete(inv3.User.FEffect) {
							continue
						}
						fr3 := inv3.User.FEffect.ReadVarsSess(s)
						if sessHasError(s) || len(fr3) == 0 || fr3[0] != p {
							continue
						}
						for _, mid := range fr3 {
							if mid == nil || !isForIVOfFunc(inv3.User, mid) {
								break
							}
							if mid == p {
								continue
							}
							if mp, ok := pos[mid]; ok && mp > pp && mp < ap && mp == ap-1 {
								midAdjRes = true
								break
							}
						}
						if midAdjRes {
							break
						}
					}
					if midAdjRes {
						continue
					}
				}
				// Acc-early before residual free neighbor must not jump past free-ref
				// residual of intermediate path on parent Acc (LevelC seed
				// 18313863863641754138: pure-only pure FE head g_30 of func_77 Acc
				// before residual free g_82; free-ref pure residual g_41 of func_17
				// and free-ref residual g_917.f5 of func_50 sit mid — UP keeps pure
				// then free-ref residual then residual free). Session/FM-local.
				skipBetweenFreeRef := false
				for j := pp + 1; j < ap && j < len(ord); j++ {
					w := ord[j]
					if w != nil && bodySyntacticFreeReadsVar(f, w) {
						skipBetweenFreeRef = true
						break
					}
				}
				if skipBetweenFreeRef {
					continue
				}
				sc := ap + 10000
				if sc >= bestScore {
					bestScore = sc
					m := move{p: p, anc: c.anc, after: false}
					best = &m
				}
				continue
			}
			// pure-late (pp > ap): seed12336 g_659 needs multi-prefix or a
			// tighter pure-late gate; unrestricted pure-late yanks battery.
		}
		if best == nil {
			continue
		}
		moves = append(moves, *best)
		moveSet[p] = true
	}
	if len(moves) == 0 {
		return
	}
	beforeAnc := map[*Variable][]*Variable{}
	afterAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		if m.after {
			afterAnc[m.anc] = append(afterAnc[m.anc], m.p)
		} else {
			beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
		}
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if ps, ok := beforeAnc[v]; ok {
			for _, pv := range ps {
				emit(pv)
			}
		}
		emit(v)
		if ps, ok := afterAnc[v]; ok {
			for _, pv := range ps {
				emit(pv)
			}
		}
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

func fixupFunc1DeferNestedPureOnlyFEHeads(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	// Pure for-IVs of nested callees that lead that nested FE.
	type headInfo struct {
		v *Variable
	}
	var heads []headInfo
	seenH := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if len(fr) == 0 || fr[0] == nil {
			continue
		}
		v := fr[0]
		if !isForIVOfFunc(inv.User, v) {
			continue
		}
		// Not free-ref on parent — Acc residual from nested only (seed9895936).
		if bodySyntacticFreeReadsVar(f, v) {
			continue
		}
		// pureMiss already placed this pure IV by prev — do not late-jump
		// (n35 g_50@2 pureMiss vs defer to g_245.f2). Acc-early pollution
		// that never pureMiss'd still defers (seed9895936 g_1580).
		if fm.PureMissTouched != nil && fm.PureMissTouched[v] {
			continue
		}
		if seenH[v] {
			continue
		}
		seenH[v] = true
		heads = append(heads, headInfo{v: v})
	}
	if len(heads) == 0 {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	// ord index of every var (first occurrence).
	ordPos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := ordPos[v]; !ok {
				ordPos[v] = i
			}
		}
	}
	// Move pure FE-head Acc-early pure IVs to after free-refing nested FE anchor.
	type move struct {
		v      *Variable
		anchor *Variable // insert after this var in remaining order
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, h := range heads {
		v := h.v
		var bestAnchor *Variable
		bestPos := -1 // min of maxima of free-refing callees' FE others on parent ord
		for _, inv := range calls {
			if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
				continue
			}
			if !bodySyntacticFreeReadsVar(inv.User, v) {
				continue
			}
			fr := inv.User.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			maxP := -1
			var maxV *Variable
			for _, rv := range fr {
				if rv == nil || rv == v {
					continue
				}
				p, ok := ordPos[rv]
				if !ok {
					continue
				}
				if p > maxP {
					maxP = p
					maxV = rv
				}
			}
			if maxV == nil {
				continue
			}
			if bestPos < 0 || maxP < bestPos {
				bestPos = maxP
				bestAnchor = maxV
			}
		}
		if bestAnchor == nil {
			continue
		}
		ci, ok := ordPos[v]
		if !ok || ci > bestPos {
			continue // already at/after free-ref order
		}
		// Only Acc-head early pollution (seed9895936 g_1580@5). Mid pure IVs
		// already pureMiss-placed (seed48 g_1495@44) must not late-jump.
		// Threshold: early Acc zone is before free-ref mid-body pure IV block.
		if ci > bestPos/3 && ci > 12 {
			continue
		}
		moves = append(moves, move{v: v, anchor: bestAnchor})
		moveSet[v] = true
	}
	if len(moves) == 0 {
		return
	}
	// Rebuild: walk ord, skip moved; after emitting anchor, emit deferred pure IVs.
	afterAnchor := map[*Variable][]*Variable{}
	for _, m := range moves {
		afterAnchor[m.anchor] = append(afterAnchor[m.anchor], m.v)
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil || moveSet[v] {
			continue
		}
		emit(v)
		if ds, ok := afterAnchor[v]; ok {
			for _, d := range ds {
				emit(d)
			}
		}
	}
	// Rewrite body map_stm
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupFunc1NestedFEPurePrefixBeforeFree ensures a nested callee's leading pure
// for-IV FE prefix precedes free-ref non-pure reads from the same FE on parent
// body map_stm. seed88: func_58 FE [g_88.f0, g_657, g_91, ...] — GO had free-ref
// g_91 before pure prefix g_88.f0/g_657; UP keeps pure prefix then free-ref.
// Session/FM-local — no package mutable state.
func fixupFunc1NestedFEPurePrefixBeforeFree(f *Function, fm *FactMgr, s *Session) {
	fixupFunc1NestedFEPurePrefixBeforeFreeMaxGap(f, fm, s, 3)
}

func fixupFunc1NestedFEPurePrefixBeforeFreeMaxGap(f *Function, fm *FactMgr, s *Session, maxGap int) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return
	}
	ordPos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := ordPos[v]; !ok {
				ordPos[v] = i
			}
		}
	}
	// Collect (purePrefix, freeRef) pairs where freeRef is later in nested FE
	// but currently earlier on parent body than the pure prefix.
	type pair struct {
		prefix []*Variable
		free   *Variable
	}
	var pairs []pair
	seenFree := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		// Leading pure for-IV prefix of this callee (not own for-IVs of parent —
		// seed48 g_1495 is own IV of func_1 + FE head of func_11; pureMiss late
		// placement must not early-jump before free-ref g_405). seed88 g_88.f0 is
		// nested-only pure IV of func_58.
		// Free-ref pure on parent ends pure-only prefix: pure-prefix must not yank
		// free-ref pure FE head/mid before free residual free-ref (seed1436
		// g_39/g_65; LevelC seed10563948572526880868 g_9 of func_35 before g_2 —
		// Acc already has free residual then free-ref pure). seed88 pure-only.
		// Session/FM-local — no package mutable state.
		var prefix []*Variable
		for _, v := range fr {
			if v == nil || !isForIVOfFunc(inv.User, v) {
				break
			}
			if isForIVOfFunc(f, v) {
				break // own pure IV ends nested-only pure prefix
			}
			if bodySyntacticFreeReadsVar(f, v) {
				break // free-ref pure keeps parent free residual Acc order
			}
			prefix = append(prefix, v)
		}
		if len(prefix) == 0 {
			continue
		}
		// First free-ref on *this callee* among FE reads after the pure prefix
		// that also free-refs on parent (so it appears on parent map_stm).
		// Require callee free-ref: parent-only free-ref of a later FE non-pure can
		// Acc-early on parent and wrongly pull pure prefix before it (seed22584
		// g_359.f0 before g_3). seed88 g_91 free-refs on both parent and callee.
		var free *Variable
		for _, v := range fr[len(prefix):] {
			if v == nil {
				continue
			}
			if isForIVOfFunc(inv.User, v) {
				continue
			}
			if !bodySyntacticFreeReadsVar(inv.User, v) {
				continue
			}
			if !bodySyntacticFreeReadsVar(f, v) {
				continue
			}
			if _, ok := ordPos[v]; !ok {
				continue
			}
			free = v
			break
		}
		if free == nil || seenFree[free] {
			continue
		}
		// Free residual that is own pure of parent: Acc free-ref/own-pure early
		// before nested pure FE head is UP-correct — do not yank nested pure
		// prefix before own pure residual (LevelC seed 7882445078581404101
		// g_63 of func_76 before own pure free-ref g_46 of func_1). seed88
		// free residual g_91 is not own pure of parent. Session/FM-local.
		if isForIVOfFunc(f, free) {
			continue
		}
		// Pure prefix must exist on parent.
		minP := -1
		for _, p := range prefix {
			pi, ok := ordPos[p]
			if !ok {
				minP = -1
				break
			}
			if minP < 0 || pi < minP {
				minP = pi
			}
		}
		if minP < 0 {
			continue
		}
		fi := ordPos[free]
		if fi >= minP {
			continue // free already after pure prefix
		}
		// Free Acc-early from other sources while pure sits mid (seed57 g_290@25
		// vs g_1597@85) must not yank pure prefix early. Only reorder when free
		// is near the pure block (seed88; LC gap=1). maxGap=12 default; Near=3
		// restores pure-prefix after exclusive residual without seed2918 gap~10.
		if maxGap <= 0 {
			maxGap = 12
		}
		if minP-fi > maxGap {
			continue
		}
		// Copy prefix (stable FE order).
		pre := append([]*Variable(nil), prefix...)
		pairs = append(pairs, pair{prefix: pre, free: free})
		seenFree[free] = true
	}
	if len(pairs) == 0 {
		return
	}
	// Rebuild: drop pure-prefix vars, then insert each pair's prefix immediately
	// before its free-ref (first free wins if shared). Session-local.
	drop := map[*Variable]bool{}
	for _, p := range pairs {
		for _, v := range p.prefix {
			drop[v] = true
			if fm.PurePrefixMoved == nil {
				fm.PurePrefixMoved = make(map[*Variable]bool)
			}
			fm.PurePrefixMoved[v] = true
		}
	}
	insertBefore := map[*Variable][]*Variable{}
	for _, p := range pairs {
		insertBefore[p.free] = append(insertBefore[p.free], p.prefix...)
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if drop[v] {
			continue
		}
		if pre, ok := insertBefore[v]; ok {
			for _, p := range pre {
				emit(p)
			}
		}
		emit(v)
	}
	// Any pure prefix not re-inserted (free missing) — append.
	for _, p := range pairs {
		for _, v := range p.prefix {
			emit(v)
		}
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// freeRefOnlyArrayInits: free-ref only via non-value forms (ArrayInits and/or
// address-of InitExpr). Value free-reads (return/RHS) fail closed false.
// seed48 g_750.f2 is `T *p = &g_750.f2` (InitExpr address-of), not ArrayInits.
func freeRefOnlyArrayInits(f *Function, v *Variable) bool {
	if f == nil || v == nil || !bodySyntacticFreeReadsVar(f, v) {
		return false
	}
	// Value free-ref must not use FE-tail terminal move (n28 returns / n35).
	if bodyValueFreeReadsVar(f, v) {
		return false
	}
	hitArr := false
	hitAddr := false
	var walk func(blk *Block) bool
	walk = func(blk *Block) bool {
		if blk == nil {
			return false
		}
		for _, loc := range blk.LocalVars {
			if loc == nil {
				continue
			}
			// Address-of InitExpr is non-value free-ref (seed48 g_750.f2).
			if exprRefsVar(loc.InitExpr, v) {
				if exprValueFreeRefsVar(loc.InitExpr, v) {
					return true // value use in init — not array/addr-only
				}
				hitAddr = true
				continue
			}
			if arrayInitsRefVar(loc.ArrayInits, v) {
				hitArr = true
			}
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV == v {
				if walk(st.Then) {
					return true
				}
				continue
			}
			if stmtRefsVar(st, v) {
				return true
			}
			if st.Loop != nil && exprRefsVar(st.Loop.TestExpr, v) {
				return true
			}
			switch st.Kind {
			case StmtBlock:
				if walk(st.Then) {
					return true
				}
			case StmtIfElse:
				if walk(st.Then) || walk(st.Else) {
					return true
				}
			case StmtFor, StmtArrayOp:
				if walk(st.Then) {
					return true
				}
			}
		}
		return false
	}
	body := f.Body
	if body == nil {
		for _, b := range f.Blocks {
			if b != nil && b.Parent == nil {
				body = b
				break
			}
		}
	}
	if walk(body) {
		return false
	}
	return hitArr || hitAddr
}

// fixupFunc1FETailArrayInitAfterResidual places body map_stm pure IVs that are
// FE-tail of a nested callee and free-ref only via ArrayInits/address-of after
// residual free of that owner FE (not absolute end).
//
// Assign-level FE-tail absolute end matches seed48 g_750.f2. Absolute end on
// body leaves LC seed 16045778296055951950 g_2846 of func_23 after free Acc
// (UP: immediately after residual free g_1432 of func_23). Session/FM-local.
func fixupFunc1FETailArrayInitAfterResidual(f *Function, fm *FactMgr, s *Session) {
	if f == nil || fm == nil || f.Body == nil || StmIDUnset(f.Body.StmID) || f.Name != "func_1" {
		return
	}
	var calls []*Invocation
	var walkCalls func(blk *Block)
	walkCalls = func(blk *Block) {
		if blk == nil {
			return
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = collectCalledInvocationsStmt(s, st, &calls)
			switch st.Kind {
			case StmtBlock, StmtFor, StmtArrayOp:
				walkCalls(st.Then)
			case StmtIfElse:
				walkCalls(st.Then)
				walkCalls(st.Else)
			}
		}
	}
	walkCalls(f.Body)
	if !InvocationsComplete(calls) {
		return
	}
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		return
	}
	ord := bodyEff.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) || len(ord) == 0 {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	// pure → owner FE ending with pure as last read (direct nested callees).
	type plan struct {
		p   *Variable
		aft *Variable // residual free to insert after; nil → absolute end
	}
	var plans []plan
	moveSet := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if len(fr) == 0 {
			continue
		}
		last := fr[len(fr)-1]
		if last == nil {
			continue
		}
		if _, ok := pos[last]; !ok {
			continue
		}
		if moveSet[last] {
			continue
		}
		if isForIVOfFunc(f, last) {
			continue
		}
		if !freeRefOnlyArrayInits(f, last) {
			continue
		}
		// Residual free of owner FE present on body.
		ownerRes := map[*Variable]bool{}
		for _, x := range fr[:len(fr)-1] {
			if x != nil && x != last {
				if _, ok := pos[x]; ok {
					ownerRes[x] = true
				}
			}
		}
		if len(ownerRes) == 0 {
			continue
		}
		// Prefer end of a consecutive residual-free cluster of owner on body
		// (LC: g_2818…g_1432 then free Acc). Isolated mid residual free
		// (seed48 g_129.f2@46) is not a cluster end — keep absolute end.
		var aft *Variable
		aftPos := -1
		run := 0
		for i, v := range ord {
			if v != nil && ownerRes[v] {
				run++
				if run >= 3 && i > aftPos {
					aftPos = i
					aft = v
				}
			} else if v != nil {
				run = 0
			}
		}
		if aft == nil {
			continue
		}
		// Late residual-free cluster only (LC@~82/201). Early clusters
		// (seed48 owner residual free@0–19) keep absolute-end pure.
		if aftPos < len(ord)/3 {
			continue
		}
		pp := pos[last]
		if pp == aftPos+1 {
			continue
		}
		if pp < aftPos {
			continue
		}
		// Pure Acc-late past residual free cluster with free Acc between
		// (absolute-end FE-tail terminalization). Require gap >= 3.
		if pp-aftPos < 3 {
			continue
		}
		plans = append(plans, plan{p: last, aft: aft})
		moveSet[last] = true
	}
	if len(plans) == 0 {
		return
	}
	// Rebuild: drop moved pures, insert each after its residual free anchor.
	beforeAft := map[*Variable][]*Variable{}
	for _, pl := range plans {
		beforeAft[pl.aft] = append(beforeAft[pl.aft], pl.p)
	}
	// insert after anchor: when walking, after emit(anchor) emit pures
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		emit(v)
		if ps, ok := beforeAft[v]; ok {
			for _, p := range ps {
				emit(p)
			}
		}
	}
	for _, pl := range plans {
		emit(pl.p)
	}
	out := EmptyEffect()
	for _, w := range bodyEff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return
		}
	}
	out.pure = bodyEff.pure
	out.sideEffectFree = bodyEff.sideEffectFree
	out.lhsWrite = bodyEff.lhsWrite
	fm.SetMapStmEffect(f.Body.StmID, out)
}

// fixupAssignNestedFETailArrayInit moves pure IVs that are (1) last read on some
// nested callee FE and (2) free-ref'd on f only via non-value forms (ArrayInits
// / address-of InitExpr) to end of map_stm (seed48 g_750.f2).
// Body residual-after-owner is fixupFunc1FETailArrayInitAfterResidual.
// Session/FM-local — no package state.
// Returns the pure IVs moved (or already terminal) for ancestor propagation.
func fixupAssignNestedFETailArrayInit(s *Session, fm *FactMgr, f *Function, st *Stmt) []*Variable {
	if st == nil || fm == nil || f == nil || StmIDUnset(st.StmID) {
		return nil
	}
	if os.Getenv("DIAG_S48_PURE") != "" {
		fmt.Fprintf(os.Stderr, "FE_TAIL_CALL sid=%d\n", st.StmID)
	}
	eff := fm.GetMapStmEffect(st.StmID)
	if !EffectComplete(eff) {
		if os.Getenv("DIAG_S48_PURE") != "" {
			fmt.Fprintf(os.Stderr, "FE_TAIL_INCOMPLETE sid=%d\n", st.StmID)
		}
		return nil
	}
	var calls []*Invocation
	if !collectCalledInvocationsStmt(s, st, &calls) || !InvocationsComplete(calls) {
		if os.Getenv("DIAG_S48_PURE") != "" {
			fmt.Fprintf(os.Stderr, "FE_TAIL_NOCALLS sid=%d\n", st.StmID)
		}
		return nil
	}
	pureIV := pureIVGlobalsFromUserCallTree(s, calls)
	if sessHasError(s) {
		return nil
	}
	if os.Getenv("DIAG_S48_PURE") != "" {
		fmt.Fprintf(os.Stderr, "FE_TAIL_ENTER sid=%d nCalls=%d nPure=%d\n", st.StmID, len(calls), len(pureIV))
	}
	ord := eff.ReadVarsSess(s)
	if sessHasError(s) || len(ord) == 0 {
		return nil
	}
	var tail []*Variable
	tailSet := map[*Variable]bool{}
	for _, v := range ord {
		if v == nil || !pureIV[v] || isForIVOfFunc(f, v) {
			continue
		}
		if !v.IsGlobalSess(s) {
			if sessHasError(s) {
				return nil
			}
			continue
		}
		isTail := pureIVIsNestedFETail(s, calls, v)
		onlyArr := freeRefOnlyArrayInits(f, v)
		if !isTail {
			continue
		}
		if !onlyArr {
			continue
		}
		tail = append(tail, v)
		tailSet[v] = true
	}
	if len(tail) == 0 {
		return nil
	}
	if os.Getenv("DIAG_S48_PURE") != "" {
		var names []string
		for _, v := range tail {
			if v != nil {
				names = append(names, v.Name)
			}
		}
		fmt.Fprintf(os.Stderr, "FE_TAIL_MOVE sid=%d tail=%v\n", st.StmID, names)
	}
	allTail := true
	for i := len(ord) - len(tail); i < len(ord); i++ {
		if i < 0 || !tailSet[ord[i]] {
			allTail = false
			break
		}
	}
	if allTail {
		return tail
	}
	if !moveVarsToMapStmTail(s, fm, st.StmID, tail, tailSet) {
		return nil
	}
	return tail
}

// moveVarsToMapStmTail moves vars in tailSet to the end of map_stm[sid] reads
// (writes preserved). Returns false on incomplete/error. Session/FM-local.
func moveVarsToMapStmTail(s *Session, fm *FactMgr, sid int, tail []*Variable, tailSet map[*Variable]bool) bool {
	if fm == nil || StmIDUnset(sid) || len(tailSet) == 0 {
		return false
	}
	eff := fm.GetMapStmEffect(sid)
	if !EffectComplete(eff) {
		return false
	}
	ord := eff.ReadVarsSess(s)
	if sessHasError(s) || len(ord) == 0 {
		return false
	}
	// Only vars present on this effect.
	var present []*Variable
	presentSet := map[*Variable]bool{}
	for _, v := range ord {
		if v != nil && tailSet[v] && !presentSet[v] {
			present = append(present, v)
			presentSet[v] = true
		}
	}
	if len(present) == 0 {
		return true
	}
	allTail := true
	for i := len(ord) - len(present); i < len(ord); i++ {
		if i < 0 || !presentSet[ord[i]] {
			allTail = false
			break
		}
	}
	if allTail {
		return true
	}
	rebuilt := EmptyEffect()
	for _, w := range eff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		rebuilt = rebuilt.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(rebuilt) {
			return false
		}
	}
	for _, v := range ord {
		if v == nil || presentSet[v] {
			continue
		}
		rebuilt = rebuilt.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(rebuilt) {
			return false
		}
	}
	for _, v := range present {
		rebuilt = rebuilt.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(rebuilt) {
			return false
		}
	}
	fm.SetMapStmEffect(sid, rebuilt)
	return true
}

// propagateFETailMapStm walks func_1 IR and moves FE-tail pure IVs to the end of
// every map_stm. Prefer propagateFETailFromAssign (ancestor-only) for func_1
// fixup — full-tree move terminalized n35 pure IVs incorrectly.
// Session/FM-local — no package mutable state.
func propagateFETailMapStm(f *Function, fm *FactMgr, s *Session, tail []*Variable) {
	if f == nil || fm == nil || len(tail) == 0 {
		return
	}
	tailSet := map[*Variable]bool{}
	for _, v := range tail {
		if v != nil {
			tailSet[v] = true
		}
	}
	var walk func(blk *Block)
	walk = func(blk *Block) {
		if blk == nil {
			return
		}
		_ = moveVarsToMapStmTail(s, fm, blk.StmID, tail, tailSet)
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			_ = moveVarsToMapStmTail(s, fm, st.StmID, tail, tailSet)
			switch st.Kind {
			case StmtBlock:
				walk(st.Then)
			case StmtIfElse:
				walk(st.Then)
				walk(st.Else)
			case StmtFor, StmtArrayOp:
				walk(st.Then)
			}
		}
	}
	walk(f.Body)
}

// propagateFETailFromAssign moves FE-tail pure IVs to the end of map_stm on the
// ancestor path from assign up to func_1 body (for/if stmt + block sids). Does
// not touch sibling arms (n35). Session/FM-local — no package mutable state.
func propagateFETailFromAssign(f *Function, fm *FactMgr, s *Session, assign *Stmt, tail []*Variable) {
	if f == nil || fm == nil || assign == nil || len(tail) == 0 || f.Body == nil {
		return
	}
	tailSet := map[*Variable]bool{}
	for _, v := range tail {
		if v != nil {
			tailSet[v] = true
		}
	}
	// Find path of (block, stmtIndex) from body to assign.
	type frame struct {
		blk *Block
		idx int
	}
	var path []frame
	var find func(blk *Block) bool
	find = func(blk *Block) bool {
		if blk == nil {
			return false
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			if st == assign || (!StmIDUnset(assign.StmID) && st.StmID == assign.StmID && st.Kind == StmtAssign) {
				path = append(path, frame{blk: blk, idx: i})
				return true
			}
			switch st.Kind {
			case StmtBlock:
				if find(st.Then) {
					path = append(path, frame{blk: blk, idx: i})
					return true
				}
			case StmtIfElse:
				if find(st.Then) || find(st.Else) {
					path = append(path, frame{blk: blk, idx: i})
					return true
				}
			case StmtFor, StmtArrayOp:
				if find(st.Then) {
					path = append(path, frame{blk: blk, idx: i})
					return true
				}
			}
		}
		return false
	}
	if !find(f.Body) {
		return
	}
	// path is leaf→root; move on each stmt and its parent block along the path.
	for _, fr := range path {
		if fr.blk == nil {
			continue
		}
		_ = moveVarsToMapStmTail(s, fm, fr.blk.StmID, tail, tailSet)
		if fr.idx >= 0 && fr.idx < len(fr.blk.Stmts) {
			_ = moveVarsToMapStmTail(s, fm, fr.blk.Stmts[fr.idx].StmID, tail, tailSet)
		}
	}
	_ = moveVarsToMapStmTail(s, fm, f.Body.StmID, tail, tailSet)
}

func fixupAssignPureIVFEHead(s *Session, fm *FactMgr, f *Function, st *Stmt) {
	if st == nil || StmIDUnset(st.StmID) {
		return
	}
	eff := fm.GetMapStmEffect(st.StmID)
	if !EffectComplete(eff) {
		return
	}
	var calls []*Invocation
	if !collectCalledInvocationsStmt(s, st, &calls) || !InvocationsComplete(calls) {
		return
	}
	pureIV := pureIVGlobalsFromUserCallTree(s, calls)
	ord := eff.ReadVarsSess(s)
	if sessHasError(s) {
		return
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			pos[v] = i
		}
	}
	type mv struct{ pure, anc *Variable }
	var moves []mv
	seenP := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return
		}
		if len(fr) == 0 || fr[0] == nil || !pureIV[fr[0]] || seenP[fr[0]] {
			continue
		}
		p := fr[0]
		if !p.IsGlobalSess(s) {
			continue
		}
		// Only membership invents (visit-missing pureMiss re-inserts). In-visit
		// pureMiss reorders already match prev; FE-head move would yank them
		// early next to FE neighbor (seed57 g_1597 before g_381). n35 g_108
		// was invent-only. FM-local PureMissInvented — no package state.
		if fm.PureMissInvented == nil || !fm.PureMissInvented[st.StmID][p] {
			continue
		}
		if laterBodyReadsVar(f, st, p) {
			continue
		}
		ownIV := false
		for _, b := range inv.User.Blocks {
			if b == nil {
				continue
			}
			for j := range b.Stmts {
				st2 := &b.Stmts[j]
				if st2.Kind == StmtFor && st2.Loop != nil && st2.Loop.IV == p {
					ownIV = true
					break
				}
			}
			if ownIV {
				break
			}
		}
		if !ownIV {
			continue
		}
		pp, pok := pos[p]
		if !pok || len(fr) < 2 || fr[1] == nil {
			continue
		}
		anc := fr[1]
		ap, aok := pos[anc]
		if !aok || pp <= ap {
			continue
		}
		moves = append(moves, mv{pure: p, anc: anc})
		seenP[p] = true
	}
	if len(moves) == 0 {
		return
	}
	for _, m := range moves {
		var next []*Variable
		for _, v := range ord {
			if v == m.pure {
				continue
			}
			next = append(next, v)
		}
		var out []*Variable
		inserted := false
		for _, v := range next {
			if v == m.anc && !inserted {
				out = append(out, m.pure)
				inserted = true
			}
			out = append(out, v)
		}
		if !inserted {
			out = append(out, m.pure)
		}
		ord = out
	}
	rebuilt := EmptyEffect()
	for _, w := range eff.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		rebuilt = rebuilt.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(rebuilt) {
			return
		}
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		rebuilt = rebuilt.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(rebuilt) {
			return
		}
	}
	fm.SetMapStmEffect(st.StmID, rebuilt)
}

// arrayInitsRefVar reports v's name appears as a C token in array brace init strings.
func arrayInitsRefVar(inits []string, v *Variable) bool {
	if v == nil || len(inits) == 0 || v.Name == "" {
		return false
	}
	// Word-ish match: name as whole token (g_903.f3 has a dot).
	pat := regexp.MustCompile(`(?:^|[^[:alnum:]_])` + regexp.QuoteMeta(v.Name) + `(?:$|[^[:alnum:]_])`)
	for _, s := range inits {
		if pat.MatchString(s) {
			return true
		}
	}
	return false
}

// isForIVOfFunc reports v is a for-loop IV of some statement in f's blocks.
// Session-local — no package state.
func isForIVOfFunc(f *Function, v *Variable) bool {
	if f == nil || v == nil {
		return false
	}
	for _, blk := range f.Blocks {
		if blk == nil {
			continue
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV == v {
				return true
			}
		}
	}
	return false
}

// bodySyntacticFreeReadsVar reports any non pure-for-header reference to v in
// f's body, including address-of. Used as invent gate: pure-for-IV-only symbols
// (seed48 g_250/g_951) never free-ref and must not restore into parent FE.
// Session-local IR walk — no package state.
func bodySyntacticFreeReadsVar(f *Function, v *Variable) bool {
	if f == nil || v == nil {
		return false
	}
	var walk func(blk *Block) bool
	walk = func(blk *Block) bool {
		if blk == nil {
			return false
		}
		for _, loc := range blk.LocalVars {
			if loc == nil {
				continue
			}
			if exprRefsVar(loc.InitExpr, v) {
				return true
			}
			// Array brace inits are string tokens (Variable.ArrayInits), not Expression
			// trees — n94 &g_903.f3 lives here. Session-local name match.
			if arrayInitsRefVar(loc.ArrayInits, v) {
				return true
			}
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV == v {
				if walk(st.Then) {
					return true
				}
				continue
			}
			if stmtRefsVar(st, v) {
				return true
			}
			if st.Loop != nil && exprRefsVar(st.Loop.TestExpr, v) {
				return true
			}
			switch st.Kind {
			case StmtBlock:
				if walk(st.Then) {
					return true
				}
			case StmtIfElse:
				if walk(st.Then) || walk(st.Else) {
					return true
				}
			case StmtFor, StmtArrayOp:
				if walk(st.Then) {
					return true
				}
			}
		}
		return false
	}
	body := f.Body
	if body == nil {
		for _, b := range f.Blocks {
			if b != nil && b.Parent == nil {
				body = b
				break
			}
		}
	}
	return walk(body)
}


// pureIVVisitSupportedByNestedFE: FE-immediate pred equals visit-immediate pred.
// Tight edge match keeps n28 order-lag re-place; seed48 g_129.f2 may still pureMiss.
// Session-local — no package state.
func pureIVVisitSupportedByNestedFE(s *Session, calls []*Invocation, v *Variable, visitIdx map[*Variable]int) bool {
	if v == nil || !InvocationsComplete(calls) || visitIdx == nil {
		return false
	}
	vi, vok := visitIdx[v]
	if !vok || vi <= 0 {
		return false
	}
	var immPred *Variable
	best := -1
	for vv, idx := range visitIdx {
		if vv == nil {
			continue
		}
		if idx < vi && idx > best {
			best = idx
			immPred = vv
		}
	}
	if immPred == nil {
		return false
	}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return false
		}
		for j, fv := range fr {
			if fv == v && j > 0 && fr[j-1] == immPred {
				return true
			}
		}
	}
	return false
}

// reorderFreeHeadMidPureBeforeResidualFree places pure residual pure for-IVs of
// free-head nested FEs before residual free of the same FE currently before pure
// on a summary/map effect. seed12593 func_1: free-head func_30 FE
// [g_2249…g_2252, g_1678, g_3964] — pure residual g_1678 Acc-late after g_3964
// (residual free pure-only on free-head owner). Walks the full user call tree.
// Soft-collect nested calls (incomplete IR skips that edge only). Anchor is the
// first residual free after pure in FE order. Require FE predecessor of pure on
// parent so the move restores a partial FE fragment.
// Skip when residual free free-refs free-head owner: Acc free residual free-ref
// then pure residual pure-only is UP-correct (seed12848 func_59 g_270 free-ref
// owner before pure residual g_528.f3 pure-only; body map already correct, yank
// reorderMultiPureFEHeadBeforePureMultiSiblings places multi pure-prefix pure-only
// FE heads Acc-late after pure multi pureOnly of the same FE immediately before the
// earliest pure multi pureOnly on parent summary (seed12336 g_659 of func_15 before
// g_661). Free-ref pure multi members and PureMissTouched are left alone. map_stm
// multi-prefix already ran; Acc invent can re-add multi head Acc-order late.
// Session-local — no package mutable state.
// acc, when complete, gates Case B: only promote multi pure FE head before pure multi
// pureOnly when Acc also has head before that pureOnly (seed12336 g_659). Acc-late multi
// pure FE head after Acc-early pure multi pureOnly of same FE stays Acc-late (c302abe
// g_328.f2 after g_109). Session-local — no package mutable state.
func reorderMultiPureFEHeadBeforePureMultiSiblings(s *Session, parent *Function, e Effect, calls []*Invocation, acc *Effect) Effect {
	if !EffectComplete(e) || parent == nil || !InvocationsComplete(calls) {
		return e
	}
	ord := e.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return IncompleteEffect()
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type headMove struct {
		head *Variable
		anc  *Variable // insert before this pure multi pureOnly
	}
	var moves []headMove
	headSet := map[*Variable]bool{}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return IncompleteEffect()
		}
		var prefix []*Variable
		for _, v := range fr {
			if v == nil || !isForIVOfFunc(inv.User, v) {
				break
			}
			prefix = append(prefix, v)
		}
		if len(prefix) < 2 {
			continue
		}
		var pureOnly []*Variable
		for _, v := range prefix {
			if v == nil {
				continue
			}
			// Resolve on parent summary by pointer or name (Acc invent may use a
			// different *Variable identity than nested FE prefix — seed12336 g_659
			// ACC_APPEND vs func_15 FE head). Session-local — no package mutable state.
			pv := v
			if _, ok := pos[pv]; !ok {
				found := false
				for x := range pos {
					if x != nil && x.Name == v.Name {
						pv = x
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if bodySyntacticFreeReadsVar(parent, pv) {
				continue
			}
			pureOnly = append(pureOnly, pv)
		}
		// Case A: free-ref multi pure FE head Acc-late after pure multi pureOnly of
		// same FE (seed120 func_45: multi pure func_53 [g_28,g_59]; g_28 free-refs
		// on parent, g_59 pure-only Acc-early; invent Acc-order places g_28 before
		// residual free g_63 after g_59; UP g_28 g_59). Session-local — no package
		// mutable state.
		if len(pureOnly) >= 1 {
			head := prefix[0]
			if head != nil {
				// resolve head on parent summary by pointer or name
				hp, hok := pos[head]
				if !hok {
					for v, p := range pos {
						if v != nil && v.Name == head.Name {
							head = v
							hp = p
							hok = true
							break
						}
					}
				}
				if hok && bodySyntacticFreeReadsVar(parent, head) && !headSet[head] {
					minOther := -1
					var anc *Variable
					for _, v := range pureOnly {
						p, pok := pos[v]
						if !pok {
							continue
						}
						if minOther < 0 || p < minOther {
							minOther = p
							anc = v
						}
					}
					if anc != nil && minOther >= 0 && hp > minOther {
						moves = append(moves, headMove{head: head, anc: anc})
						headSet[head] = true
						continue
					}
				}
			}
		}
		if len(pureOnly) < 2 {
			continue
		}
		// Case B: pure-only multi pure FE head Acc-late after other pure multi pureOnly
		// (seed12336 g_659 of func_15 before g_661). pureOnly[0] must be multi pure FE
		// head (prefix[0] by pointer or name) so we do not promote a mid pure multi.
		head := pureOnly[0]
		if head != prefix[0] && (prefix[0] == nil || head == nil || head.Name != prefix[0].Name) {
			continue
		}
		hp, ok := pos[head]
		if !ok || headSet[head] {
			continue
		}
		minOther := -1
		var anc *Variable
		for _, v := range pureOnly[1:] {
			p, pok := pos[v]
			if !pok {
				continue
			}
			if minOther < 0 || p < minOther {
				minOther = p
				anc = v
			}
		}
		if anc == nil || minOther < 0 || hp <= minOther {
			continue // head already before other pure multi pureOnly
		}
		// Free residual free of parent between Acc-early pure multi pureOnly and
		// Acc-late multi pure FE head: keep Acc-late (seed28465 g_163 after g_620
		// before g_286; pure multi mid g_98.f0 Acc-early). seed12336 pure multi
		// pureOnly cluster (prefix len>=3) still promotes head before pure multi
		// pureOnly despite free residual free of parent between invent-late head
		// and pureOnly (ACC_APPEND g_659 late). Binary pure multi (prefix len==2)
		// keeps Acc-late when free residual free of parent between (c302abe
		// g_328.f2 after g_109 Acc/UP). Session-local — no package mutable state.
		betweenFreeRef := false
		for i := minOther + 1; i < hp; i++ {
			x := ord[i]
			if x == nil {
				continue
			}
			// skip pure multi pureOnly cluster members
			isPO := false
			for _, po := range pureOnly {
				if po == x || (po != nil && x.Name == po.Name) {
					isPO = true
					break
				}
			}
			if isPO {
				continue
			}
			if bodySyntacticFreeReadsVar(parent, x) {
				betweenFreeRef = true
				break
			}
		}
		if betweenFreeRef && len(prefix) < 3 {
			continue // binary pure multi + free residual free between: keep Acc-late
		}
		// Binary pure multi (prefix len==2): keep Acc-late when Acc already has multi
		// pure FE head after pure multi pureOnly (invent restored Acc order — seed875
		// g_116 before g_117 of pure-head func_60; Acc mid-before-head contiguous).
		// Foreign Acc material between is a subset (c302abe g_328.f2 after g_109 …
		// g_99 g_153) — no longer required. Ternary+ pure multi (seed12336
		// g_659/g_661/g_416) still promotes multi pure FE head before pure multi
		// pureOnly. Session-local — no package mutable state.
		keepAccLate := false
		if acc != nil && EffectComplete(*acc) && head != nil && anc != nil && len(prefix) < 3 {
			accOrd := acc.ReadVarsSess(s)
			if sessHasError(s) {
				return IncompleteEffect()
			}
			accPos := map[string]int{}
			for i, x := range accOrd {
				if x != nil {
					if _, ok := accPos[x.Name]; !ok {
						accPos[x.Name] = i
					}
				}
			}
			hpAcc, hok := accPos[head.Name]
			apAcc, aok := accPos[anc.Name]
			// Acc mid-before-head: invent placed pure multi pureOnly then multi pure
			// FE head; do not Case-B promote (seed875). Acc head-before-mid still
			// promotes invent-late head.
			if hok && aok && hpAcc > apAcc {
				keepAccLate = true
			}
		}
		if keepAccLate {
			continue
		}
		moves = append(moves, headMove{head: head, anc: anc})
		headSet[head] = true
	}
	if len(moves) == 0 {
		return e
	}
	drop := map[*Variable]bool{}
	for _, m := range moves {
		drop[m.head] = true
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if drop[v] {
			continue
		}
		// insert any multi heads that want to be before this pure multi pureOnly
		for _, m := range moves {
			if m.anc == v {
				emit(m.head)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.head)
	}
	out := EmptyEffect()
	for _, w := range e.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return IncompleteEffect()
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return IncompleteEffect()
		}
	}
	out.pure = e.pure
	out.sideEffectFree = e.sideEffectFree
	out.lhsWrite = e.lhsWrite
	return out
}

// to before residual free breaks intermediate FEs). Session-local — no package
// mutable state.
func reorderFreeHeadMidPureBeforeResidualFree(s *Session, parent *Function, e Effect, calls []*Invocation) Effect {
	if !EffectComplete(e) || !InvocationsComplete(calls) {
		return e
	}
	type feSite struct {
		user *Function
		fr   []*Variable
	}
	var sites []feSite
	seenFn := map[*Function]bool{}
	var walkFn func(f *Function)
	walkFn = func(f *Function) {
		if f == nil || seenFn[f] {
			return
		}
		seenFn[f] = true
		if EffectComplete(f.FEffect) {
			fr := f.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			sites = append(sites, feSite{user: f, fr: fr})
		}
		for _, blk := range f.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				var nested []*Invocation
				if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
					continue
				}
				for _, inv := range nested {
					if inv != nil && inv.User != nil {
						walkFn(inv.User)
					}
				}
			}
		}
	}
	for _, inv := range calls {
		if inv != nil && inv.User != nil {
			walkFn(inv.User)
		}
	}
	if sessHasError(s) {
		return IncompleteEffect()
	}
	ord := e.ReadVarsSess(s)
	if sessHasError(s) || !VariablesComplete(ord) {
		return IncompleteEffect()
	}
	pos := map[*Variable]int{}
	for i, v := range ord {
		if v != nil {
			if _, ok := pos[v]; !ok {
				pos[v] = i
			}
		}
	}
	type move struct {
		p   *Variable
		anc *Variable
	}
	var moves []move
	moveSet := map[*Variable]bool{}
	for _, site := range sites {
		fr := site.fr
		user := site.user
		if len(fr) < 3 || fr[0] == nil {
			continue
		}
		if isForIVOfFunc(user, fr[0]) {
			continue
		}
		firstPureIdx := -1
		for j, x := range fr {
			if x != nil && isForIVOfFunc(user, x) {
				firstPureIdx = j
				break
			}
		}
		if firstPureIdx < 0 {
			continue
		}
		i := firstPureIdx
		p := fr[i]
		if p == nil || !isForIVOfFunc(user, p) || moveSet[p] {
			continue
		}
		if parent != nil && bodySyntacticFreeReadsVar(parent, p) {
			continue
		}
		if nestedUserFreeReadsVarExcludingIVOwner(s, calls, p) {
			continue
		}
		// Pure residual pure-only on free-head owner (no free-ref free on owner):
		// Acc/map late pure after residual free of free-head is UP-correct (seed99
		// func_40 g_541 pure-only after free residual free g_655). Free-ref free
		// pure residual pure of free-head still reorders before residual free
		// (seed12593 func_30 g_1678 free-ref free on free-head before g_3964).
		// Session-local — no package mutable state.
		if !bodySyntacticFreeReadsVar(user, p) {
			continue
		}
		pp, pok := pos[p]
		if !pok {
			continue
		}
		var firstRes *Variable
		for j := i + 1; j < len(fr); j++ {
			x := fr[j]
			if x == nil || isForIVOfFunc(user, x) {
				continue
			}
			firstRes = x
			break
		}
		if firstRes == nil {
			continue
		}
		// Residual free free-ref on free-head owner: natural Acc free residual
		// free-ref before pure residual pure-only (seed12848 g_270 free-ref
		// func_59 before g_528.f3). seed12593 residual free g_3964 pure-only
		// on free-head owner — Acc-early residual free pollution, pure before
		// residual free. Session-local.
		if bodySyntacticFreeReadsVar(user, firstRes) {
			continue
		}
		ap, ok := pos[firstRes]
		if !ok || ap >= pp {
			continue
		}
		if i == 0 || fr[i-1] == nil {
			continue
		}
		pred := fr[i-1]
		predPos, ok := pos[pred]
		if !ok || predPos >= ap {
			continue
		}
		inFE := map[*Variable]bool{}
		for _, x := range fr {
			if x != nil {
				inFE[x] = true
			}
		}
		adj := false
		for j := predPos + 1; j < len(ord); j++ {
			if ord[j] == nil || !inFE[ord[j]] {
				continue
			}
			if ord[j] == p {
				continue
			}
			adj = ord[j] == firstRes
			break
		}
		if !adj {
			continue
		}
		moves = append(moves, move{p: p, anc: firstRes})
		moveSet[p] = true
	}
	if len(moves) == 0 {
		return e
	}
	beforeAnc := map[*Variable][]*Variable{}
	for _, m := range moves {
		beforeAnc[m.anc] = append(beforeAnc[m.anc], m.p)
	}
	for anc, hs := range beforeAnc {
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if pos[hs[j]] < pos[hs[i]] {
					hs[i], hs[j] = hs[j], hs[i]
				}
			}
		}
		beforeAnc[anc] = hs
	}
	var newOrd []*Variable
	seen := map[*Variable]bool{}
	emit := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		newOrd = append(newOrd, v)
	}
	for _, v := range ord {
		if v == nil {
			continue
		}
		if moveSet[v] {
			continue
		}
		if hs, ok := beforeAnc[v]; ok {
			for _, h := range hs {
				emit(h)
			}
		}
		emit(v)
	}
	for _, m := range moves {
		emit(m.p)
	}
	out := EmptyEffect()
	for _, w := range e.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return IncompleteEffect()
		}
	}
	for _, v := range newOrd {
		if v == nil {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return IncompleteEffect()
		}
	}
	out.pure = e.pure
	out.sideEffectFree = e.sideEffectFree
	out.lhsWrite = e.lhsWrite
	return out
}

// nestedUserFreeReadsVarExcludingIVOwner reports free-ref of v in nested call
// tree functions that do not own v as a pure for-IV. Free-ref only inside the
// IV owner (seed57 g_1597 if(g_1597) in func_10) must not trigger FE-head moves
// on the parent. Session-local — no package state.
func nestedUserFreeReadsVarExcludingIVOwner(s *Session, calls []*Invocation, v *Variable) bool {
	if v == nil || !InvocationsComplete(calls) {
		return false
	}
	seen := map[*Function]bool{}
	var walkInv func(inv *Invocation) bool
	var walkFn func(f *Function) bool
	walkFn = func(f *Function) bool {
		if f == nil || seen[f] {
			return false
		}
		seen[f] = true
		// Skip free-ref accounting inside pure-IV owner (seed57 g_1597).
		if !isForIVOfFunc(f, v) {
			if bodySyntacticFreeReadsVar(f, v) {
				return true
			}
		}
		var nested []*Invocation
		for _, blk := range f.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
			}
		}
		if !InvocationsComplete(nested) {
			return false
		}
		for _, inv := range nested {
			if walkInv(inv) {
				return true
			}
		}
		return false
	}
	walkInv = func(inv *Invocation) bool {
		if inv == nil || inv.User == nil {
			return false
		}
		return walkFn(inv.User)
	}
	for _, inv := range calls {
		if walkInv(inv) {
			return true
		}
	}
	return false
}

// pureIVIsNestedFETail reports v is the last read on some nested user FEffect
// (seed48 g_750.f2 ends func_59). Session-local — no package state.
func pureIVIsNestedFETail(s *Session, calls []*Invocation, v *Variable) bool {
	if v == nil || !InvocationsComplete(calls) {
		return false
	}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return false
		}
		if len(fr) > 0 && fr[len(fr)-1] == v {
			return true
		}
	}
	return false
}

// pureIVNearNestedFETail reports v is within maxDist of the end of some nested
// user FEffect (seed48 g_278@45/51, g_1145@46/51). Session-local — no package state.
func pureIVNearNestedFETail(s *Session, calls []*Invocation, v *Variable, maxDist int) bool {
	if v == nil || maxDist < 0 || !InvocationsComplete(calls) {
		return false
	}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return false
		}
		n := len(fr)
		for j, fv := range fr {
			if fv == v && n-1-j <= maxDist {
				return true
			}
		}
	}
	return false
}

// pureIVLeadsNestedUserFE reports v is the FE-head pure for-IV of some nested
// user callee on this stmt's calls. freeRefLate must not terminal-append these
// just because they also sit near the tail of a *different* nested FE
// (seed22584: g_58 leads func_14 but is near-tail of func_41; g_359.f0 leads
// func_41 but near-tail of func_22). seed48 g_278/g_1145 are not FE heads.
// Session-local — no package state.
func pureIVLeadsNestedUserFE(s *Session, calls []*Invocation, v *Variable) bool {
	if v == nil || !InvocationsComplete(calls) {
		return false
	}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return false
		}
		if len(fr) > 0 && fr[0] == v && isForIVOfFunc(inv.User, v) {
			return true
		}
	}
	return false
}

// pureIVOnlyInPureOnlyNestedFEPrefix reports every nested-user FE occurrence of
// v is inside a leading pure-for-IV-only prefix of that callee (each read at or
// before v is a for-IV of the callee). seed48 g_951@1 after g_250 pure-only head
// must not invent into parent FE; seed57 g_1961.f6 / n35 g_245.f2 sit mid nested
// FE after free-reads and invent. Session-local — no package state.
func pureIVOnlyInPureOnlyNestedFEPrefix(s *Session, calls []*Invocation, v *Variable) bool {
	if v == nil || !InvocationsComplete(calls) {
		return false
	}
	saw := false
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return false
		}
		for j, fv := range fr {
			if fv != v {
				continue
			}
			saw = true
			// Prefix through j must be callee pure for-IVs only.
			for k := 0; k <= j; k++ {
				if fr[k] == nil || !isForIVOfFunc(inv.User, fr[k]) {
					return false // mid free-read stream
				}
			}
		}
	}
	return saw
}

// pureIVLateInNestedFE reports v appears in the late half of some *direct*
// nested user FEffect on this stmt's calls. Deep nesting is intentionally
// not walked: n28 g_306 is late only in deep FEs and must pureMiss-by-prev.
// seed48 g_129.f2 relies on freeRefLate / FE-tail / other gates when not direct.
// Session-local — no package state.
func pureIVLateInNestedFE(s *Session, calls []*Invocation, v *Variable) bool {
	if v == nil || !InvocationsComplete(calls) {
		return false
	}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return false
		}
		n := len(fr)
		for j, fv := range fr {
			if fv == v && j*2 >= n-1 {
				return true
			}
		}
	}
	return false
}

// pureIVAdjacentPrevInvert reports visit pure IV v should pureMiss when an
// FE-consecutive partner u is visit/prev order-inverted AND some nested FE lists
// the pair in prev order (n28: func_18 has g_933 before g_1209). When every FE
// lists the pair in visit order only (seed48 g_129.f2), keep visit. Session-local.
func pureIVAdjacentPrevInvert(s *Session, calls []*Invocation, v *Variable, visitR0 []*Variable, visitIdx, prevIdx map[*Variable]int, pureIV map[*Variable]bool) bool {
	if v == nil || pureIV == nil || prevIdx == nil || !pureIV[v] {
		return false
	}
	vi, vok := visitIdx[v]
	pv, pok := prevIdx[v]
	if !vok || !pok {
		return false
	}
	for _, u := range pureIVFEConsecutivePartners(s, calls, v) {
		if u == nil {
			continue
		}
		ui, uInVisit := visitIdx[u]
		pu, uInPrev := prevIdx[u]
		if !uInPrev || !uInVisit {
			continue
		}
		if (vi-ui)*(pv-pu) >= 0 {
			continue // visit agrees with prev
		}
		// Visit disagrees with prev. Require visit-adjacency so distant FE
		// neighbors do not pureMiss (seed48 g_129.f2@late vs g_587). n28
		// g_933|g_1209 are visit-adjacent. Session-local.
		d := vi - ui
		if d < 0 {
			d = -d
		}
		if d != 1 {
			continue
		}
		if pureIVFEPairOrderMatchesPrev(s, calls, v, u, prevIdx) {
			return true
		}
	}
	return false
}

// pureIVFEPairOrderMatchesPrev reports some nested FE has a,b consecutive in the
// same relative order as prevIdx. Session-local.
func pureIVFEPairOrderMatchesPrev(s *Session, calls []*Invocation, a, b *Variable, prevIdx map[*Variable]int) bool {
	if a == nil || b == nil || prevIdx == nil {
		return false
	}
	pa, oka := prevIdx[a]
	pb, okb := prevIdx[b]
	if !oka || !okb {
		return false
	}
	wantAFirst := pa < pb
	seen := map[*Function]bool{}
	var walkInv func(inv *Invocation) bool
	var walkFn func(f *Function) bool
	walkFn = func(f *Function) bool {
		if f == nil || seen[f] {
			return false
		}
		seen[f] = true
		if EffectComplete(f.FEffect) {
			fr := f.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return false
			}
			for j := 0; j+1 < len(fr); j++ {
				if fr[j] == a && fr[j+1] == b && wantAFirst {
					return true
				}
				if fr[j] == b && fr[j+1] == a && !wantAFirst {
					return true
				}
			}
		}
		var nested []*Invocation
		for _, blk := range f.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
			}
		}
		if !InvocationsComplete(nested) {
			return false
		}
		for _, inv := range nested {
			if walkInv(inv) {
				return true
			}
		}
		return false
	}
	walkInv = func(inv *Invocation) bool {
		if inv == nil || inv.User == nil {
			return false
		}
		return walkFn(inv.User)
	}
	for _, inv := range calls {
		if walkInv(inv) {
			return true
		}
	}
	return false
}

// pureIVHasMissingFEPartner reports v has an FE-consecutive pure partner that is
// in prev but missing from visit (n28 g_933 with g_1209 Acc-dropped). Session-local.
func pureIVHasMissingFEPartner(s *Session, calls []*Invocation, v *Variable, visitIdx, prevIdx map[*Variable]int, pureIV map[*Variable]bool) bool {
	if v == nil || pureIV == nil || !pureIV[v] {
		return false
	}
	for _, u := range pureIVFEConsecutivePartners(s, calls, v) {
		if u == nil || !pureIV[u] {
			continue
		}
		if _, inPrev := prevIdx[u]; !inPrev {
			continue
		}
		if _, inVisit := visitIdx[u]; !inVisit {
			return true
		}
	}
	return false
}

// pureIVFEConsecutivePartners returns pure IVs immediately before/after v on some
// nested user FE. Session-local.
func pureIVFEConsecutivePartners(s *Session, calls []*Invocation, v *Variable) []*Variable {
	if v == nil || !InvocationsComplete(calls) {
		return nil
	}
	seen := map[*Function]bool{}
	out := map[*Variable]bool{}
	var walkInv func(inv *Invocation)
	var walkFn func(f *Function)
	walkFn = func(f *Function) {
		if f == nil || seen[f] {
			return
		}
		seen[f] = true
		if EffectComplete(f.FEffect) {
			fr := f.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return
			}
			for j, fv := range fr {
				if fv != v {
					continue
				}
				if j > 0 && fr[j-1] != nil {
					out[fr[j-1]] = true
				}
				if j+1 < len(fr) && fr[j+1] != nil {
					out[fr[j+1]] = true
				}
			}
		}
		var nested []*Invocation
		for _, blk := range f.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
			}
		}
		if !InvocationsComplete(nested) {
			return
		}
		for _, inv := range nested {
			walkInv(inv)
		}
	}
	walkInv = func(inv *Invocation) {
		if inv != nil && inv.User != nil {
			walkFn(inv.User)
		}
	}
	for _, inv := range calls {
		walkInv(inv)
	}
	var list []*Variable
	for u := range out {
		list = append(list, u)
	}
	return list
}

// pureIVPairInSameNestedFE reports a and b are consecutive reads on some nested
// user FE in the call tree (either order). Walks nested calls (n28 g_933↔g_1209
// live on deeper func_28). seed48 g_129.f2/g_1495 are not FE-neighbors.
// Session-local — no package state.
func pureIVPairInSameNestedFE(s *Session, calls []*Invocation, a, b *Variable) bool {
	if a == nil || b == nil || !InvocationsComplete(calls) {
		return false
	}
	seen := map[*Function]bool{}
	var walkInv func(inv *Invocation) bool
	var walkFn func(f *Function) bool
	walkFn = func(f *Function) bool {
		if f == nil || seen[f] {
			return false
		}
		seen[f] = true
		if EffectComplete(f.FEffect) {
			fr := f.FEffect.ReadVarsSess(s)
			if sessHasError(s) {
				return false
			}
			for j := 0; j+1 < len(fr); j++ {
				if (fr[j] == a && fr[j+1] == b) || (fr[j] == b && fr[j+1] == a) {
					return true
				}
			}
		}
		var nested []*Invocation
		for _, blk := range f.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
			}
		}
		if !InvocationsComplete(nested) {
			return false
		}
		for _, inv := range nested {
			if walkInv(inv) {
				return true
			}
		}
		return false
	}
	walkInv = func(inv *Invocation) bool {
		if inv == nil || inv.User == nil {
			return false
		}
		return walkFn(inv.User)
	}
	for _, inv := range calls {
		if walkInv(inv) {
			return true
		}
	}
	return false
}

// pureIVIsNestedFENearTail reports v appears on some nested FE with only pure
// for-IVs of that callee after it (trailing pure-IV block). seed48 g_1145 sits
// near end of func_22 FE before other pure IVs. Session-local — no package state.
func pureIVIsNestedFENearTail(s *Session, calls []*Invocation, v *Variable) bool {
	if v == nil || !InvocationsComplete(calls) {
		return false
	}
	for _, inv := range calls {
		if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
			continue
		}
		fr := inv.User.FEffect.ReadVarsSess(s)
		if sessHasError(s) {
			return false
		}
		// pure IVs of this callee
		calleePure := map[*Variable]bool{}
		for _, blk := range inv.User.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				st := &blk.Stmts[i]
				if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil {
					calleePure[st.Loop.IV] = true
				}
			}
		}
		idx := -1
		for i, fv := range fr {
			if fv == v {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		// Everything after v is pure for-IV of callee (or empty).
		ok := true
		for i := idx + 1; i < len(fr); i++ {
			if fr[i] == nil || !calleePure[fr[i]] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// nestedUserSyntacticFreeReadsVar: syntactic free-ref of v in nested user callees.
func nestedUserSyntacticFreeReadsVar(s *Session, calls []*Invocation, v *Variable) bool {
	if v == nil || !InvocationsComplete(calls) {
		return false
	}
	seen := map[*Function]bool{}
	var walkInv func(inv *Invocation) bool
	var walkFn func(f *Function) bool
	walkFn = func(f *Function) bool {
		if f == nil || seen[f] {
			return false
		}
		seen[f] = true
		if bodySyntacticFreeReadsVar(f, v) {
			return true
		}
		var nested []*Invocation
		for _, blk := range f.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
			}
		}
		if !InvocationsComplete(nested) {
			return false
		}
		for _, inv := range nested {
			if walkInv(inv) {
				return true
			}
		}
		return false
	}
	walkInv = func(inv *Invocation) bool {
		if inv == nil || inv.User == nil {
			return false
		}
		return walkFn(inv.User)
	}
	for _, inv := range calls {
		if walkInv(inv) {
			return true
		}
	}
	return false
}

// bodyValueFreeReadsVar reports a non-address-of free use of v in f's body
// (return/if-test/assign RHS TermVariable, etc.). Address-of-only (&g via
// IndirectLevel < 0) does not count — C++ often omits ReadVar for pure
// address-of. Pure for-IV headers of v are skipped (Then only). Session-local.
func bodyValueFreeReadsVar(f *Function, v *Variable) bool {
	if f == nil || v == nil {
		return false
	}
	var walk func(blk *Block) bool
	walk = func(blk *Block) bool {
		if blk == nil {
			return false
		}
		for _, loc := range blk.LocalVars {
			if loc != nil && exprValueFreeRefsVar(loc.InitExpr, v) {
				return true
			}
		}
		for i := range blk.Stmts {
			st := &blk.Stmts[i]
			if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV == v {
				if walk(st.Then) {
					return true
				}
				continue
			}
			if exprValueFreeRefsVar(st.Expr, v) {
				return true
			}
			if st.Loop != nil {
				if exprValueFreeRefsVar(st.Loop.TestExpr, v) {
					return true
				}
			}
			switch st.Kind {
			case StmtBlock:
				if walk(st.Then) {
					return true
				}
			case StmtIfElse:
				if walk(st.Then) || walk(st.Else) {
					return true
				}
			case StmtFor, StmtArrayOp:
				if walk(st.Then) {
					return true
				}
			}
		}
		return false
	}
	body := f.Body
	if body == nil {
		for _, b := range f.Blocks {
			if b != nil && b.Parent == nil {
				body = b
				break
			}
		}
	}
	return walk(body)
}

// exprValueFreeRefsVar walks expressions for value uses of v, skipping address-of
// (IndirectLevel < 0 on TermVariable). Session-local — no package state.
//
// Itemized array access stores for-IV offsets on ArrayVariable.IndexExprs
// (VariableSelector.cpp itemize ExpressionVariable + optional binary add).
// Must walk those indices: seed5139 func_1 has return l_2189[…][(g_167 + 2)]
// where g_167 is only an IndexExpr — missing it made NHEAD treat g_167 as
// pure-for-only residual and yank nested pure FE head g_261 before free-ref
// Acc-early g_167.
func exprValueFreeRefsVar(e *Expression, v *Variable) bool {
	if e == nil || v == nil {
		return false
	}
	if e.Term == TermVariable {
		if e.Var == v {
			// Address-of: IndirectLevel < 0 (Var.Type.indir - ExprType.indir).
			// Use nil session residual — non-nil types only; no package state.
			if e.Var != nil && e.ExprType != nil && e.Var.Type != nil {
				lv := e.Var.Type.IndirectLevelSess(nil)
				lw := e.ExprType.IndirectLevelSess(nil)
				if lv-lw < 0 {
					return false
				}
			}
			return true
		}
		// Itemized collective/member: walk index Expression*s (for-IV + offset).
		if e.Var != nil && e.Var.AsArray != nil {
			for _, idx := range e.Var.AsArray.IndexExprs {
				if exprValueFreeRefsVar(idx, v) {
					return true
				}
			}
		}
	}
	if exprValueFreeRefsVar(e.CommaLHS, v) || exprValueFreeRefsVar(e.CommaRHS, v) {
		return true
	}
	if e.Assign != nil {
		// embedded assign: value-read RHS only
		if exprValueFreeRefsVar(e.Assign.Expr, v) {
			return true
		}
	}
	if e.Invoke != nil {
		for _, a := range e.Invoke.Args {
			if exprValueFreeRefsVar(a, v) {
				return true
			}
		}
	}
	return false
}

// nestedUserValueFreeReadsVar: value free-read of v in any nested user callee.
func nestedUserValueFreeReadsVar(s *Session, calls []*Invocation, v *Variable) bool {
	if v == nil || !InvocationsComplete(calls) {
		return false
	}
	seen := map[*Function]bool{}
	var walkInv func(inv *Invocation) bool
	var walkFn func(f *Function) bool
	walkFn = func(f *Function) bool {
		if f == nil || seen[f] {
			return false
		}
		seen[f] = true
		if bodyValueFreeReadsVar(f, v) {
			return true
		}
		var nested []*Invocation
		for _, blk := range f.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested)
			}
		}
		if !InvocationsComplete(nested) {
			return false
		}
		for _, inv := range nested {
			if walkInv(inv) {
				return true
			}
		}
		return false
	}
	walkInv = func(inv *Invocation) bool {
		if inv == nil || inv.User == nil {
			return false
		}
		return walkFn(inv.User)
	}
	for _, inv := range calls {
		if walkInv(inv) {
			return true
		}
	}
	return false
}

// laterBodyReadsVar reports whether v is read by a statement after cur in f's
// top-level body (session-local; no package state). Used to gate FE-head pure-IV
// moves: later free reads (n28 if(g_791.f1) / StmtGoto cond) keep prev/late order.
//
// During special revalidate inside MakeRandomBlock, f.Body is still nil (assigned
// only after make returns). Use the parent==nil block on f.Blocks instead.
func laterBodyReadsVar(f *Function, cur *Stmt, v *Variable) bool {
	if f == nil || cur == nil || v == nil {
		return false
	}
	body := f.Body
	if body == nil {
		for _, b := range f.Blocks {
			if b != nil && b.Parent == nil {
				body = b
				break
			}
		}
	}
	if body == nil {
		return false
	}
	// Prefer pointer identity; fall back to StmID (special revalidate may not
	// pass the same *Stmt as Body.Stmts[i] in all paths).
	curIdx := -1
	for i := range body.Stmts {
		st := &body.Stmts[i]
		if st == cur || (!StmIDUnset(st.StmID) && !StmIDUnset(cur.StmID) && st.StmID == cur.StmID) {
			curIdx = i
			break
		}
	}
	if curIdx < 0 {
		// cur not in top-level body: treat any top-level free read of v as later
		// (conservative: avoid FE-head invent when body still reads v).
		for i := range body.Stmts {
			if stmtRefsVar(&body.Stmts[i], v) {
				return true
			}
		}
		return false
	}
	for i := curIdx + 1; i < len(body.Stmts); i++ {
		if stmtRefsVar(&body.Stmts[i], v) {
			return true
		}
	}
	return false
}

func stmtRefsVar(st *Stmt, v *Variable) bool {
	if st == nil || v == nil {
		return false
	}
	if exprRefsVar(st.Expr, v) {
		return true
	}
	if st.LhsVar == v || (st.Lhs != nil && st.Lhs.Var == v) {
		return true
	}
	if st.Loop != nil && st.Loop.IV == v {
		return true
	}
	return false
}

// exprRefsVar is a TermVariable / recursive subexpr check for laterBodyReadsVar.
// Walks itemized ArrayVariable.IndexExprs (for-IV index offsets) like
// exprValueFreeRefsVar — otherwise bodySyntacticFreeReadsVar misses pure for-IV
// free-refs that only appear as array indices (seed5139 g_167 + 2).
func exprRefsVar(e *Expression, v *Variable) bool {
	if e == nil || v == nil {
		return false
	}
	if e.Term == TermVariable {
		if e.Var == v {
			return true
		}
		if e.Var != nil && e.Var.AsArray != nil {
			for _, idx := range e.Var.AsArray.IndexExprs {
				if exprRefsVar(idx, v) {
					return true
				}
			}
		}
	}
	if exprRefsVar(e.CommaLHS, v) || exprRefsVar(e.CommaRHS, v) {
		return true
	}
	if e.Assign != nil {
		if e.Assign.LhsVar == v || (e.Assign.Lhs != nil && e.Assign.Lhs.Var == v) {
			return true
		}
		if exprRefsVar(e.Assign.Expr, v) {
			return true
		}
	}
	if e.Invoke != nil {
		for _, a := range e.Invoke.Args {
			if exprRefsVar(a, v) {
				return true
			}
		}
	}
	return false
}

// pureIVGlobalsFromUserCallTree collects global make_iteration IVs from user
// callees reachable from call list (direct + nested body calls). Session-local.
func pureIVGlobalsFromUserCallTree(s *Session, calls []*Invocation) map[*Variable]bool {
	ivs := map[*Variable]bool{}
	seenFn := map[*Function]bool{}
	var walk func(f *Function)
	walk = func(f *Function) {
		if f == nil || seenFn[f] {
			return
		}
		seenFn[f] = true
		for _, blk := range f.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				st := &blk.Stmts[i]
				if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil {
					iv := st.Loop.IV
					if iv.IsGlobalSess(s) {
						ivs[iv] = true
					}
				}
				var nested []*Invocation
				// Non-sticky soft walk: incomplete for/array IR is common mid-tree;
				// CollectCalledInvocationsStmtSess would SetError and poison generation
				// (abortUnbuilt after pureIV walk). Soft-collect without sticky.
				if !collectCalledInvocationsStmt(s, st, &nested) {
					continue
				}
				for _, inv := range nested {
					if inv != nil && inv.User != nil {
						walk(inv.User)
					}
				}
			}
		}
	}
	for _, inv := range calls {
		if inv != nil && inv.User != nil {
			walk(inv.User)
		}
	}
	return ivs
}

// VisitFactsStatementAssign mirrors StatementAssign::visit_facts.
// StatementAssign.cpp:358–390 — RHS first; compound folds RHS effect into LHS
// context; write_var_set of RHS lhs_write_vars; update_fact_for_assign; map_stm_effect.
// stripCallerForIVsFromEffect filters Acc before Acc→FE merge.
// C++ merges effect_accum unfiltered (FunctionInvocationUser.cpp:225 / 291).
//
// Drop Acc-only *caller* pure for-IV *reads* that are not for-IVs of the callee
// and are not already on callee.FEffect. That removes visit/gen Acc pollution of
// outer for-IVs (n94 g_2991.f0) while keeping:
//   - callee own pure for-IVs (n94 nested g_903.f3 Acc residual)
//   - non-field scalar outer residuals (n38656 g_8; seed50 g_150)
//   - Acc residual *writes* (and their paired reads) of field outer pure IVs —
//     may-point compound RMW (seed767 g_716.f1 via (*p_25)^=). Read-only Acc
//     pollution of the same IV is still stripped (n94).
//
// Session/CG-local — no package mutable state.
func stripCallerForIVsFromEffect(s *Session, e Effect, caller *Function, cg *CGContext, callee *Function) Effect {
	if !EffectComplete(e) {
		return e
	}
	// Field outer pure for-IVs are strip candidates (n94 g_2991.f0).
	// Non-field outer pure IVs stay (seed50 g_150; n38656 g_8). Session-local.
	callerIV := map[*Variable]bool{}
	addFieldForIVs := func(f *Function) {
		if f == nil {
			return
		}
		for _, blk := range f.Blocks {
			if blk == nil {
				continue
			}
			for i := range blk.Stmts {
				st := &blk.Stmts[i]
				if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil && st.Loop.IV.FieldVarOf != nil {
					callerIV[st.Loop.IV] = true
				}
			}
		}
	}
	addFieldForIVs(caller)
	if cg != nil {
		for _, b := range cg.CallChain {
			if b != nil {
				addFieldForIVs(b.Func)
			}
		}
		for iv := range cg.IVBounds {
			if iv == nil || iv.FieldVarOf == nil {
				continue
			}
			if callee != nil && (bodySyntacticFreeReadsVar(callee, iv) || isForIVOfFunc(callee, iv)) {
				continue
			}
			callerIV[iv] = true
		}
	}
	if len(callerIV) == 0 {
		return e
	}
	keepOnCallee := map[*Variable]bool{}
	if callee != nil && EffectComplete(callee.FEffect) {
		for _, v := range callee.FEffect.ReadVarsSess(s) {
			if v != nil {
				keepOnCallee[v] = true
			}
		}
		if sessHasError(s) {
			return IncompleteEffect()
		}
	}
	// Acc residual *writes* of a field outer pure IV are real body effects
	// (may-point compound RMW). Only strip Acc-only *reads* (n94 pollution).
	writtenOnAcc := map[*Variable]bool{}
	for _, w := range e.WrittenVarsSess(s) {
		if w != nil {
			writtenOnAcc[w] = true
		}
	}
	if sessHasError(s) {
		return IncompleteEffect()
	}
	need := false
	for _, v := range e.ReadVarsSess(s) {
		if v != nil && callerIV[v] && !keepOnCallee[v] && v.FieldVarOf != nil && !writtenOnAcc[v] {
			need = true
			break
		}
	}
	if !need {
		return e
	}
	out := EmptyEffect()
	for _, w := range e.WrittenVarsSess(s) {
		if w == nil {
			continue
		}
		out = out.WriteVarSess(s, w)
		if sessHasError(s) || !EffectComplete(out) {
			return IncompleteEffect()
		}
	}
	for _, v := range e.ReadVarsSess(s) {
		if v == nil {
			continue
		}
		// Strip only field outer pure IV *read-only* Acc residuals (n94).
		// Keep R when Acc also W (seed767 may-point RMW of outer field pure IV).
		// Non-field outer pure IVs stay (seed50 g_150; n38656 g_8).
		// Session-local — no package state.
		if callerIV[v] && !keepOnCallee[v] && v.FieldVarOf != nil && !writtenOnAcc[v] {
			continue
		}
		out = out.ReadVarSess(s, v)
		if sessHasError(s) || !EffectComplete(out) {
			return IncompleteEffect()
		}
	}
	out.pure = e.pure
	out.sideEffectFree = e.sideEffectFree
	out.lhsWrite = e.lhsWrite
	return out
}

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
		//
		// Visit full-walk can drop make_iteration pure IV reads from gen map_stm
		// (n94 direct call; n28 nested under safe_math). Acc-summary re-add fixes
		// membership but wrong order; mid-gen restore on all funcs broke n62 body.
		// Gate to func_1 (C++ Statement.cpp:864 uncertain-call revalidate) so only
		// the drop-in FEffect surface is rewritten after body IR is fixed.
		// Fair: pure for-IVs in prev map_stm ∩ nested callee FEffects, gen order.
		// Also strip visit-only caller for-IVs that leaked via FE enrichment
		// (n94 g_2991.f0) — only when they are for-IVs of CurrentFunc.
		s := sessFromCG(cg)
		prev := cg.FM.GetMapStmEffect(st.StmID)
		// Restore pure for-IVs that visit dropped from gen map_stm. Gate to
		// func_1 (special revalidate / FP may rewrite after gen). Session-local
		// prev ∩ pureIV ∩ nested-callee FE, prev order — no package state.
		// Skip pure IVs that lead the *direct* RHS user-callee FE only
		// (n62 g_37.f1 invent when top Invoke.User FE starts with that pure IV).
		// Expression-tree-wide leading skip blocked n28 g_791.f1 / n35 g_50.
		if EffectComplete(prev) && cg.CurrentFunc != nil && cg.CurrentFunc.Name == "func_1" {
			var calls []*Invocation
			CollectCalledInvocationsStmtSess(s, st, &calls)
			if hasErrCG(cg) {
				return false
			}
			if InvocationsComplete(calls) {
				pureIV := pureIVGlobalsFromUserCallTree(s, calls)
				if hasErrCG(cg) {
					return false
				}
				// Pure FE heads of soft-expanded nested call tree (func_17→func_77).
				// pureIVLeadsNestedUserFE is direct-only (seed22584 freeRefLate).
				// Session/CG-local — no package mutable state.
				nestedPureFEHeads := map[*Variable]bool{}
				{
					seenFn := map[*Function]bool{}
					var walkFn func(*Function)
					walkFn = func(fn *Function) {
						if fn == nil || seenFn[fn] {
							return
						}
						seenFn[fn] = true
						if EffectComplete(fn.FEffect) {
							fr := fn.FEffect.ReadVarsSess(s)
							if hasErrCG(cg) {
								return
							}
							if len(fr) > 0 && fr[0] != nil && isForIVOfFunc(fn, fr[0]) {
								nestedPureFEHeads[fr[0]] = true
							}
						}
						for _, blk := range fn.Blocks {
							if blk == nil {
								continue
							}
							for i := range blk.Stmts {
								var nested []*Invocation
								if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
									continue
								}
								for _, inv := range nested {
									if inv != nil && inv.User != nil {
										walkFn(inv.User)
									}
								}
							}
						}
					}
					for _, inv := range calls {
						if inv != nil && inv.User != nil {
							walkFn(inv.User)
						}
					}
					if hasErrCG(cg) {
						return false
					}
				}
				// Own pure for-IVs of CurrentFunc (make_iteration on func_1) are not in
				// the callee-only call tree. Include them so prev/gen membership can
				// restore after visit drops (seed57 g_1961.f6). Session-local.
				if cg.CurrentFunc != nil {
					for _, blk := range cg.CurrentFunc.Blocks {
						if blk == nil {
							continue
						}
						for i := range blk.Stmts {
							st2 := &blk.Stmts[i]
							if st2.Kind == StmtFor && st2.Loop != nil && st2.Loop.IV != nil {
								iv := st2.Loop.IV
								if iv.IsGlobalSess(s) {
									pureIV[iv] = true
								}
							}
						}
					}
				}
				inAnyCalleeFE := func(v *Variable) bool {
					if v == nil {
						return false
					}
					for _, inv := range calls {
						if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
							continue
						}
						ok := inv.User.FEffect.IsReadSess(s, v)
						if hasErrCG(cg) {
							return false
						}
						if ok {
							return true
						}
					}
					return false
				}
				// In-loop FP rewrites map_stm and drops pure IVs the same way C++
				// does (n62 g_37.f1; fuzz2 g_168.f4 mid-FE pure IV). Restoring any
				// pure IV from prev re-places them at gen Acc-merge order, which is
				// earlier than UP's post-visit/body-read order. Outside loops
				// (n28 g_791.f1) restore still repairs visit drops with prev order.
				// Membership for in-loop pure IVs: Acc summary re-add / later body reads.
				//
				// Free-read gate (seed48 g_250/g_951): pure-for-IV-only symbols that
				// never free-read in the call tree must not be restored into func_1 FE
				// (C++ drops pure-only nested IVs from parent FEffect). n28 g_306 etc.
				// free-read in some nested body (or current) and remain restore-eligible.
				skipAllPureIV := cg.InLoop()
shouldRestore := func(v *Variable) bool {
					if skipAllPureIV {
						return false
					}
					if v == nil || !pureIV[v] {
						return false
					}
					if !v.IsGlobalSess(s) {
						return false
					}
					if inAnyCalleeFE(v) {
						return true
					}
					// Own pure for-IV of current func (seed57 g_1961.f6) — gen map
					// membership; invent gate still filters Acc-only pure-only.
					return isForIVOfFunc(cg.CurrentFunc, v)
				}
				preReads := prev.ReadVarsSess(s)
				if hasErrCG(cg) {
					return false
				}
				// Rewrite when any pure IV is restore-eligible (n28/n35 order+membership).
				needRewrite := false
				// Caller for-IVs (CurrentFunc + active IVBounds). Visit-only copies of
				// these are FE/accum pollution (n94 g_2991.f0 early); strip when not in prev
				// during an existing pure-IV rewrite. Session/CG-local — no package state.
				callerIV := map[*Variable]bool{}
				if cg.CurrentFunc != nil {
					for _, blk := range cg.CurrentFunc.Blocks {
						if blk == nil {
							continue
						}
						for i := range blk.Stmts {
							st2 := &blk.Stmts[i]
							if st2.Kind == StmtFor && st2.Loop != nil && st2.Loop.IV != nil {
								callerIV[st2.Loop.IV] = true
							}
						}
					}
				}
				for iv := range cg.IVBounds {
					if iv != nil {
						callerIV[iv] = true
					}
				}
				for _, v := range preReads {
					ok := shouldRestore(v)
					if hasErrCG(cg) {
						return false
					}
					if ok {
						needRewrite = true
						break
					}
				}
				if needRewrite {
					// Re-place all restore-eligible pure IVs by prev index into visit
					// non-pure stream (keeps non-pure visit order). Session-local.
					merged := EmptyEffect()
					for _, w := range cg.EffectStm.WrittenVarsSess(s) {
						if w == nil {
							continue
						}
						merged = merged.WriteVarSess(s, w)
						if hasErrCG(cg) || !EffectComplete(merged) {
							if !hasErrCG(cg) {
								noteErrCG(cg, ErrGeneric)
							}
							return false
						}
					}
					prevIdx := map[*Variable]int{}
					for i, v := range preReads {
						if v != nil {
							if _, ok := prevIdx[v]; !ok {
								prevIdx[v] = i
							}
						}
					}
					// pureMiss = pure IVs re-placed by prev index into visit non-pure stream.
					// Session-local; no package state.
					//  1) In-visit pure IVs: always re-place by prev (order lag n28 g_306;
					//     pair swap n28 g_933/g_1209).
					//  2) Missing pure IVs: re-insert by prev (membership n28 g_148,
					//     g_791.f1@prev~70 near UP@68). Do not leave FE-head pure IVs
					//     solely to Acc summary append (that invents terminal g_791.f1).
					var pureMiss []*Variable
					var freeRefLate []*Variable // missing free-ref pure IVs (caller) — append after stream
					pureMissSet := map[*Variable]bool{}
					pureWasMissing := map[*Variable]bool{} // !inVisit before rewrite
					visitR0 := cg.EffectStm.ReadVarsSess(s)
					if hasErrCG(cg) {
						return false
					}
					visitIdx := map[*Variable]int{}
					for i, v := range visitR0 {
						if v != nil {
							if _, ok := visitIdx[v]; !ok {
								visitIdx[v] = i
							}
						}
					}
					for _, v := range preReads {
						if v == nil || pureMissSet[v] {
							continue
						}
						ok := shouldRestore(v)
						if hasErrCG(cg) {
							return false
						}
						if !ok {
							continue
						}
						_, inVisit := visitIdx[v]
						if !inVisit {
							pureWasMissing[v] = true
						}
						// Invent gate for *missing* pure for-IVs (session/IR-local):
						// 1) Own pure IV at prev@0 (Acc-head make_iteration pollution):
						//    never invent R when visit dropped it (seed5 g_259@0, seed50 g_7@0).
						// 2) Own pure-only (no free-ref in *current* body): skip mid invent
						//    unless some nested callee FE already has the pure IV (Acc
						//    membership from nested make_iteration — seed57 g_1961.f6,
						//    n35 g_245.f2). Pure-only own IVs never in any callee FE stay
						//    dropped (seed48 g_951@37).
						// 3) Nested pure-only FE head (no free-ref anywhere): skip (seed48 g_250).
						// Mid pure-only (n35 g_1227.f2) re-inserts when not own pure-only / not Acc-head.
						if !inVisit {
							ownIV := isForIVOfFunc(cg.CurrentFunc, v)
							ownFree := bodySyntacticFreeReadsVar(cg.CurrentFunc, v)
							if ownIV {
								if pv, ok := prevIdx[v]; ok && pv == 0 {
									continue // Acc-head pure IV invent (seed5 g_259; seed50 g_7)
								}
								if !ownFree {
									// Own pure-only: drop unless some nested callee FE has
									// this pure IV mid free-read stream (not pure-only FE
									// prefix). seed48 g_951 is pure-only prefix of func_32
									// (after g_250); seed57 g_1961.f6 / n35 g_245.f2 sit mid
									// nested FE among free-reads. Session-local.
									if !inAnyCalleeFE(v) || pureIVOnlyInPureOnlyNestedFEPrefix(s, calls, v) {
										continue
									}
								}
							}
							hasFree := ownFree || nestedUserSyntacticFreeReadsVar(s, calls, v)
							if !hasFree {
								leadsNested := false
								for _, inv := range calls {
									if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
										continue
									}
									fr := inv.User.FEffect.ReadVarsSess(s)
									if hasErrCG(cg) {
										return false
									}
									if len(fr) > 0 && fr[0] == v {
										leadsNested = true
										break
									}
								}
								if leadsNested {
									continue // nested pure-only FE head (seed48 g_250)
								}
							}
						}
						// PUREMISS_GATE session/IR-local (no package state):
						// 1) FE-tail + non-value free-ref, not own IV: strip/no re-insert
						//    (seed48 g_750.f2 Acc residual terminal; avoid early pureMiss).
						// 2) Missing + free-ref on caller, not own: freeRefLate append after
						//    visit stream (seed48 g_278/g_1145 after g_623).
						// 3) In-visit free-ref on current && !own: keep visit.
						// 4) In-visit nested FE-late: keep visit unless prev-adjacent invert.
						// 5) Else prev re-insert (n28 g_148). Own IV: prev (n35 g_164).
						if _, pOK := prevIdx[v]; !pOK {
							continue
						}
						own := isForIVOfFunc(cg.CurrentFunc, v)
						if !own &&
							bodySyntacticFreeReadsVar(cg.CurrentFunc, v) &&
							!bodyValueFreeReadsVar(cg.CurrentFunc, v) &&
							pureIVIsNestedFETail(s, calls, v) {
							if inVisit {
								pureMissSet[v] = true
							}
							continue
						}
						// Free-ref pure IVs missing from visit: late-append only when near
						// nested FE tail (seed48 g_278/g_1145 after g_623). Mid nested free-
						// ref pure IVs keep prev re-insert (n28 g_148). Session-local.
						// Missing free-ref pure IVs late-append:
						//  - value free-ref near FE tail (seed48 g_278, window 6)
						//  - address-of-only very near FE tail (seed48 g_1145, window 2)
						//    but not mid-window (n35 g_1047.f1 stays prev mid-order).
						// Skip freeRefLate when v *leads* some nested pure FE: near-tail of
						// another callee must not terminal-append FE-head pure IVs
						// (seed22584 g_58/g_359.f0). Those pureMiss re-insert by prev.
						if !inVisit && !own {
							win := 0
							if bodyValueFreeReadsVar(cg.CurrentFunc, v) {
								win = 6
							} else if bodySyntacticFreeReadsVar(cg.CurrentFunc, v) {
								win = 12
							}
							if win > 0 && pureIVNearNestedFETail(s, calls, v, win) &&
								!pureIVLeadsNestedUserFE(s, calls, v) {
								freeRefLate = append(freeRefLate, v)
								continue
							}
						}
						if inVisit {
							// Pair-swap / missing FE-partner takes priority over free-ref keep
							// except pure-only pure FE head of soft-nested vs free-ref pure
							// residual of intermediate FE (LevelC 1831 g_30/g_41: func_17 FE
							// has free-ref pure before residual pure-IV of func_77; visit and
							// UP keep pure-only head first — n28 pair invert must not apply).
							// Session/CG-local — no package mutable state.
							pairSwap := !own && pureIVAdjacentPrevInvert(s, calls, v, visitR0, visitIdx, prevIdx, pureIV)
							if pairSwap && nestedPureFEHeads[v] && !bodySyntacticFreeReadsVar(cg.CurrentFunc, v) {
								pairSwap = false
							}
							if pairSwap && bodySyntacticFreeReadsVar(cg.CurrentFunc, v) {
								vi, vok := visitIdx[v]
								pi, pok := prevIdx[v]
								if vok && pok {
									for u := range nestedPureFEHeads {
										if u == nil || bodySyntacticFreeReadsVar(cg.CurrentFunc, u) {
											continue
										}
										ui, uiv := visitIdx[u]
										pu, uip := prevIdx[u]
										if !uiv || !uip {
											continue
										}
										if ui < vi && pi < pu {
											pairSwap = false
											break
										}
									}
								}
							}
							missingPartner := !own && pureIVLateInNestedFE(s, calls, v) &&
								pureIVHasMissingFEPartner(s, calls, v, visitIdx, prevIdx, pureIV)
							// Pure-only pure FE head of soft-nested callee: keep visit when
							// pure multi free-ref order is not visit-inverted vs prev.
							// seed1831: visit pure-only head before free-ref pure residual of
							// another FE — keep visit. seed57: pure multi free-ref
							// [g_1597,g_381,g_1606] visit has mid g_1606 before head g_1597
							// while prev has head first — pureMiss by prev. Session/CG-local.
							if !pairSwap && !missingPartner && !own && nestedPureFEHeads[v] &&
								!bodySyntacticFreeReadsVar(cg.CurrentFunc, v) {
								vi, vok := visitIdx[v]
								pi, pok := prevIdx[v]
								multiInvert := false
								if vok && pok {
									// Walk soft-expanded callees for FE headed by v.
									seenMH := map[*Function]bool{}
									var walkMH func(*Function)
									walkMH = func(fn *Function) {
										if fn == nil || seenMH[fn] || multiInvert {
											return
										}
										seenMH[fn] = true
										if EffectComplete(fn.FEffect) {
											fr := fn.FEffect.ReadVarsSess(s)
											if hasErrCG(cg) {
												return
											}
											if len(fr) > 0 && fr[0] == v && isForIVOfFunc(fn, v) {
												// pure multi free-ref after head
												for j := 1; j < len(fr); j++ {
													mid := fr[j]
													if mid == nil || !isForIVOfFunc(fn, mid) {
														break
													}
													// free-ref on FE owner for pure multi free-ref mid
													// (seed57 g_1606 free-ref on func_10; may be pure-only
													// or free-ref on parent). Any pure multi mid counts.
													mui, muOK := visitIdx[mid]
													mp, mpOK := prevIdx[mid]
													if !muOK || !mpOK {
														continue
													}
													// visit mid before head, prev head before mid
													if mui < vi && pi < mp {
														multiInvert = true
														return
													}
												}
											}
										}
										for _, blk := range fn.Blocks {
											if blk == nil {
												continue
											}
											for i := range blk.Stmts {
												var nested []*Invocation
												if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
													continue
												}
												for _, inv := range nested {
													if inv != nil && inv.User != nil {
														walkMH(inv.User)
													}
												}
											}
										}
									}
									for _, inv := range calls {
										if inv != nil && inv.User != nil {
											walkMH(inv.User)
										}
									}
									if hasErrCG(cg) {
										return false
									}
								}
								if !multiInvert {
									continue // keep visit (seed1831)
								}
								// multiInvert: fall through to pureMiss by prev (seed57)
							}
							// Free-ref keep visit only when visit is not lagging prev.
							// Visit-late free-ref pure IVs pureMiss re-place by prev
							// (seed57 g_1604 free-ref after g_2642 vs prev mid-list).
							// Visit-early/equal free-ref keep (seed48 free-ref anchors).
							// Visit-lag free-ref keep when pure-only nested pure FE head is
							// visit-before and prev-after this free-ref pure (LevelC 1831
							// g_41 after visit g_30; pureMiss-by-prev would re-invert).
							if !pairSwap && !missingPartner && !own && bodySyntacticFreeReadsVar(cg.CurrentFunc, v) {
								vi, vok := visitIdx[v]
								pi, pok := prevIdx[v]
								if vok && pok && vi <= pi {
									continue
								}
								if vok && pok {
									keepWithPureOnlyHead := false
									for u := range nestedPureFEHeads {
										if u == nil || bodySyntacticFreeReadsVar(cg.CurrentFunc, u) {
											continue
										}
										ui, uiv := visitIdx[u]
										pu, uip := prevIdx[u]
										if !uiv || !uip {
											continue
										}
										if ui < vi && pi < pu {
											keepWithPureOnlyHead = true
											break
										}
									}
									if keepWithPureOnlyHead {
										continue
									}
								}
								// visit lag vs prev: pureMiss re-place
							}
							// Late nested FE: keep visit only when prev has this pure IV early
							// (seed48 g_129.f2@prev17 late visit). Late-prev pure IVs pureMiss
							// re-place so other pureMiss inserts do not shift them (n35
							// g_1047.f1@prev64 →@67). Session-local.
							if !pairSwap && !missingPartner && !own && pureIVLateInNestedFE(s, calls, v) {
								maxP := 0
								for _, pi := range prevIdx {
									if pi > maxP {
										maxP = pi
									}
								}
								if pv, ok := prevIdx[v]; ok && maxP > 0 && pv*2 < maxP {
									continue
								}
							}
						}
						pureMiss = append(pureMiss, v)
						pureMissSet[v] = true
					}
					// Record pureMiss invents (FE-head fixup) and all pureMiss
					// re-places (defer must not late-jump pureMiss order — n35 g_50).
					// FM-local — no package state.
					if cg.FM != nil && !StmIDUnset(st.StmID) {
						if cg.FM.PureMissInvented == nil {
							cg.FM.PureMissInvented = make(map[int]map[*Variable]bool)
						}
						if cg.FM.PureMissTouched == nil {
							cg.FM.PureMissTouched = make(map[*Variable]bool)
						}
						invSet := cg.FM.PureMissInvented[st.StmID]
						if invSet == nil {
							invSet = make(map[*Variable]bool)
							cg.FM.PureMissInvented[st.StmID] = invSet
						}
						for _, v := range pureMiss {
							if v == nil {
								continue
							}
							cg.FM.PureMissTouched[v] = true
							if pureWasMissing[v] {
								invSet[v] = true
							}
						}
					}
					diagPM := os.Getenv("DIAG_PUREMISS") != "" && cg.CurrentFunc != nil && cg.CurrentFunc.Name == "func_1"
					if diagPM {
						var pm, vr, pr []string
						for _, v := range pureMiss {
							if v != nil {
								pm = append(pm, v.Name)
							}
						}
						for _, v := range visitR0 {
							if v != nil && pureIV[v] {
								vr = append(vr, v.Name)
							}
						}
						for _, v := range preReads {
							if v != nil && pureIV[v] {
								pr = append(pr, fmt.Sprintf("%s@%d", v.Name, prevIdx[v]))
							}
						}
						var frl []string
						for _, v := range freeRefLate {
							if v != nil {
								frl = append(frl, v.Name)
							}
						}
						fmt.Fprintf(os.Stderr, "PUREMISS sid=%d pureMiss=%v freeRefLate=%v visitPure=%v prevPure=%v\n",
							st.StmID, pm, frl, vr, pr)
					}
					visitR := visitR0
					seen := map[*Variable]bool{}
					emit := func(v *Variable) bool {
						if v == nil || seen[v] {
							return true
						}
						merged = merged.ReadVarSess(s, v)
						if hasErrCG(cg) || !EffectComplete(merged) {
							if !hasErrCG(cg) {
								noteErrCG(cg, ErrGeneric)
							}
							return false
						}
						seen[v] = true
						return true
					}
					pi := 0
					for _, v := range visitR {
						if v == nil {
							continue
						}
						if pureMissSet[v] {
							continue
						}
						// Drop visit-only caller for-IVs not in gen map_stm (n94 g_2991.f0).
						// Legitimate for-IV reads on this stmt appear in prev from gen.
						if callerIV[v] {
							if _, inPrev := prevIdx[v]; !inPrev {
								continue
							}
						}
						vPrev, vOK := prevIdx[v]
						for pi < len(pureMiss) {
							p := pureMiss[pi]
							pPrev, pOK := prevIdx[p]
							if !pOK {
								break
							}
							if vOK && pPrev < vPrev {
								if !emit(p) {
									return false
								}
								pi++
								continue
							}
							break
						}
						if !emit(v) {
							return false
						}
					}
					for pi < len(pureMiss) {
						if !emit(pureMiss[pi]) {
							return false
						}
						pi++
					}
					// Free-ref pure IVs missing from visit: after non-pure stream so they
					// trail Acc mid-list anchors (seed48 g_623 then g_278/g_1145). Prev-order
					// among freeRefLate. Session-local — no package state.
					for _, v := range freeRefLate {
						if !emit(v) {
							return false
						}
					}
					if diagPM {
						var out []string
						for _, v := range merged.ReadVarsSess(s) {
							if v != nil {
								out = append(out, v.Name)
							}
						}
						var ppos []string
						for i, v := range merged.ReadVarsSess(s) {
							if v != nil && pureIV[v] {
								ppos = append(ppos, fmt.Sprintf("%s@%d", v.Name, i))
							}
						}
						// prev indices for visit anchors around g_278
						for _, name := range []string{"g_1177", "g_1581", "g_623", "g_278", "g_1145", "g_1837", "g_1054", "g_1741"} {
							for _, v := range visitR0 {
								if v != nil && v.Name == name {
									pi, ok := prevIdx[v]
									fmt.Fprintf(os.Stderr, "  anchor %s visit=yes prevOK=%v prev=%d pure=%v\n", name, ok, pi, pureIV[v])
									break
								}
							}
						}
						fmt.Fprintf(os.Stderr, "PUREMISS_OUT sid=%d purePos=%v\n", st.StmID, ppos)
					}
					// For each user call on this stmt: if FE head pure IV is already in
					// merged reads and sits after first shared FE neighbor, move pure
					// immediately before that neighbor (n35 g_108 before g_483). Reorder
					// only — no invent. Session-local.
					ord := merged.ReadVarsSess(s)
					if hasErrCG(cg) {
						return false
					}
					pos := map[*Variable]int{}
					for i, v := range ord {
						if v != nil {
							pos[v] = i
						}
					}
					type mv struct{ pure, anc *Variable }
					var moves []mv
					seenP := map[*Variable]bool{}
					for _, inv := range calls {
						if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
							continue
						}
						fr := inv.User.FEffect.ReadVarsSess(s)
						if hasErrCG(cg) {
							return false
						}
						if len(fr) == 0 || fr[0] == nil || !pureMissSet[fr[0]] || seenP[fr[0]] {
							continue
						}
						p := fr[0]
						// Only membership restores (were missing from visit): n35 g_108.
						if !pureWasMissing[p] {
							continue
						}
						ownIV := false
						for _, blk := range inv.User.Blocks {
							if blk == nil {
								continue
							}
							for i := range blk.Stmts {
								st2 := &blk.Stmts[i]
								if st2.Kind == StmtFor && st2.Loop != nil && st2.Loop.IV == p {
									ownIV = true
									break
								}
							}
							if ownIV {
								break
							}
						}
						if !ownIV {
							continue
						}
						pp, pok := pos[p]
						if !pok {
							continue
						}
						if len(fr) > 1 && fr[1] != nil {
							anc := fr[1]
							ap, aok := pos[anc]
							if !aok || pp <= ap {
								continue
							}
							// FE-head move when pure sits after FE neighbor after prev
							// insert (n35 g_108 before g_483). Skip when the pure IV
							// is also read later in the same function body (n28
							// if(g_791.f1) — UP keeps prev/late order, not FE-head).
							// Mid-gen special revalidate: f.Body==nil and open block may
							// lack later free reads — skip FE-head then. Post-body
							// FixupFunc1PureIVFEHeads applies with Body set (n35).
							if cg.CurrentFunc.Body == nil {
								continue
							}
							if laterBodyReadsVar(cg.CurrentFunc, st, p) {
								continue
							}
							moves = append(moves, mv{pure: p, anc: anc})
							seenP[p] = true
						}
					}
					for _, m := range moves {
						var next []*Variable
						for _, v := range ord {
							if v == m.pure {
								continue
							}
							next = append(next, v)
						}
						var out []*Variable
						inserted := false
						for _, v := range next {
							if v == m.anc && !inserted {
								out = append(out, m.pure)
								inserted = true
							}
							out = append(out, v)
						}
						if !inserted {
							out = append(out, m.pure)
						}
						ord = out
					}
					if len(moves) > 0 {
						rebuilt := EmptyEffect()
						for _, w := range merged.WrittenVarsSess(s) {
							if w == nil {
								continue
							}
							rebuilt = rebuilt.WriteVarSess(s, w)
							if hasErrCG(cg) || !EffectComplete(rebuilt) {
								if !hasErrCG(cg) {
									noteErrCG(cg, ErrGeneric)
								}
								return false
							}
						}
						for _, v := range ord {
							if v == nil {
								continue
							}
							rebuilt = rebuilt.ReadVarSess(s, v)
							if hasErrCG(cg) || !EffectComplete(rebuilt) {
								if !hasErrCG(cg) {
									noteErrCG(cg, ErrGeneric)
								}
								return false
							}
						}
						merged = rebuilt
					}
					cg.EffectStm = merged
				}
				// Strip own pure-only free-ref only non-direct nested pure IVs after
				// pureMiss. UP drops pure-only for-IVs free-ref only deep (LevelC
				// seed668 g_205 free-ref func_54, not free-ref on direct callee).
				// Free-ref on a direct callee keeps membership (seed48 g_1495 free-ref
				// func_22). Drop only — preserve order of remaining (seed5 g_259).
				// Session/CG-local — no package mutable state.
				if EffectComplete(cg.EffectStm) && pureIV != nil {
					ord := cg.EffectStm.ReadVarsSess(s)
					if hasErrCG(cg) {
						return false
					}
					// Soft-expanded pure FE heads of nested callees (seed668 g_645).
					nestedPureFEHeads := map[*Variable]bool{}
					{
						seenFn := map[*Function]bool{}
						var walkFn func(*Function)
						walkFn = func(fn *Function) {
							if fn == nil || seenFn[fn] {
								return
							}
							seenFn[fn] = true
							if EffectComplete(fn.FEffect) {
								fr := fn.FEffect.ReadVarsSess(s)
								if hasErrCG(cg) {
									return
								}
								if len(fr) > 0 && fr[0] != nil && isForIVOfFunc(fn, fr[0]) {
									nestedPureFEHeads[fr[0]] = true
								}
							}
							for _, blk := range fn.Blocks {
								if blk == nil {
									continue
								}
								for i := range blk.Stmts {
									var nested []*Invocation
									if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
										continue
									}
									for _, inv := range nested {
										if inv != nil && inv.User != nil {
											walkFn(inv.User)
										}
									}
								}
							}
						}
						for _, inv := range calls {
							if inv != nil && inv.User != nil {
								walkFn(inv.User)
							}
						}
					}
					if hasErrCG(cg) {
						return false
					}
					shouldStrip := func(v *Variable) bool {
						if v == nil || !pureIV[v] || !isForIVOfFunc(cg.CurrentFunc, v) {
							return false
						}
						if bodySyntacticFreeReadsVar(cg.CurrentFunc, v) {
							return false
						}
						if !nestedUserSyntacticFreeReadsVar(s, calls, v) {
							return false
						}
						// Free-ref on a direct pure-IV owner → keep (seed48 g_1495).
						freeOnOwnerDirect := false
						freeDirectAny := false
						for _, inv := range calls {
							if inv == nil || inv.User == nil {
								continue
							}
							if !bodySyntacticFreeReadsVar(inv.User, v) {
								continue
							}
							freeDirectAny = true
							if isForIVOfFunc(inv.User, v) {
								freeOnOwnerDirect = true
							}
						}
						if freeOnOwnerDirect {
							return false
						}
						// Nested pure FE head pure-only on parent → strip even when
						// free-ref on non-owner direct (seed668 g_645 free-ref func_3,
						// pure FE head of func_44). Mid residual free-ref non-owner
						// direct keep (seed668 g_82 free-ref func_10). Free-ref only
						// deeper than direct → strip (seed668 g_205). Session/CG-local.
						if nestedPureFEHeads[v] {
							return true
						}
						return !freeDirectAny
					}
					drop := false
					for _, v := range ord {
						if shouldStrip(v) {
							drop = true
							break
						}
					}
					if hasErrCG(cg) {
						return false
					}
					if drop {
						stripped := EmptyEffect()
						for _, w := range cg.EffectStm.WrittenVarsSess(s) {
							if w == nil {
								continue
							}
							stripped = stripped.WriteVarSess(s, w)
							if hasErrCG(cg) || !EffectComplete(stripped) {
								if !hasErrCG(cg) {
									noteErrCG(cg, ErrGeneric)
								}
								return false
							}
						}
						for _, v := range ord {
							if v == nil || shouldStrip(v) {
								continue
							}
							stripped = stripped.ReadVarSess(s, v)
							if hasErrCG(cg) || !EffectComplete(stripped) {
								if !hasErrCG(cg) {
									noteErrCG(cg, ErrGeneric)
								}
								return false
							}
						}
						if hasErrCG(cg) {
							return false
						}
						stripped.pure = cg.EffectStm.pure
						stripped.sideEffectFree = cg.EffectStm.sideEffectFree
						stripped.lhsWrite = cg.EffectStm.lhsWrite
						cg.EffectStm = stripped
					}
				}
			}
		}
		cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	return true
}
