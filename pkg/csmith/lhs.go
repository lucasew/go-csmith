// Upstream: Lhs.h / Lhs.cpp (make_random, Output via ExpressionVariable).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// Lhs mirrors Lhs : Expression — variable + desired type for * / bare / VOL_LVAL.
// Lhs.cpp:149–165.
type Lhs struct {
	Var              *Variable
	// Type is the desired LHS type (may differ from Var.Type by indirection).
	Type *Type
	// CompoundAssign mirrors for_compound_assign.
	CompoundAssign bool
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
func (l *Lhs) IndirectLevelComplete() (n int, ok bool) {
	if l == nil || l.Var == nil || l.Var.Type == nil {
		return 0, false
	}
	want := l.Type
	if want == nil {
		want = l.Var.Type
	}
	if want == nil {
		return 0, false
	}
	return l.Var.Type.IndirectLevel() - want.IndirectLevel(), true
}

// GetVar mirrors Lhs::get_var.
func (l *Lhs) GetVar() *Variable {
	if l == nil {
		return nil
	}
	return l.Var
}

// GetType mirrors Lhs::get_type.
func (l *Lhs) GetType() *Type {
	if l == nil {
		return nil
	}
	if l.Type != nil {
		return l.Type
	}
	if l.Var != nil {
		return l.Var.Type
	}
	return nil
}

// IsVolatile mirrors Lhs::is_volatile.
// Lhs.cpp:220–222 — volatile after deref of indirect level.
// Incomplete Lhs type IR fails closed as volatile (restrictive — no invent
// non-vol eligibility via invented level 0).
func (l *Lhs) IsVolatile() bool {
	if l == nil || l.Var == nil {
		return false
	}
	n, ok := l.IndirectLevelComplete()
	if !ok {
		return true
	}
	return l.Var.IsVolatileAfterDeref(n)
}

// GetQualifiers mirrors Lhs::get_qualifiers.
// Lhs.cpp:197–202 — var.qfer.indirect_qualifiers(indirect).
// Incomplete Lhs type IR fails closed sticky error + empty qfer (no invent
// storage-level quals via invented level 0).
func (l *Lhs) GetQualifiers() CVQualifiers {
	if l == nil || l.Var == nil {
		return CVQualifiers{}
	}
	n, ok := l.IndirectLevelComplete()
	if !ok {
		SetError(ErrGeneric)
		return CVQualifiers{}
	}
	q := l.Var.Qfer.IndirectQualifiers(n)
	// Lhs.cpp:200 — assert(!qfer.is_const()); const LHS is broken IR
	// sticky error for ERROR_GUARD callers; no soft invent strip of const
	if q.IsConst() {
		SetError(ErrGeneric)
	}
	return q
}

// GetLvars mirrors Lhs::get_lvars.
// Lhs.cpp:181–185 — merge pointees of var at indirect level.
// Incomplete Lhs type IR fails closed IncompleteVariables (no invent level-0 merge).
func (l *Lhs) GetLvars(facts []*FactPointTo) []*Variable {
	if l == nil || l.Var == nil {
		return IncompleteVariables()
	}
	n, ok := l.IndirectLevelComplete()
	if !ok {
		return IncompleteVariables()
	}
	return MergePointeesOfPointer(l.Var.GetCollective(), n, facts)
}

// GetReferencedPtrs mirrors Lhs::get_referenced_ptrs.
// Lhs.cpp:234–238 — pointer vars only.
// Incomplete Lhs/Var fails closed IncompleteVariables (not bare nil invent
// empty-complete "no ptrs" via VariablesComplete(nil)/len==0).
// Non-pointer live Var → complete empty nil.
func (l *Lhs) GetReferencedPtrs() []*Variable {
	if l == nil || l.Var == nil {
		return IncompleteVariables()
	}
	if !l.Var.IsPointer() {
		return nil
	}
	return []*Variable{l.Var}
}

// VisitIndices mirrors Lhs::visit_indices.
// Lhs.cpp:264–284 — visit array IndexExprs under RHS effect context
// (effect_context + effect_stm, null accum).
func (l *Lhs) VisitIndices(cg *CGContext, opts Options) bool {
	// Lhs.cpp:264+ — get_var()->get_array may be null → true without using cg
	if l == nil || l.Var == nil {
		return false
	}
	av := l.Var.AsArray
	if av == nil || len(av.IndexExprs) == 0 {
		// Lhs.cpp:267–268 — av == 0 → true (non-array / string-only Indices)
		return true
	}
	// need cg to visit Expression indices
	if cg == nil {
		return false
	}
	// Lhs.cpp:273–276 — combine context + stm as ambient; no accum
	eff := cg.EffectContext().AddEffect(cg.EffectStm)
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
	// incomplete IndexExprs — fail closed (no invent soft-skip nil index)
	if !ExpressionsComplete(av.IndexExprs) {
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
func (l *Lhs) CompatibleVar(v *Variable, expandStruct bool) bool {
	if l == nil || l.Var == nil {
		return false
	}
	return l.Var.Compatible(v, expandStruct)
}

// CompatibleExpr mirrors Lhs::compatible(Expression*).
// Lhs.cpp:359–362 — exp->compatible(&var).
func (l *Lhs) CompatibleExpr(exp *Expression, expandStruct bool) bool {
	if l == nil || l.Var == nil || exp == nil {
		return false
	}
	return exp.CompatibleWithVar(l.Var, expandStruct)
}

// Output mirrors Lhs::Output — ExpressionVariable shape, optional VOL_LVAL wrap.
// Lhs.cpp:207–218.
func (l *Lhs) Output(wrapVolatiles bool) string {
	if l == nil || l.Var == nil {
		return ""
	}
	// ExpressionVariable::Output for (var, type)
	ev := outputExpressionVariable(l.Var, l.Type)
	if wrapVolatiles && l.Var.IsVolatile() {
		// Lhs.cpp:211–216 — type->Output always live; no invent "int"
		t := l.GetType()
		if t == nil {
			return ""
		}
		ty := t.CName()
		if ty == "" || ev == "" {
			return ""
		}
		return "VOL_LVAL(" + ev + ", " + ty + ")"
	}
	return ev
}

// outputExpressionVariable mirrors ExpressionVariable::Output without cast.
// ExpressionVariable.cpp:202–219 — (*…)/& + Variable::Output.
func outputExpressionVariable(v *Variable, want *Type) string {
	if v == nil {
		return ""
	}
	ind := 0
	if v.Type != nil {
		wt := want
		if wt == nil {
			wt = v.Type
		}
		ind = v.Type.IndirectLevel() - wt.IndirectLevel()
	}
	base := v.OutputC()
	// ExpressionVariable always has live var Output; no invent "(***)" / "&" without base
	if base == "" {
		return ""
	}
	if ind > 0 {
		return "(" + strings.Repeat("*", ind) + base + ")"
	}
	if ind < 0 {
		// ExpressionVariable.cpp:210–212 — assert(indirect_level == -1)
		// multi-level & is broken IR; no soft invent single &
		if ind != -1 {
			return ""
		}
		// no invent bare "&" when get_actual_name empty
		nm := v.GetActualName(false)
		if nm == "" {
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
	// Lhs::make_random always receives a type from assign/factories (no GetIntType invent)
	if typ == nil || r == nil || vs == nil || cg == nil {
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
		// Lhs.cpp:77–87 — flipcoin SelectDerefPointerProb
		if v == nil {
			derefProb := 0
			if probs != nil {
				derefProb = probs.Single(PSelectDerefPointerProb)
			}
			if derefProb > 0 && r.RndFlipcoin(uint32(derefProb)) {
				v = selectDerefPointerInv(r, opts, probs, vs, *cg, typ, &q, AccessWrite, dummy)
			}
		}
		// Lhs.cpp:89–100 — select(WRITE, restricted qfer, dummy, eDerefExact)
		if v == nil {
			newQ := q
			// Lhs.cpp:90–93 — restrict unless wildcard
			if !newQ.Wildcard {
				newQ.Restrict(AccessWrite, *cg)
			}
			v = vs.SelectWithInvalid(AccessWrite, *cg, typ, &newQ, r, MatchDerefExact, dummy)
			// Lhs.cpp:94 — ERROR_GUARD(nullptr); select may create vars itself
		}
		if v == nil {
			// Lhs.cpp:101 assert(var) / ERROR_GUARD — no separate SelectGlobal soft path
			restore()
			continue
		}
		// Variable::type always live; incomplete type IR fails closed (no invent
		// skip const-after-deref filter and still accept the candidate)
		if v.Type == nil {
			dummy = append(dummy, v)
			restore()
			continue
		}
		// Lhs.cpp:85–86 / 97–99 — assert(!qfer.is_const_after_deref(deref_level))
		// select+restrict should exclude const WRITE; reject if violated
		deref := v.Type.IndirectLevel() - typ.IndirectLevel()
		if v.IsConstAfterDeref(deref) {
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
		}
		// Lhs.cpp:105 — !effect_stm.is_written(var)
		if valid && cg.EffectStm.IsWritten(v) {
			valid = false
		}
		// Lhs.cpp:110–113 — no_signed_overflow rejects signed base / bitfield for ++/--
		if valid && typ.IsSimple() && noSignedOverflow {
			base := v.Type
			if base != nil {
				base = base.BaseType()
			}
			if (base != nil && base.IsSigned()) || v.IsBitfield {
				valid = false
			}
		}
		// Lhs.cpp:114–116 — ccomp forbids bitfield assigned as long long
		if valid && opts.CComp && v.IsBitfield && typ.IsSimple() {
			switch typ.Simple() {
			case ELongLong, EULongLong:
				valid = false
			}
		}
		// Lhs.cpp:117–121 — float filters
		if valid && typ != nil && v.Type != nil {
			if !typ.IsFloat() && v.Type.IsFloat() {
				valid = false
			}
			if opts.StrictFloat && typ.IsFloat() && !v.Type.IsFloat() {
				valid = false
			}
		}
		if !valid {
			dummy = append(dummy, v)
			restore()
			continue
		}

		lhs := finishLhs(v, typ, compoundAssign, cg, opts)
		if lhs != nil {
			return lhs
		}
		// visit_facts failed — Lhs.cpp:135–139
		dummy = append(dummy, v)
		restore()
	}
	return nil
}

// finishLhs builds Lhs, optional visit_facts, records write dereference / volatile access.
// Lhs.cpp:106–140 — visit_facts then bookkeeping.
func finishLhs(v *Variable, typ *Type, compound bool, cg *CGContext, opts Options) *Lhs {
	if v == nil || cg == nil {
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
		if v.IsConst() {
			return
		}
		if compound && v.IsVolatile() {
			return
		}
		// expand fields for aggregates
		exp := v.CollectExpandable()
		if !VariablesComplete(exp) {
			incomplete = true
			return
		}
		for _, x := range exp {
			if x == nil || x.Type == nil {
				incomplete = true
				return
			}
			if x.IsConst() || !typ.Match(x.Type, MatchFlexible) {
				continue
			}
			if compound && x.IsVolatile() {
				continue
			}
			ok = append(ok, x)
		}
	}
	if cg.CurrentFunc != nil {
		for i := len(cg.CurrentFunc.Stack) - 1; i >= 0; i-- {
			blk := cg.CurrentFunc.Stack[i]
			// Block* always live on Stack; nil hole fails closed
			if blk == nil {
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
		for _, v := range vs.GlobalList {
			add(v)
		}
	}
	if incomplete {
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
		return nil
	}
	// VariableSelector.cpp:1249 — assert(qfer && qfer->sanity_check(type)); no invent
	if qfer == nil || !qfer.SanityCheck(typ) {
		return nil
	}
	// VariableSelector.cpp:1252–1266 — GlobalNonvolatilesList only (no GlobalList soft-fallback)
	var cands []*Variable
	if vs != nil {
		cands = append(cands, vs.GlobalNonvolatilesList...)
	}
	var blk *Block
	if cg.CurrentFunc != nil {
		if len(cg.CurrentFunc.Stack) > 0 {
			blk = cg.CurrentFunc.Stack[len(cg.CurrentFunc.Stack)-1]
		}
		for b := blk; b != nil; b = b.Parent {
			cands = append(cands, b.LocalVars...)
		}
		cands = append(cands, cg.CurrentFunc.Param...)
	}
	// VariableSelector.cpp:1264–1265 — choose_var eDereference
	if v := ChooseVarFull(r, cands, access, cg, typ, qfer, MatchDereference, invalidVars, false, false, false); v != nil {
		return v
	}

	// VariableSelector.cpp:1268–1272 — create ptr if under max_indirect_level
	if typ.IndirectLevel() >= opts.MaxPointerDepth {
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
		noVol := !cg.EffectContext().IsSideEffectFree()
		pq = qfer.RandomAddQualifiers(r, opts, probs, noVol)
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
