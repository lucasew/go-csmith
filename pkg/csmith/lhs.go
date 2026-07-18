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

	// Lhs.cpp:106+ — VariableSelector::select(WRITE, …, eDerefExact) simplified to eFlexible
	v := vs.Select(AccessWrite, cg, typ, &q, r, MatchFlexible)
	if v != nil && compoundAssign && v.IsVolatile() {
		// compound assigns avoid volatile LHS (StatementAssign non-vol preference)
		if nv := selectWritable(r, vs, cg, typ, true); nv != nil {
			return nv, typ
		}
	}
	if v != nil {
		return v, typ
	}
	// last resort create global
	return vs.SelectGlobal(AccessWrite, cg, typ, &q, r), typ
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
