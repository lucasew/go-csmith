// Upstream: Lhs.cpp (make_random, Output via ExpressionVariable).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomLhs mirrors Lhs::make_random without FactMgr / visit_facts loop.
// Lhs.cpp:72–155 — SelectDerefPointerProb then select WRITE; else create.
// Returns LHS variable and optional Indir (desired type for * emit).
func MakeRandomLhs(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	cg CGContext,
	typ *Type,
	compoundAssign bool,
) (lhs *Variable, exprType *Type) {
	if typ == nil {
		typ = GetIntType()
	}
	if r == nil || vs == nil {
		return nil, typ
	}
	// non-const WRITE qualifiers
	q := NewCVQualifiers([]bool{false}, []bool{false})

	// Lhs.cpp:84–96 — flipcoin SelectDerefPointerProb
	derefProb := 0
	if probs != nil {
		derefProb = probs.Single(PSelectDerefPointerProb)
	}
	if derefProb > 0 && r.RndFlipcoin(uint32(derefProb)) {
		if v := selectDerefPointer(r, opts, probs, vs, cg, typ, &q, AccessWrite); v != nil {
			// compound: skip volatile
			if !compoundAssign || !v.IsVolatile() {
				return v, typ
			}
		}
	}

	// select WRITE among locals/params/globals matching type (flexible)
	if v := selectWritable(r, vs, cg, typ, compoundAssign); v != nil {
		return v, typ
	}

	// VariableSelector::select → create new global
	v := vs.SelectGlobal(AccessWrite, cg, typ, &q, r)
	return v, typ
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

// selectDerefPointer mirrors VariableSelector::select_deref_pointer simplified.
// VariableSelector.cpp:1246–1318 — collect visible vars; MatchDereference; else new pointer.
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
	// candidates: globals (nonvol for simplicity) + locals + params
	var cands []*Variable
	addAll := func(list []*Variable) {
		for _, v := range list {
			if v == nil || v.Type == nil {
				continue
			}
			// eDereference: var is pointer-to-typ (one extra indirection)
			if v.Type.IndirectLevel() == typ.IndirectLevel()+1 {
				// pointee match flexible
				if pt := v.Type.PtrType(); pt != nil && typ.Match(pt, MatchFlexible) {
					if access == AccessWrite && v.IsConst() {
						continue
					}
					cands = append(cands, v)
				}
			}
		}
	}
	if vs != nil {
		addAll(vs.GlobalNonvolatilesList)
		if len(vs.GlobalNonvolatilesList) == 0 {
			addAll(vs.GlobalList)
		}
	}
	if cg.CurrentFunc != nil {
		for _, blk := range cg.CurrentFunc.Stack {
			if blk != nil {
				addAll(blk.LocalVars)
			}
		}
		addAll(cg.CurrentFunc.Param)
	}
	if v := ChooseOKVar(r, cands); v != nil {
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
	// new local non-volatile preferred; else global
	if cg.CurrentFunc != nil && len(cg.CurrentFunc.Stack) > 0 {
		blk := cg.CurrentFunc.Stack[len(cg.CurrentFunc.Stack)-1]
		pq := RandomQualifiersDefaultProbs(ptrType, access, cg, true, opts, probs, r)
		if access == AccessWrite {
			// set_const(false, 1) on pointer level — simplified: no const
			pq = NewCVQualifiers([]bool{false}, []bool{false})
		}
		return vs.GenerateNewParentLocal(blk, access, cg, ptrType, &pq, r)
	}
	pq := NewCVQualifiers([]bool{false}, []bool{false})
	return vs.GenerateNewGlobal(access, cg, ptrType, &pq, r)
}
