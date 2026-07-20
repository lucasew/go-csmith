// Upstream: Lhs.h / Lhs.cpp (make_random, Output via ExpressionVariable).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// Lhs mirrors Lhs : Expression — variable + desired type for * / bare / VOL_LVAL.
// Lhs.cpp:149–165.
type Lhs struct {
	Var *Variable
	// Type is the desired LHS type (may differ from Var.Type by indirection).
	Type *Type
	// CompoundAssign mirrors for_compound_assign.
	CompoundAssign bool
}

// Clone mirrors Lhs::clone.
// Lhs.cpp:174 — new Lhs(*this).
// Incomplete Lhs sticky nil (no invent empty Lhs shell).
func (l *Lhs) Clone() *Lhs {
	if l == nil || l.Var == nil {
		SetError(ErrGeneric)
		return nil
	}
	cp := *l
	return &cp
}

// GetComplexity mirrors Expression::get_complexity for Lhs — ExpressionVariable leaf.
// ExpressionVariable is complexity 0 (Bookkeeper ExpressionComplexity TermVariable).
func (l *Lhs) GetComplexity() int {
	if l == nil || l.Var == nil {
		SetError(ErrGeneric)
		return -1
	}
	return 0
}

// GetDereferencedPtrs mirrors Lhs::get_dereferenced_ptrs.
// Lhs.cpp:225–232 — self ExpressionVariable when indirect_level > 0.
// Incomplete Lhs sticky IncompleteExpressions.
func (l *Lhs) GetDereferencedPtrs() []*Expression {
	if l == nil || l.Var == nil {
		SetError(ErrGeneric)
		return IncompleteExpressions()
	}
	n, ok := l.IndirectLevelComplete()
	if !ok {
		return IncompleteExpressions()
	}
	if n <= 0 {
		return []*Expression{}
	}
	ty := l.Type
	if ty == nil {
		ty = l.Var.Type
	}
	return []*Expression{{Term: TermVariable, Var: l.Var, ExprType: ty}}
}

// PtrModifiedInRhs mirrors Lhs::ptr_modified_in_rhs via CGContext.
// Lhs.cpp:233–257.
func (l *Lhs) PtrModifiedInRhs(cg *CGContext, facts []*FactPointTo) bool {
	if l == nil || cg == nil {
		SetError(ErrGeneric)
		return true // fail closed as modified
	}
	return cg.PtrModifiedInRhs(l, facts)
}

// IndirectLevel mirrors Lhs::get_indirect_level.
// Lhs.cpp:190–192 — var.type.indirect - type.indirect.
// Incomplete Lhs IR (nil var/type) returns 0 for the bit; callers that must not
// invent non-deref visit success use IndirectLevelComplete.
func (l *Lhs) IndirectLevel() int {
	n, ok := l.IndirectLevelComplete()
	if !ok {
		return 0
	}
	return n
}

// IndirectLevelComplete is get_indirect_level with ok=false on incomplete Lhs IR
// (no invent treat broken Lhs as bare non-deref level 0 for visit/validate).
// Incomplete shell sticky (callers that only use IndirectLevel still surface ERROR).
func (l *Lhs) IndirectLevelComplete() (n int, ok bool) {
	// Lhs always live with Variable+Type; sticky incomplete no invent level 0 complete
	if l == nil || l.Var == nil || l.Var.Type == nil {
		SetError(ErrGeneric)
		return 0, false
	}
	want := l.Type
	if want == nil {
		want = l.Var.Type
	}
	if want == nil {
		SetError(ErrGeneric)
		return 0, false
	}
	lv := l.Var.Type.IndirectLevel()
	// residual ERROR sticky — no invent level-0 past subject IndirectLevel residual
	if HasError() {
		return 0, false
	}
	lw := want.IndirectLevel()
	// residual ERROR sticky — no invent level-0 past desired IndirectLevel residual
	if HasError() {
		return 0, false
	}
	return lv - lw, true
}

// GetVar mirrors Lhs::get_var.
func (l *Lhs) GetVar() *Variable {
	// Lhs always live; sticky no invent missing subject shell
	if l == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete without Var sticky (no invent soft-skip missing lhs subject)
	if l.Var == nil {
		SetError(ErrGeneric)
		return nil
	}
	return l.Var
}

// GetType mirrors Lhs::get_type.
func (l *Lhs) GetType() *Type {
	// Lhs always live; sticky no invent type shell without it
	if l == nil {
		SetError(ErrGeneric)
		return nil
	}
	if l.Type != nil {
		return l.Type
	}
	if l.Var != nil && l.Var.Type != nil {
		return l.Var.Type
	}
	// incomplete Lhs type IR sticky — no invent nil type soft-success
	SetError(ErrGeneric)
	return nil
}

// IsVolatile mirrors Lhs::is_volatile.
// Lhs.cpp:220–222 — volatile after deref of indirect level.
// Incomplete Lhs type IR fails closed sticky as volatile (restrictive — no invent
// non-vol eligibility via invented level 0 / soft re-pick).
func (l *Lhs) IsVolatile() bool {
	// Lhs always live; sticky incomplete fails closed true (restrictive volatile)
	// (no invent non-vol eligibility / soft re-pick past hole)
	if l == nil || l.Var == nil {
		SetError(ErrGeneric)
		return true
	}
	n, ok := l.IndirectLevelComplete()
	if !ok {
		SetError(ErrGeneric)
		return true
	}
	vol := l.Var.IsVolatileAfterDeref(n)
	// residual ERROR sticky — no invent non-vol soft-skip past IsVolatileAfterDeref residual
	if HasError() {
		return true
	}
	return vol
}

// GetQualifiers mirrors Lhs::get_qualifiers.
// Lhs.cpp:197–202 — var.qfer.indirect_qualifiers(indirect).
// Incomplete Lhs type IR fails closed sticky error + empty qfer (no invent
// storage-level quals via invented level 0).
func (l *Lhs) GetQualifiers() CVQualifiers {
	// Lhs always live; sticky incomplete empty qfer (no invent storage quals)
	if l == nil || l.Var == nil {
		SetError(ErrGeneric)
		return CVQualifiers{}
	}
	n, ok := l.IndirectLevelComplete()
	if !ok {
		SetError(ErrGeneric)
		return CVQualifiers{}
	}
	q := l.Var.Qfer.IndirectQualifiers(n)
	// residual ERROR sticky — no invent soft-quals past IndirectQualifiers residual
	if HasError() {
		return CVQualifiers{}
	}
	// Lhs.cpp:200 — assert(!qfer.is_const()); const LHS is broken IR
	// sticky error for ERROR_GUARD callers; no soft invent strip of const / invent quals shell
	if q.IsConst() {
		// residual ERROR sticky — no invent soft-quals past IsConst residual true
		if HasError() {
			return CVQualifiers{}
		}
		SetError(ErrGeneric)
		return CVQualifiers{}
	}
	// residual ERROR sticky — no invent soft-complete quals past IsConst residual false
	if HasError() {
		return CVQualifiers{}
	}
	return q
}

// GetLvars mirrors Lhs::get_lvars.
// Lhs.cpp:181–185 — merge pointees of var at indirect level.
// Incomplete Lhs type IR fails closed IncompleteVariables (no invent level-0 merge).
func (l *Lhs) GetLvars(facts []*FactPointTo) []*Variable {
	if l == nil || l.Var == nil {
		// incomplete Lhs fails closed sticky (no invent empty pointees / soft re-pick)
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	n, ok := l.IndirectLevelComplete()
	if !ok {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	coll := l.Var.GetCollective()
	// residual ERROR sticky — no invent soft-merge past GetCollective residual hole
	if HasError() {
		return IncompleteVariables()
	}
	vars := MergePointeesOfPointer(coll, n, facts)
	// residual ERROR sticky — no invent soft-merge past MergePointees residual hole
	if HasError() {
		return IncompleteVariables()
	}
	return vars
}

// GetReferencedPtrs mirrors Lhs::get_referenced_ptrs.
// Lhs.cpp:234–238 — pointer vars only.
// Incomplete Lhs/Var fails closed IncompleteVariables (not bare nil invent
// empty-complete "no ptrs" via VariablesComplete(nil)/len==0).
// Type* always live for non-special Var; Type-nil sticky IncompleteVariables
// (IsPointer residual ERROR+false invents complete empty no-ptrs past shell).
// Non-pointer live Var → complete empty nil.
func (l *Lhs) GetReferencedPtrs() []*Variable {
	if l == nil || l.Var == nil {
		// incomplete Lhs fails closed sticky (no invent empty-complete "no ptrs")
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	// Type* always live for non-special subjects; Type-nil sticky incomplete
	// (no invent IsPointer residual false as complete empty no-ptrs)
	if l.Var.Type == nil && !IsSpecialPtr(l.Var) {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	if !l.Var.IsPointer() {
		// residual ERROR sticky — no invent soft-empty no-ptrs past IsPointer residual false
		if HasError() {
			return IncompleteVariables()
		}
		return nil
	}
	// residual ERROR sticky — no invent ptr list past IsPointer residual true path
	if HasError() {
		return IncompleteVariables()
	}
	return []*Variable{l.Var}
}

// VisitIndices mirrors Lhs::visit_indices.
// Lhs.cpp:264–284 — visit array IndexExprs under RHS effect context
// (effect_context + effect_stm, null accum).
// Incomplete Lhs / ambient / IndexExprs sticky (no invent soft-skip past holes).
// IsArray without AsArray hard IR sticky false (no invent visit success as
// "no array indices" past broken array shell — mirrors ReadIndices).
func (l *Lhs) VisitIndices(cg *CGContext, opts Options) bool {
	// Lhs.cpp:264+ — get_var()->get_array may be null → true without using cg
	// incomplete Lhs shell sticky (visit always has live Lhs* in C++)
	if l == nil || l.Var == nil {
		SetError(ErrGeneric)
		return false
	}
	// C++ static_cast ArrayVariable* when isArray; missing AsArray is broken IR
	// sticky (no invent complete true soft-skip past IsArray without AsArray)
	if l.Var.IsArray && l.Var.AsArray == nil {
		SetError(ErrGeneric)
		return false
	}
	av := l.Var.AsArray
	if av == nil || len(av.IndexExprs) == 0 {
		// Lhs.cpp:267–268 — av == 0 → true (non-array / string-only Indices)
		return true
	}
	// need cg to visit Expression indices; nil cg sticky hard IR
	if cg == nil {
		SetError(ErrGeneric)
		return false
	}
	// Lhs.cpp:273–276 — combine context + stm as ambient; no accum
	// Incomplete ambient sticky (no invent index visit under incomplete context)
	eff := cg.EffectContext().AddEffect(cg.EffectStm)
	// residual ERROR sticky — no invent soft-continue index visit past AddEffect residual
	if HasError() {
		return false
	}
	if !EffectComplete(eff) {
		SetError(ErrGeneric)
		return false
	}
	rhsCG := CGContext{
		effectContext: eff,
		CurrentFunc:   cg.CurrentFunc,
		BlkDepth:      cg.BlkDepth,
		Flags:         cg.Flags,
		ExprDepth:     cg.ExprDepth,
		Funcs:         cg.Funcs,
		Types:         cg.Types,
		FM:            cg.FM,
		RW:            cg.RW,
		IVBounds:      cg.IVBounds,
		CallChain:     cg.CallChain,
	}
	// incomplete IndexExprs sticky (no invent soft-skip nil index)
	if !ExpressionsComplete(av.IndexExprs) {
		SetError(ErrGeneric)
		return false
	}
	for _, e := range av.IndexExprs {
		// Lhs.cpp:278–280 — get_indices()[i] always live Expression*
		if !VisitFactsExpression(e, &rhsCG, opts) {
			return false
		}
	}
	return true
}

// CompatibleVar mirrors Lhs::compatible(Variable*).
// Lhs.cpp:364.
// Lhs + Var always live; sticky false (no invent not-compatible soft-skip past hole).
func (l *Lhs) CompatibleVar(v *Variable, expandStruct bool) bool {
	if l == nil || l.Var == nil {
		SetError(ErrGeneric)
		return false
	}
	ok := l.Var.Compatible(v, expandStruct)
	// residual ERROR sticky — no invent soft-compat past Compatible residual
	if HasError() {
		return false
	}
	return ok
}

// CompatibleExpr mirrors Lhs::compatible(Expression*).
// Lhs.cpp:359–362 — exp->compatible(&var).
// Lhs + Var + Expression always live; sticky false (no invent not-compatible soft-skip).
func (l *Lhs) CompatibleExpr(exp *Expression, expandStruct bool) bool {
	if l == nil || l.Var == nil || exp == nil {
		SetError(ErrGeneric)
		return false
	}
	return exp.CompatibleWithVar(l.Var, expandStruct)
}

// Output mirrors Lhs::Output — ExpressionVariable shape, optional VOL_LVAL wrap.
// Lhs.cpp:207–218.
func (l *Lhs) Output(wrapVolatiles bool) string {
	// Lhs always live with Var at emit; sticky no invent empty LHS without them
	if l == nil || l.Var == nil {
		SetError(ErrGeneric)
		return ""
	}
	// ExpressionVariable::Output for (var, type)
	ev := outputExpressionVariable(l.Var, l.Type)
	// residual ERROR sticky — no invent soft-empty LHS past outputExpressionVariable residual
	if HasError() {
		return ""
	}
	if wrapVolatiles && l.Var.IsVolatile() {
		// residual ERROR sticky — no invent soft-wrap past IsVolatile residual hole
		if HasError() {
			return ""
		}
		// Lhs.cpp:211–216 — type->Output always live; sticky no invent "int"
		t := l.GetType()
		// residual ERROR sticky — no invent soft-wrap past GetType residual hole
		if HasError() {
			return ""
		}
		if t == nil {
			SetError(ErrGeneric)
			return ""
		}
		ty := t.CName()
		// residual ERROR sticky — no invent soft-wrap past CName residual hole
		if HasError() {
			return ""
		}
		if ty == "" || ev == "" {
			SetError(ErrGeneric)
			return ""
		}
		return "VOL_LVAL(" + ev + ", " + ty + ")"
	}
	// residual ERROR sticky — no invent bare LHS past IsVolatile residual false path
	if HasError() {
		return ""
	}
	return ev
}

// outputExpressionVariable mirrors ExpressionVariable::Output without cast.
// ExpressionVariable.cpp:202–219 — (*…)/& + Variable::Output.
func outputExpressionVariable(v *Variable, want *Type) string {
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	ind := 0
	if v.Type != nil {
		wt := want
		if wt == nil {
			wt = v.Type
		}
		vi := v.Type.IndirectLevel()
		// residual ERROR sticky — no invent soft-level past IndirectLevel residual hole
		if HasError() {
			return ""
		}
		wi := wt.IndirectLevel()
		// residual ERROR sticky — no invent soft-level past want IndirectLevel residual
		if HasError() {
			return ""
		}
		ind = vi - wi
	}
	base := v.OutputC()
	// residual ERROR sticky — no invent soft-empty base past OutputC residual hole
	if HasError() {
		return ""
	}
	// ExpressionVariable always has live var Output; sticky no invent "(***)" / "&" without base
	if base == "" {
		SetError(ErrGeneric)
		return ""
	}
	if ind > 0 {
		return "(" + strings.Repeat("*", ind) + base + ")"
	}
	if ind < 0 {
		// ExpressionVariable.cpp:210–212 — assert(indirect_level == -1)
		// multi-level & is broken IR sticky; no soft invent single &
		if ind != -1 {
			SetError(ErrGeneric)
			return ""
		}
		// sticky no invent bare "&" when get_actual_name empty
		nm := v.GetActualName(false)
		// residual ERROR sticky — no invent soft-empty & past GetActualName residual
		if HasError() {
			return ""
		}
		if nm == "" {
			SetError(ErrGeneric)
			return ""
		}
		return "&" + nm
	}
	return base
}

// MakeRandomLhs mirrors Lhs::make_random.
// Lhs.cpp:58–143 — must_use / select_deref_pointer / select(eDerefExact) with dummy
// invalid_vars; no_signed_overflow, ccomp bitfield, float filters; visit_facts.
// qfer is StatementAssign-built qualifiers (may be wildcard); nil → non-const WRITE base.
// noSignedOverflow is StatementAssign::need_no_rhs(op) at call sites.
// MakeRandomLhs mirrors Lhs::make_random.
// Pass *CGContext so visit_facts mutations (effect_stm / accum) persist for the caller
// (StatementAssign merges lhs context after selection).
func MakeRandomLhs(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	cg *CGContext,
	typ *Type,
	compoundAssign bool,
	noSignedOverflow bool,
	qfer *CVQualifiers,
) *Lhs {
	// Lhs::make_random always receives type + RNG + VS + CG; sticky no invent LHS shell without them
	if typ == nil || r == nil || vs == nil || cg == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent LHS / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// Lhs.cpp:qfer from caller; default non-const non-vol storage
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if qfer != nil {
		q = *qfer
	}

	// Lhs.cpp:67–69 — save effects for visit_facts backtrack
	var accumSave *Effect
	if cg.EffectAccum != nil {
		cp := *cg.EffectAccum
		accumSave = &cp
	}
	stmSave := cg.EffectStm

	restore := func() {
		// Lhs.cpp:135–139 — reset_effect_accum + reset_effect_stm
		if accumSave != nil && cg.EffectAccum != nil {
			*cg.EffectAccum = *accumSave
		}
		cg.EffectStm = stmSave
	}

	// Lhs.cpp:63, 70–140 — do { DEPTH_GUARD; select; filters; visit } while (!lhs)
	// C++ loops until success or ERROR_GUARD; cap high to avoid soft invent nil early
	var dummy []*Variable
	for tries := 0; tries < 256; tries++ {
		// Lhs.cpp:71 — DEPTH_GUARD_BY_TYPE_RETURN(dtLhs, nullptr) inside the do-loop
		if DepthGuardByType(opts, DtLhs) == BadDepth {
			return nil
		}
		if HasError() {
			return nil
		}
		var v *Variable
		// Lhs.cpp:73–76 — try must_use WRITE first
		v = vs.SelectMustUseVar(r, AccessWrite, *cg, typ, &q)
		// residual ERROR sticky — no invent fall through to soft select past must-use hole
		if HasError() {
			restore()
			return nil
		}
		// Lhs.cpp:77–87 — flipcoin SelectDerefPointerProb
		if v == nil {
			derefProb := 0
			if probs != nil {
				derefProb = probs.Single(PSelectDerefPointerProb)
			}
			if derefProb > 0 && r.RndFlipcoin(uint32(derefProb)) {
				v = selectDerefPointerInv(r, opts, probs, vs, *cg, typ, &q, AccessWrite, dummy)
				// residual ERROR sticky — no invent soft-continue past deref select hole
				if HasError() {
					restore()
					return nil
				}
			}
		}
		// Lhs.cpp:89–100 — select(WRITE, restricted qfer, dummy, eDerefExact)
		if v == nil {
			newQ := q
			// Lhs.cpp:90–93 — restrict unless wildcard
			if !newQ.Wildcard {
				newQ.Restrict(AccessWrite, *cg)
			}
			// residual Restrict sticky — no invent soft select past qfer hole
			if HasError() {
				restore()
				return nil
			}
			v = vs.SelectWithInvalid(AccessWrite, *cg, typ, &newQ, r, MatchDerefExact, dummy)
			// Lhs.cpp:94 — ERROR_GUARD(nullptr); select may create vars itself
			// residual ERROR sticky — no invent soft-continue / create past select hole
			if HasError() {
				restore()
				return nil
			}
		}
		if v == nil {
			// Lhs.cpp:101 assert(var) / ERROR_GUARD — no separate SelectGlobal soft path
			// complete soft miss (no residual): restore and re-pick
			restore()
			continue
		}
		// Variable::type always live; incomplete type IR fails closed sticky
		// (no invent soft-skip const-after-deref filter then accept later create)
		if v.Type == nil {
			SetError(ErrGeneric)
			restore()
			return nil
		}
		// C++ isArray always ArrayVariable*; missing AsArray sticky
		// (no invent WRITE Lhs past broken array shell)
		if v.IsArray && v.AsArray == nil {
			SetError(ErrGeneric)
			restore()
			return nil
		}
		// Lhs.cpp:85–86 / 97–99 — assert(!qfer.is_const_after_deref(deref_level))
		// select+restrict should exclude const WRITE; reject if violated
		deref := v.Type.IndirectLevel() - typ.IndirectLevel()
		if v.IsConstAfterDeref(deref) {
			// residual ERROR sticky — no invent soft-continue past incomplete const peel
			if HasError() {
				restore()
				return nil
			}
			dummy = append(dummy, v)
			restore()
			continue
		}

		// Lhs.cpp:103–122 — validity filters before visit_facts
		valid := true
		if cg.FM != nil {
			if OpportunisticValidate(r, v, typ, cg.FM.GlobalFacts, opts.NullPointerDerefProb, opts.DeadPointerDerefProb) == 0 {
				valid = false
			}
			// residual ERROR sticky — no invent soft-continue past validate hole
			if HasError() {
				restore()
				return nil
			}
		}
		// Lhs.cpp:105 — !effect_stm.is_written(var)
		if valid && cg.EffectStm.IsWritten(v) {
			valid = false
		}
		// residual ERROR sticky — no invent soft-continue past IsWritten hole
		if HasError() {
			restore()
			return nil
		}
		// Lhs.cpp:110–113 — no_signed_overflow rejects signed base / bitfield for ++/--
		if valid && typ.IsSimple() && noSignedOverflow {
			// residual ERROR sticky — no invent soft-continue past IsSimple residual
			if HasError() {
				restore()
				return nil
			}
			base := v.Type
			if base != nil {
				base = base.BaseType()
			}
			// residual ERROR sticky — no invent soft-continue past BaseType residual
			if HasError() {
				restore()
				return nil
			}
			if base != nil {
				if base.IsSigned() {
					// residual ERROR sticky — no invent soft-continue past IsSigned residual true
					if HasError() {
						restore()
						return nil
					}
					valid = false
				} else if HasError() {
					// residual ERROR sticky — no invent soft-continue past IsSigned residual false
					restore()
					return nil
				}
			}
			if valid && v.IsBitfield {
				valid = false
			}
		} else if HasError() {
			// residual ERROR sticky — no invent soft-continue past IsSimple residual false
			restore()
			return nil
		}
		// Lhs.cpp:114–116 — ccomp forbids bitfield assigned as long long
		if valid && opts.CComp && v.IsBitfield && typ.IsSimple() {
			// residual ERROR sticky — no invent soft-continue past IsSimple residual
			if HasError() {
				restore()
				return nil
			}
			switch typ.Simple() {
			case ELongLong, EULongLong:
				valid = false
			}
		} else if HasError() {
			restore()
			return nil
		}
		// Lhs.cpp:117–121 — float filters
		if valid && typ != nil && v.Type != nil {
			if !typ.IsFloat() && v.Type.IsFloat() {
				// residual ERROR sticky — no invent soft-continue past IsFloat residual
				if HasError() {
					restore()
					return nil
				}
				valid = false
			} else if HasError() {
				restore()
				return nil
			}
			if valid && opts.StrictFloat && typ.IsFloat() && !v.Type.IsFloat() {
				if HasError() {
					restore()
					return nil
				}
				valid = false
			} else if HasError() {
				restore()
				return nil
			}
		}
		if !valid {
			dummy = append(dummy, v)
			restore()
			continue
		}

		lhs := finishLhs(v, typ, compoundAssign, cg, opts)
		// residual ERROR sticky — no invent soft-continue past visit_facts hard IR hole
		if HasError() {
			restore()
			return nil
		}
		if lhs != nil {
			return lhs
		}
		// visit_facts failed soft — Lhs.cpp:135–139
		dummy = append(dummy, v)
		restore()
	}
	return nil
}

// finishLhs builds Lhs, optional visit_facts, records write dereference / volatile access.
// Lhs.cpp:106–140 — visit_facts then bookkeeping.
func finishLhs(v *Variable, typ *Type, compound bool, cg *CGContext, opts Options) *Lhs {
	// Lhs always has live var + CG after select; sticky no invent LHS shell without them
	if v == nil || cg == nil {
		SetError(ErrGeneric)
		return nil
	}
	lhs := &Lhs{Var: v, Type: typ, CompoundAssign: compound}
	// Lhs.cpp:122–140 — visit_facts when FactMgr present; fail → caller retries
	if cg.FM != nil {
		if !cg.VisitFactsLhs(lhs, opts) {
			return nil
		}
	}
	// Lhs.cpp:132–140 — bookkeeping on successful make
	// VisitFactsLhs already required complete Lhs; still use Complete for safety
	deref, _ := lhs.IndirectLevelComplete()
	if deref > 0 {
		IncrCounter(&writeDereferenceCnts, deref)
	}
	RecordVolatileAccess(v, deref, true)
	// wrap volatiles for OutputLhsC path on Variable
	if opts.WrapVolatiles {
		v.UseVolRVal = true
	}
	return lhs
}

// selectWritable gathers non-const matching variables from stack, params, globals.
// Stack/Param/Global Variable* always live; nil holes fail closed (nil select).
func selectWritable(r *Rng, vs *VariableSelector, cg CGContext, typ *Type, compound bool) *Variable {
	// want Type always live for Match; sticky no invent empty pool / soft re-pick past hole
	if typ == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before pool scan (no invent soft re-pick)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	var ok []*Variable
	incomplete := false
	add := func(v *Variable) {
		if incomplete {
			return
		}
		// Variable* always live in stack/params/globals
		if v == nil || v.Type == nil {
			incomplete = true
			return
		}
		// IsConst / IsVolatile residual only on nil (v already gated); complete path never stickies
		if v.IsConst() {
			// residual ERROR sticky — no invent soft-skip const past IsConst residual true
			if HasError() {
				incomplete = true
			}
			return
		}
		// residual ERROR sticky — no invent soft-continue past IsConst residual false
		if HasError() {
			incomplete = true
			return
		}
		if compound && v.IsVolatile() {
			// residual ERROR sticky — no invent soft-skip vol past IsVolatile residual true
			if HasError() {
				incomplete = true
			}
			return
		}
		// residual ERROR sticky — no invent soft-continue past IsVolatile residual false
		if HasError() {
			incomplete = true
			return
		}
		// expand fields for aggregates
		exp := v.CollectExpandable()
		// residual ERROR sticky — no invent soft-expand past CollectExpandable residual
		if HasError() {
			incomplete = true
			return
		}
		if !VariablesComplete(exp) {
			incomplete = true
			return
		}
		for _, x := range exp {
			if x == nil || x.Type == nil {
				incomplete = true
				return
			}
			if x.IsConst() {
				// residual ERROR sticky — no invent soft-continue then pick later past IsConst hole
				if HasError() {
					incomplete = true
					return
				}
				continue
			}
			if !typ.Match(x.Type, MatchFlexible) {
				// residual ERROR sticky — no invent soft-continue then pick later past Match hole
				if HasError() {
					incomplete = true
					return
				}
				continue
			}
			if compound && x.IsVolatile() {
				// residual ERROR sticky — no invent soft-continue then pick later past IsVolatile hole
				if HasError() {
					incomplete = true
					return
				}
				continue
			}
			ok = append(ok, x)
		}
	}
	if cg.CurrentFunc != nil {
		// incomplete Stack fails closed sticky (no invent soft-skip nil frame)
		if !BlocksComplete(cg.CurrentFunc.Stack) {
			SetError(ErrGeneric)
			return nil
		}
		if !VariablesComplete(cg.CurrentFunc.Param) {
			SetError(ErrGeneric)
			return nil
		}
		for i := len(cg.CurrentFunc.Stack) - 1; i >= 0; i-- {
			blk := cg.CurrentFunc.Stack[i]
			// pre-validated BlocksComplete
			if !VariablesComplete(blk.LocalVars) {
				SetError(ErrGeneric)
				return nil
			}
			for _, v := range blk.LocalVars {
				add(v)
			}
		}
		for _, v := range cg.CurrentFunc.Param {
			add(v)
		}
	}
	if vs != nil {
		if !VariablesComplete(vs.GlobalList) {
			SetError(ErrGeneric)
			return nil
		}
		for _, v := range vs.GlobalList {
			add(v)
		}
	}
	if incomplete {
		// incomplete expand/type IR fails closed sticky (no invent soft re-pick past hole)
		SetError(ErrGeneric)
		return nil
	}
	return ChooseOKVar(r, ok)
}

// selectDerefPointer mirrors VariableSelector::select_deref_pointer.
// VariableSelector.cpp:1246–1318 — nonvol globals+locals+params, eDereference;
// else create ptr with random_add_qualifiers / random_qualifiers.
func selectDerefPointer(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	access Access,
) *Variable {
	return selectDerefPointerInv(r, opts, probs, vs, cg, typ, qfer, access, nil)
}

// selectDerefPointerInv is select_deref_pointer with invalid_vars.
// Type + RNG always live; sticky nil (no invent soft-skip deref select past hole).
func selectDerefPointerInv(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	access Access,
	invalidVars []*Variable,
) *Variable {
	if typ == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before choose/create (no invent soft re-pick)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1249 — assert(qfer && qfer->sanity_check(type)); sticky no invent
	if qfer == nil {
		SetError(ErrGeneric)
		return nil
	}
	if !qfer.SanityCheck(typ) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return nil
	}
	// residual ERROR sticky — no invent soft-create past SanityCheck residual true path
	if HasError() {
		return nil
	}
	// incomplete invalid_vars fails closed sticky
	if !VariablesComplete(invalidVars) {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1252–1266 — GlobalNonvolatilesList only (no GlobalList soft-fallback)
	var cands []*Variable
	if vs != nil {
		if !VariablesComplete(vs.GlobalNonvolatilesList) {
			SetError(ErrGeneric)
			return nil
		}
		cands = append(cands, vs.GlobalNonvolatilesList...)
	}
	var blk *Block
	if cg.CurrentFunc != nil {
		if !BlocksComplete(cg.CurrentFunc.Stack) {
			SetError(ErrGeneric)
			return nil
		}
		if !VariablesComplete(cg.CurrentFunc.Param) {
			SetError(ErrGeneric)
			return nil
		}
		if len(cg.CurrentFunc.Stack) > 0 {
			blk = cg.CurrentFunc.Stack[len(cg.CurrentFunc.Stack)-1]
		}
		for b := blk; b != nil; b = b.Parent {
			if !VariablesComplete(b.LocalVars) {
				SetError(ErrGeneric)
				return nil
			}
			cands = append(cands, b.LocalVars...)
		}
		cands = append(cands, cg.CurrentFunc.Param...)
	}
	// VariableSelector.cpp:1264–1265 — choose_var eDereference
	if v := ChooseVarFull(r, cands, access, cg, typ, qfer, MatchDereference, invalidVars, false, false, false); v != nil {
		return v
	}

	// VariableSelector.cpp:1268–1272 — create ptr if under max_indirect_level
	if typ == nil {
		SetError(ErrGeneric)
		return nil
	}
	il := typ.IndirectLevel()
	// residual ERROR sticky — no invent soft-create ptr past IndirectLevel residual
	if HasError() {
		return nil
	}
	if il >= opts.MaxPointerDepth {
		return nil
	}
	var ptrType *Type
	if vs != nil && vs.Types != nil {
		ptrType = vs.Types.FindPointerType(typ, true)
	} else {
		ptrType = PointerTo(typ)
	}
	if ptrType == nil {
		return nil
	}
	// VariableSelector.cpp:1274–1285 — ptr_qfer
	var pq CVQualifiers
	if qfer.Wildcard || !opts.GlobalVariables {
		pq = RandomQualifiersDefaultProbs(ptrType, access, cg, true, opts, probs, r)
	} else {
		// random_add_qualifiers(!SE-free)
		seFree := cg.EffectContext().IsSideEffectFree()
		// residual ERROR sticky — no invent soft-no-vol RandomAdd past IsSideEffectFree residual
		if HasError() {
			return nil
		}
		pq = qfer.RandomAddQualifiers(r, opts, probs, !seFree)
	}
	// VariableSelector.cpp:1281 ERROR_GUARD after random_add/random_qualifiers
	if HasError() {
		return nil
	}
	pq.AcceptStricter = false
	if access == AccessWrite {
		// VariableSelector.cpp:1283–1285 — set_const(false, 1)
		pq.SetConst(false, 1)
	}
	if vs == nil {
		return nil
	}
	// VariableSelector.cpp:1286–1316 — expand_struct fail → Error::set_error, no Generate fallthrough
	if vs.Opts.ExpandStruct {
		if pq.IsVolatile() {
			v := vs.EagerCreateGlobalStruct(access, cg, ptrType, &pq, r, MatchDereference, invalidVars)
			if HasError() {
				return nil
			}
			if v != nil {
				return v
			}
			SetError(ErrGeneric)
			return nil
		}
		if blk == nil {
			// C++ GenerateNewParentLocal(*block) with null block — fail closed
			return nil
		}
		v := vs.EagerCreateLocalStruct(blk, access, cg, ptrType, &pq, r, MatchDereference, invalidVars)
		if HasError() {
			return nil
		}
		if v != nil {
			return v
		}
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1286–1315 non-expand: volatile global else parent local
	if pq.IsVolatile() {
		v := vs.GenerateNewGlobal(access, cg, ptrType, &pq, r)
		if HasError() {
			return nil
		}
		return v
	}
	if blk == nil {
		// no soft invent global when current block missing
		return nil
	}
	v := vs.GenerateNewParentLocal(blk, access, cg, ptrType, &pq, r)
	// VariableSelector.cpp:1318 ERROR_GUARD
	if HasError() {
		return nil
	}
	return v
}
