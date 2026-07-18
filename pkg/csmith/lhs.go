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
func (l *Lhs) IndirectLevel() int {
	if l == nil || l.Var == nil || l.Var.Type == nil {
		return 0
	}
	want := l.Type
	if want == nil {
		want = l.Var.Type
	}
	return l.Var.Type.IndirectLevel() - want.IndirectLevel()
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
// Lhs.cpp:220–222.
func (l *Lhs) IsVolatile() bool {
	if l == nil || l.Var == nil {
		return false
	}
	return l.Var.IsVolatile()
}

// CompatibleVar mirrors Lhs::compatible(Variable*).
// Lhs.cpp:364.
func (l *Lhs) CompatibleVar(v *Variable, expandStruct bool) bool {
	if l == nil || l.Var == nil {
		return false
	}
	return l.Var.Compatible(v, expandStruct)
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
		ty := "int"
		if t := l.GetType(); t != nil {
			ty = t.CName()
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
	if ind > 0 {
		return "(" + strings.Repeat("*", ind) + base + ")"
	}
	if ind < 0 {
		// only -1 is valid for address-of
		return "&" + v.GetActualName(false)
	}
	return base
}

// MakeRandomLhs mirrors Lhs::make_random without full visit_facts retry loop.
// Lhs.cpp:58–147 — must_use, SelectDerefPointerProb, select eDerefExact, bookkeeping.
// Returns Lhs (var + desired type) or nil.
func MakeRandomLhs(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	cg CGContext,
	typ *Type,
	compoundAssign bool,
) *Lhs {
	if typ == nil {
		typ = GetIntType()
	}
	if r == nil || vs == nil {
		return nil
	}
	// non-const WRITE qualifiers + restrict (Lhs.cpp:111–116)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	q.Restrict(AccessWrite, cg)

	// Lhs.cpp:73–76 — try must_use WRITE first
	if v := vs.SelectMustUseVar(r, AccessWrite, cg, typ, &q); v != nil {
		if !compoundAssign || !v.IsVolatile() {
			return finishLhs(v, typ, compoundAssign, cg, opts)
		}
	}

	// Lhs.cpp:84–96 — flipcoin SelectDerefPointerProb
	derefProb := 0
	if probs != nil {
		derefProb = probs.Single(PSelectDerefPointerProb)
	}
	if derefProb > 0 && r.RndFlipcoin(uint32(derefProb)) {
		if v := selectDerefPointer(r, opts, probs, vs, cg, typ, &q, AccessWrite); v != nil {
			if !compoundAssign || !v.IsVolatile() {
				return finishLhs(v, typ, compoundAssign, cg, opts)
			}
		}
	}

	// Lhs.cpp:106–118 — VariableSelector::select(WRITE, eDerefExact)
	v := vs.Select(AccessWrite, cg, typ, &q, r, MatchDerefExact)
	if v != nil && compoundAssign && v.IsVolatile() {
		if nv := selectWritable(r, vs, cg, typ, true); nv != nil {
			return finishLhs(nv, typ, compoundAssign, cg, opts)
		}
	}
	if v != nil {
		return finishLhs(v, typ, compoundAssign, cg, opts)
	}
	// last resort create global
	v = vs.SelectGlobal(AccessWrite, cg, typ, &q, r)
	if v == nil {
		return nil
	}
	return finishLhs(v, typ, compoundAssign, cg, opts)
}

// finishLhs builds Lhs and records write dereference / volatile access.
// Lhs.cpp:132–140 bookkeeping.
func finishLhs(v *Variable, typ *Type, compound bool, cg CGContext, opts Options) *Lhs {
	if v == nil {
		return nil
	}
	// optional: skip signed overflow-prone for compound (Lhs.cpp:106–120 subset)
	if compound && typ != nil && typ.IsSimple() && !typ.IsFloat() {
		if v.IsBitfield || (v.Type != nil && v.Type.IsSigned()) {
			// still allow; full no_signed_overflow path needs visit_facts retry
		}
	}
	lhs := &Lhs{Var: v, Type: typ, CompoundAssign: compound}
	// Lhs.cpp:132–140 — bookkeeping on successful make
	deref := lhs.IndirectLevel()
	if deref > 0 {
		IncrCounter(&writeDereferenceCnts, deref)
	}
	RecordVolatileAccess(v, deref, true)
	// wrap volatiles for OutputLhsC path on Variable
	if opts.WrapVolatiles {
		v.UseVolRVal = true
	}
	_ = cg
	return lhs
}

// selectWritable gathers non-const matching variables from stack, params, globals.
func selectWritable(r *Rng, vs *VariableSelector, cg CGContext, typ *Type, compound bool) *Variable {
	var ok []*Variable
	add := func(v *Variable) {
		if v == nil || v.Type == nil {
			return
		}
		if v.IsConst() {
			return
		}
		if compound && v.IsVolatile() {
			return
		}
		// expand fields for aggregates
		for _, x := range v.CollectExpandable() {
			if x != nil && x.Type != nil && !x.IsConst() && typ.Match(x.Type, MatchFlexible) {
				if compound && x.IsVolatile() {
					continue
				}
				ok = append(ok, x)
			}
		}
	}
	if cg.CurrentFunc != nil {
		for i := len(cg.CurrentFunc.Stack) - 1; i >= 0; i-- {
			blk := cg.CurrentFunc.Stack[i]
			if blk == nil {
				continue
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
	return ChooseOKVar(r, ok)
}

// selectDerefPointer mirrors VariableSelector::select_deref_pointer.
// VariableSelector.cpp:1246–1318 — visible vars + MatchDereference + eligibility; else new ptr.
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
	if typ == nil || r == nil {
		return nil
	}
	// VariableSelector.cpp:1252–1266 — nonvol globals + block locals + params
	var cands []*Variable
	if vs != nil {
		if len(vs.GlobalNonvolatilesList) > 0 {
			cands = append(cands, vs.GlobalNonvolatilesList...)
		} else {
			cands = append(cands, vs.GlobalList...)
		}
	}
	var blk *Block
	if cg.CurrentFunc != nil {
		if len(cg.CurrentFunc.Stack) > 0 {
			blk = cg.CurrentFunc.Stack[len(cg.CurrentFunc.Stack)-1]
		}
		// walk parent chain for locals
		for b := blk; b != nil; b = b.Parent {
			cands = append(cands, b.LocalVars...)
		}
		cands = append(cands, cg.CurrentFunc.Param...)
	}
	// choose_var with eDereference
	if v := ChooseVar(r, cands, access, cg, typ, MatchDereference); v != nil {
		return v
	}

	// create pointer to typ if under max indirect
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
	// expand_struct on create path (VariableSelector.cpp:1288–1310)
	// volatile qfer → global eager; else local eager
	if vs != nil && vs.Opts.ExpandStruct {
		vol := qfer != nil && qfer.IsVolatile()
		if vol {
			if v := vs.EagerCreateGlobalStruct(access, cg, ptrType, qfer, r, MatchDereference); v != nil {
				return v
			}
		} else if blk != nil {
			if v := vs.EagerCreateLocalStruct(blk, access, cg, ptrType, qfer, r, MatchDereference); v != nil {
				return v
			}
		}
	}
	// new local non-volatile preferred; else global
	if vs != nil && cg.CurrentFunc != nil && len(cg.CurrentFunc.Stack) > 0 {
		b := cg.CurrentFunc.Stack[len(cg.CurrentFunc.Stack)-1]
		pq := RandomQualifiersDefaultProbs(ptrType, access, cg, true, opts, probs, r)
		if access == AccessWrite {
			pq = NewCVQualifiers([]bool{false}, []bool{false})
		}
		return vs.GenerateNewParentLocal(b, access, cg, ptrType, &pq, r)
	}
	pq := NewCVQualifiers([]bool{false}, []bool{false})
	if vs == nil {
		return nil
	}
	return vs.GenerateNewGlobal(access, cg, ptrType, &pq, r)
}
